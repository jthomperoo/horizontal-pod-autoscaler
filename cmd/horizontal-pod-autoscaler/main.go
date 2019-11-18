/*
Copyright 2019 The Custom Pod Autoscaler Authors.

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
	"strings"
	"time"

	cpametric "github.com/jthomperoo/custom-pod-autoscaler/metric"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
	"github.com/jthomperoo/horizontal-pod-autoscaler/podclient"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2beta2"
	"k8s.io/apimachinery/pkg/util/yaml"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	resourceclient "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	customclient "k8s.io/metrics/pkg/client/custom_metrics"
	externalclient "k8s.io/metrics/pkg/client/external_metrics"
)

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
	var deployment appsv1.Deployment
	err := yaml.NewYAMLOrJSONDecoder(stdin, 10).Decode(&deployment)
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
	gatherer := metric.Gatherer{
		MetricsClient: metrics.NewRESTMetricsClient(
			resourceclient.NewForConfigOrDie(clusterConfig),
			customclient.NewForConfig(
				clusterConfig,
				restmapper.NewDeferredDiscoveryRESTMapper(cacheddiscovery.NewMemCacheClient(clientset.Discovery())),
				customclient.NewAvailableAPIsGetter(clientset.Discovery()),
			),
			externalclient.NewForConfigOrDie(clusterConfig),
		),
		PodLister:                     &podclient.OnDemandPodLister{Clientset: clientset},
		CPUInitializationPeriod:       5 * time.Minute,
		DelayOfInitialReadinessStatus: 30 * time.Second,
	}

	// Get metrics for deployment
	metrics, err := gatherer.GetMetrics(&deployment, metricSpecs, deployment.ObjectMeta.Namespace)
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

	var cpaMetrics []*cpametric.Metric
	err := yaml.NewYAMLOrJSONDecoder(stdin, 10).Decode(&cpaMetrics)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	if len(cpaMetrics) != 1 {
		log.Fatalf("Expected 1 CPA metric, got %d", len(cpaMetrics))
		os.Exit(1)
	}

	var combinedMetrics []*metric.CombinedMetric
	err = yaml.NewYAMLOrJSONDecoder(strings.NewReader(cpaMetrics[0].Value), 10).Decode(&combinedMetrics)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	evaluator := evaluate.Evaluator{}
	evaluation, err := evaluator.GetEvaluation(combinedMetrics)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Marhsal evaluation into JSON
	jsonEvaluation, err := json.Marshal(evaluation)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	fmt.Print(string(jsonEvaluation))
}
