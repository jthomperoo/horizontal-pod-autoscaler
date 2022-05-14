/*
Copyright 2021 The Custom Pod Autoscaler Authors.

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
	"context"
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

	autoscalingv1 "k8s.io/api/autoscaling/v1"

	cpaevaluate "github.com/jthomperoo/custom-pod-autoscaler/v2/evaluate"
	cpametric "github.com/jthomperoo/custom-pod-autoscaler/v2/metric"
	"github.com/jthomperoo/k8shorizmetrics"
	"github.com/jthomperoo/k8shorizmetrics/metrics"
	"github.com/jthomperoo/k8shorizmetrics/metricsclient"
	"github.com/jthomperoo/k8shorizmetrics/podsclient"
	autoscalingv2 "k8s.io/api/autoscaling/v2beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	k8sscale "k8s.io/client-go/scale"
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
	Metrics              []*cpametric.ResourceMetric `json:"metrics"`
	UnstructuredResource unstructured.Unstructured   `json:"resource"`
	RunType              string                      `json:"runType"`
}

// MetricSpec represents the information fed to the metric gatherer
type MetricSpec struct {
	UnstructuredResource unstructured.Unstructured `json:"resource"`
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
		gather(bytes.NewReader(stdin))
	case "evaluate":
		evaluate(bytes.NewReader(stdin))
	default:
		log.Fatalf("Unknown command mode: %s", *modePtr)
		os.Exit(1)
	}
}

func gather(stdin io.Reader) {
	var spec MetricSpec
	err := yaml.NewYAMLOrJSONDecoder(stdin, 10).Decode(&spec)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

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
	initialReadinessDelaySecs, err := parseInt64EnvVar("initialReadinessDelay", defaultInitialReadinessDelay)
	if err != nil {
		log.Fatalf("Invalid initial readiness delay provided: %e\n", err)
		os.Exit(1)
	}

	// Get CPU initialization period, can be set as a configuration variable
	cpuInitializationPeriodSecs, err := parseInt64EnvVar("cpuInitializationPeriod", defaultCPUInitializationPeriod)
	if err != nil {
		log.Fatalf("Invalid CPU initialization period provided: %e\n", err)
		os.Exit(1)
	}

	clusterConfig, clientset, err := getKubernetesClients()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	scale, err := getScaleSubResource(clientset, &spec.UnstructuredResource)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	metricsclient := metricsclient.NewClient(clusterConfig, clientset.Discovery())
	podsclient := &podsclient.OnDemandPodLister{
		Clientset: clientset,
	}
	cpuInitializationPeriod := time.Duration(cpuInitializationPeriodSecs) * time.Second
	initialReadinessDelay := time.Duration(initialReadinessDelaySecs) * time.Second

	// Create metric gatherer, with required clients and configuration
	gatherer := k8shorizmetrics.NewGatherer(metricsclient, podsclient, cpuInitializationPeriod, initialReadinessDelay)

	selector, err := labels.Parse(scale.Status.Selector)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Get metrics for deployment
	metrics, err := gatherer.Gather(metricSpecs, scale.GetNamespace(), selector)
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

func evaluate(stdin io.Reader) {
	var spec EvaluateSpec
	err := yaml.NewYAMLOrJSONDecoder(stdin, 10).Decode(&spec)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

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
			log.Fatalf("Invalid tolerance provided: %e\n", err)
			os.Exit(1)
		}
	}

	_, clientset, err := getKubernetesClients()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	scale, err := getScaleSubResource(clientset, &spec.UnstructuredResource)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	var combinedMetrics []*metrics.Metric
	err = yaml.NewYAMLOrJSONDecoder(strings.NewReader(spec.Metrics[0].Value), 10).Decode(&combinedMetrics)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	evaluator := k8shorizmetrics.NewEvaluator(tolerance)
	targetReplicas, err := evaluator.Evaluate(combinedMetrics, scale.Spec.Replicas)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Marshal evaluation into JSON
	jsonEvaluation, err := json.Marshal(cpaevaluate.Evaluation{
		TargetReplicas: targetReplicas,
	})
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Write serialised evaluation to stdout
	fmt.Print(string(jsonEvaluation))
}

func getScaleSubResource(clientset *kubernetes.Clientset, resource *unstructured.Unstructured) (*autoscalingv1.Scale, error) {
	groupResources, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return nil, err
	}

	scaleClient := k8sscale.New(
		clientset.RESTClient(),
		restmapper.NewDiscoveryRESTMapper(groupResources),
		dynamic.LegacyAPIPathResolverFunc,
		k8sscale.NewDiscoveryScaleKindResolver(
			clientset.Discovery(),
		),
	)

	resourceGV, err := schema.ParseGroupVersion(resource.GetAPIVersion())
	if err != nil {
		return nil, err
	}

	targetGR := schema.GroupResource{
		Group:    resourceGV.Group,
		Resource: resource.GetKind(),
	}

	scale, err := scaleClient.Scales(resource.GetNamespace()).Get(context.Background(), targetGR, resource.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return scale, nil
}

func getKubernetesClients() (*rest.Config, *kubernetes.Clientset, error) {
	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return nil, nil, err

	}

	return clusterConfig, clientset, nil
}

func parseInt64EnvVar(name string, defaultValue int64) (int64, error) {
	var envVar int64
	var err error
	envVarVal, exists := os.LookupEnv(name)
	if !exists {
		// use default
		envVar = defaultValue
	} else {
		// try to parse provided value
		envVar, err = strconv.ParseInt(envVarVal, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	return envVar, nil
}
