/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Modifications Copyright 2019 The Custom Pod Autoscaler Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.

Modified to split up evaluations and metric gathering to work with the
Custom Pod Autoscaler framework.
Original source:
https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/podautoscaler/horizontal.go
https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/podautoscaler/replica_calculator.go
*/

package pods

import (
	"fmt"
	"time"

	"github.com/jthomperoo/horizontal-pod-autoscaler/metric/podutil"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	corelisters "k8s.io/client-go/listers/core/v1"
	metricsclient "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
)

// Gatherer (Pods) allows retrieval of pods metrics.
type Gatherer interface {
	GetMetric(metricName string, namespace string, selector labels.Selector, metricSelector labels.Selector) (*Metric, error)
}

// Metric (Pods) is a metric describing each pod in the current scale
// target (for example, transactions-processed-per-second).  The values
// will be averaged together before being compared to the target value.
type Metric struct {
	PodMetricsInfo metricsclient.PodMetricsInfo
	ReadyPodCount  int64
	IgnoredPods    sets.String
	MissingPods    sets.String
	TotalPods      int
	Timestamp      time.Time
}

// Gather (Pods) provides functionality for retrieving metrics for pods metric specs.
type Gather struct {
	MetricsClient metricsclient.MetricsClient
	PodLister     corelisters.PodLister
}

// GetMetric retrieves a pods metric
func (c *Gather) GetMetric(metricName string, namespace string, selector labels.Selector, metricSelector labels.Selector) (*Metric, error) {
	// Get metrics
	metrics, timestamp, err := c.MetricsClient.GetRawMetric(metricName, namespace, selector, metricSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to get metric %s: %v", metricName, err)
	}

	// Get pods
	podList, err := c.PodLister.Pods(namespace).List(selector)
	if err != nil {
		return nil, fmt.Errorf("unable to get pods while calculating replica count: %v", err)
	}

	totalPods := len(podList)
	if totalPods == 0 {
		return &Metric{
			ReadyPodCount: 0,
			TotalPods:     0,
			Timestamp:     timestamp,
		}, nil
	}

	// Remove missing pod metrics
	readyPodCount, _, missingPods := podutil.GroupPods(podList, metrics, v1.ResourceName(""), 0, 0)

	return &Metric{
		PodMetricsInfo: metrics,
		ReadyPodCount:  int64(readyPodCount),
		IgnoredPods:    nil, // Pods metric cannot be CPU based, so Pods cannot be ignored
		MissingPods:    missingPods,
		TotalPods:      totalPods,
		Timestamp:      timestamp,
	}, nil
}
