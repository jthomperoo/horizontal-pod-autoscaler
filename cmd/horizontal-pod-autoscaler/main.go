/*
Copyright 2020 The Custom Pod Autoscaler Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Horizontal Pod Autoscaler provides executable Horizontal Pod Autoscaler logic, which
// can be built into a Custom Pod Autoscaler.
// The Horizontal Pod Autoscaler has two modes, metric gathering and evaluation.
// Metric mode gathers metrics, taking in a resource to get the metrics for and outputting
// these metrics as serialised JSON.
// Evaluation mode makes decisions on how many replicas a resource should have, taking in
// metrics and outputting evaluation decisions as seralised JSON.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	cpametric "github.com/jthomperoo/custom-pod-autoscaler/metric"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
	"github.com/jthomperoo/horizontal-pod-autoscaler/podclient"
	autoscalingv2 "k8s.io/api/autoscaling/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	resourceclient "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	customclient "k8s.io/metrics/pkg/client/custom_metrics"
	externalclient "k8s.io/metrics/pkg/client/external_metrics"
)

const (
	defaultTolerance = float64(0.1)
	// 5 minute CPU initialization period
	defaultCPUInitializationPeriod = 300
	// 30 second initial readiness delay
	defaultInitialReadinessDelay = 30
)

// EvaluateSpec represents the information fed to the evaluator
type EvaluateSpec struct {
	Metrics              []*cpametric.Metric       `json:"metrics"`
	UnstructuredResource unstructured.Unstructured `json:"resource"`
	Resource             metav1.Object             `json:"-"`
	RunType              string                    `json:"runType"`
}

// MetricSpec represents the information fed to the metric gatherer
type MetricSpec struct {
	UnstructuredResource unstructured.Unstructured `json:"resource"`
	Resource             metav1.Object             `json:"-"`
	RunType              string                    `json:"runType"`
}

func main() {
	stdin, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	modePtr := flag.String("mode", "no_mode", "command mode, either metric or evaluate")
	flag.Parse()

	switch *modePtr {
	case "metric":
		getMetrics(bytes.NewReader(stdin))
	case "evaluate":
		getEvaluation(bytes.NewReader(stdin))
	default:
		log.Fatalf("Unknown command mode: %s", *modePtr)
		os.Exit(1)
	}
}

func getMetrics(stdin io.Reader) {
	var spec MetricSpec
	err := yaml.NewYAMLOrJSONDecoder(stdin, 10).Decode(&spec)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Create object from version and kind of piped value
	resourceGVK := spec.UnstructuredResource.GroupVersionKind()
	resourceRuntime, err := scheme.Scheme.New(resourceGVK)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Parse the unstructured k8s resource into the object created, then convert to generic metav1.Object
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(spec.UnstructuredResource.Object, resourceRuntime)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	spec.Resource = resourceRuntime.(metav1.Object)

	metricSpecsValue, exists := os.LookupEnv("metrics")
	if !exists {
		log.Fatal("Metric specs not supplied")
		os.Exit(1)
	}

	// Read in metric specs to evaluate
	var metricSpecs []autoscalingv2.MetricSpec
	err = yaml.NewYAMLOrJSONDecoder(strings.NewReader(metricSpecsValue), 10).Decode(&metricSpecs)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	if len(metricSpecs) == 0 {
		log.Fatal("Metric specs not supplied")
		os.Exit(1)
	}

	// Get initial readiness delay, can be set as a configuration variable
	var initialReadinessDelay int64
	initialReadinessDelayValue, exists := os.LookupEnv("initialReadinessDelay")
	if !exists {
		// use default
		initialReadinessDelay = defaultInitialReadinessDelay
	} else {
		// try to parse provided value
		initialReadinessDelay, err = strconv.ParseInt(initialReadinessDelayValue, 10, 64)
		if err != nil {
			log.Fatalf("Invalid initial readiness delay provided - %e\n", err)
			os.Exit(1)
		}
	}

	// Get CPU initialization period, can be set as a configuration variable
	var cpuInitializationPeriod int64
	cpuInitializationPeriodValue, exists := os.LookupEnv("cpuInitializationPeriod")
	if !exists {
		// use default
		cpuInitializationPeriod = defaultCPUInitializationPeriod
	} else {
		// try to parse provided value
		cpuInitializationPeriod, err = strconv.ParseInt(cpuInitializationPeriodValue, 10, 64)
		if err != nil {
			log.Fatalf("Invalid CPU initialization period provided - %e\n", err)
			os.Exit(1)
		}
	}

	// Create the in-cluster Kubernetes config
	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Create the Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Create metric gatherer, with required clients and configuration
	gatherer := metric.NewGather(metrics.NewRESTMetricsClient(
		resourceclient.NewForConfigOrDie(clusterConfig),
		customclient.NewForConfig(
			clusterConfig,
			restmapper.NewDeferredDiscoveryRESTMapper(cacheddiscovery.NewMemCacheClient(clientset.Discovery())),
			customclient.NewAvailableAPIsGetter(clientset.Discovery()),
		),
		externalclient.NewForConfigOrDie(clusterConfig),
	), &podclient.OnDemandPodLister{Clientset: clientset}, time.Duration(cpuInitializationPeriod)*time.Second, time.Duration(initialReadinessDelay)*time.Second)

	// Get metrics for deployment
	metrics, err := gatherer.GetMetrics(spec.Resource, metricSpecs, spec.Resource.GetNamespace())
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Marshal metrics into JSON
	jsonMetrics, err := json.Marshal(metrics)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Write serialised metrics to stdout
	fmt.Print(string(jsonMetrics))
}

func getEvaluation(stdin io.Reader) {
	var spec EvaluateSpec
	err := yaml.NewYAMLOrJSONDecoder(stdin, 10).Decode(&spec)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Create object from version and kind of piped value
	resourceGVK := spec.UnstructuredResource.GroupVersionKind()
	resourceRuntime, err := scheme.Scheme.New(resourceGVK)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Parse the unstructured k8s resource into the object created, then convert to generic metav1.Object
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(spec.UnstructuredResource.Object, resourceRuntime)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	spec.Resource = resourceRuntime.(metav1.Object)

	// Get tolerance, can be set as a configuration variable
	var tolerance float64
	toleranceValue, exists := os.LookupEnv("tolerance")
	if !exists {
		// use default
		tolerance = defaultTolerance
	} else {
		// try to parse provided value
		tolerance, err = strconv.ParseFloat(toleranceValue, 64)
		if err != nil {
			log.Fatalf("Invalid tolerance provided - %e\n", err)
			os.Exit(1)
		}
	}

	var combinedMetrics []*metric.Metric
	err = yaml.NewYAMLOrJSONDecoder(strings.NewReader(spec.Metrics[0].Value), 10).Decode(&combinedMetrics)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	evaluator := evaluate.NewEvaluate(tolerance)
	evaluation, err := evaluator.GetEvaluation(combinedMetrics)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Marshal evaluation into JSON
	jsonEvaluation, err := json.Marshal(evaluation)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	fmt.Print(string(jsonEvaluation))
}
