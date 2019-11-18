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

package metric

import (
	"time"

	autoscaling "k8s.io/api/autoscaling/v2beta2"
	"k8s.io/apimachinery/pkg/util/sets"
	metricsclient "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
)

// CombinedMetric represents a metric that has been gathered using a MetricSpec, it can be any of the types of
// metrics within the CombinedMetric as each is optional. The CombinedMetric also provides the Spec used to
// gather the metric, alongside the CurrentReplicas at time of gathering.
type CombinedMetric struct {
	CurrentReplicas int32                  `json:"current_replicas"`
	Spec            autoscaling.MetricSpec `json:"spec"`
	Resource        *ResourceMetric        `json:"resource,omitempty"`
	Pods            *PodsMetric            `json:"pods,omitempty"`
	Object          *ObjectMetric          `json:"object,omitempty"`
	External        *ExternalMetric        `json:"external,omitempty"`
}

// ResourceMetric is a resource metric known to Kubernetes, as
// specified in requests and limits, describing each pod in the current
// scale target (e.g. CPU or memory).  Such metrics are built in to
// Kubernetes, and have special scaling options on top of those available
// to normal per-pod metrics (the "pods" source).
type ResourceMetric struct {
	PodMetricsInfo metricsclient.PodMetricsInfo `json:"pod_metrics_info"`
	Requests       map[string]int64             `json:"requests"`
	ReadyPodCount  int64                        `json:"ready_pod_count"`
	IgnoredPods    sets.String                  `json:"ignored_pods"`
	MissingPods    sets.String                  `json:"missing_pods"`
	TotalPods      int                          `json:"total_pods"`
	Timestamp      time.Time                    `json:"timestamp"`
}

// PodsMetric is a metric describing each pod in the current scale
// target (for example, transactions-processed-per-second).  The values
// will be averaged together before being compared to the target value.
type PodsMetric struct {
	PodMetricsInfo metricsclient.PodMetricsInfo
	ReadyPodCount  int64
	IgnoredPods    sets.String
	MissingPods    sets.String
	TotalPods      int
	Timestamp      time.Time
}

// ObjectMetric is a metric describing a kubernetes object
// (for example, hits-per-second on an Ingress object).
type ObjectMetric struct {
	Utilization   int64
	ReadyPodCount *int64
	Timestamp     time.Time
}

// ExternalMetric is a global metric that is not associated
// with any Kubernetes object. It allows autoscaling based on information
// coming from components running outside of cluster
// (for example length of queue in cloud messaging service, or
// QPS from loadbalancer running outside of cluster).
type ExternalMetric struct {
	Utilization   int64
	ReadyPodCount *int64
	Timestamp     time.Time
}
