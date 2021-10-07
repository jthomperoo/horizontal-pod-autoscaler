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

Modifications Copyright 2021 The Custom Pod Autoscaler Authors.

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
	"github.com/jthomperoo/custom-pod-autoscaler/v2/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/calculate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
)

// Evaluator (pods) produces an evaluation based on a pods metric provided
type Evaluator interface {
	GetEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) *evaluate.Evaluation
}

// Evaluate (pods) calculates a replica count evaluation, using the tolerance and calculater provided
type Evaluate struct {
	Calculater calculate.Calculater
}

// GetEvaluation calculates an evaluation based on the metric provided and the current number of replicas
func (e *Evaluate) GetEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) *evaluate.Evaluation {
	return &evaluate.Evaluation{
		TargetReplicas: e.Calculater.GetPlainMetricReplicaCount(
			gatheredMetric.Pods.PodMetricsInfo,
			currentReplicas,
			gatheredMetric.Spec.Pods.Target.AverageValue.MilliValue(),
			gatheredMetric.Pods.ReadyPodCount,
			gatheredMetric.Pods.MissingPods,
			gatheredMetric.Pods.IgnoredPods,
		),
	}
}
