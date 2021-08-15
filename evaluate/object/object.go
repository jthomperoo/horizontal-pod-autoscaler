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

package object

import (
	"fmt"
	"math"

	"github.com/jthomperoo/custom-pod-autoscaler/v2/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/calculate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
	autoscaling "k8s.io/api/autoscaling/v2beta2"
)

// Evaluator (object) produces an evaluation based on an object metric provided
type Evaluator interface {
	GetEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error)
}

// Evaluate (object) calculates a replica count evaluation, using the tolerance and calculater provided
type Evaluate struct {
	Calculater calculate.Calculater
	Tolerance  float64
}

// GetEvaluation calculates an evaluation based on the metric provided and the current number of replicas
func (e *Evaluate) GetEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error) {
	if gatheredMetric.Spec.Object.Target.Type == autoscaling.ValueMetricType {
		usageRatio := float64(gatheredMetric.Object.Utilization) / float64(gatheredMetric.Spec.Object.Target.Value.MilliValue())
		replicaCount := e.Calculater.GetUsageRatioReplicaCount(currentReplicas, usageRatio, *gatheredMetric.Object.ReadyPodCount)
		return &evaluate.Evaluation{
			TargetReplicas: replicaCount,
		}, nil
	}
	if gatheredMetric.Spec.Object.Target.Type == autoscaling.AverageValueMetricType {
		utilization := gatheredMetric.Object.Utilization
		replicaCount := currentReplicas
		usageRatio := float64(utilization) / (float64(gatheredMetric.Spec.Object.Target.AverageValue.MilliValue()) * float64(replicaCount))
		if math.Abs(1.0-usageRatio) > e.Tolerance {
			// update number of replicas if change is large enough
			replicaCount = int32(math.Ceil(float64(utilization) / float64(gatheredMetric.Spec.Object.Target.AverageValue.MilliValue())))
		}
		return &evaluate.Evaluation{
			TargetReplicas: replicaCount,
		}, nil
	}
	return nil, fmt.Errorf("invalid object metric source: neither a value target nor an average value target was set")
}
