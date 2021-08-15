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

// Package evaluate handles the decision making part of the Horizontal Pod Autoscaler, being fed metrics
// gathered by the metric gatherer and using these to calculate how many replicas a resource should have.
package evaluate

import (
	"fmt"

	"github.com/jthomperoo/custom-pod-autoscaler/v2/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/calculate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/external"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/object"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/pods"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/resource"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
	autoscaling "k8s.io/api/autoscaling/v2beta2"
)

// Evaluater is used to take metrics of any type and produce a single evaluation
type Evaluater interface {
	GetEvaluation(gatheredMetrics []*metric.Metric) (*evaluate.Evaluation, error)
}

// Evaluate provides functionality for deciding how many replicas a resource should have based on provided metrics.
type Evaluate struct {
	External external.Evaluator
	Object   object.Evaluator
	Pods     pods.Evaluator
	Resource resource.Evaluator
}

// NewEvaluate sets up an evaluate that can process external, object, pod and resource metrics, with a shared replica calculater
func NewEvaluate(tolerance float64) *Evaluate {
	calculate := &calculate.ReplicaCalculate{
		Tolerance: tolerance,
	}
	return &Evaluate{
		External: &external.Evaluate{
			Calculater: calculate,
			Tolerance:  tolerance,
		},
		Object: &object.Evaluate{
			Calculater: calculate,
			Tolerance:  tolerance,
		},
		Pods: &pods.Evaluate{
			Calculater: calculate,
		},
		Resource: &resource.Evaluate{
			Calculater: calculate,
			Tolerance:  tolerance,
		},
	}
}

// GetEvaluation takes in metrics and outputs an evaluation decision
func (e *Evaluate) GetEvaluation(gatheredMetrics []*metric.Metric) (*evaluate.Evaluation, error) {
	var evaluation *evaluate.Evaluation
	var invalidEvaluationError error
	invalidEvaluationsCount := 0

	for _, gatheredMetric := range gatheredMetrics {
		proposedEvaluation, err := e.getEvaluation(gatheredMetric.CurrentReplicas, gatheredMetric)
		if err != nil {
			if invalidEvaluationsCount <= 0 {
				invalidEvaluationError = err
			}
			invalidEvaluationsCount++
			continue
		}
		if evaluation == nil {
			evaluation = proposedEvaluation
			continue
		}
		// Mutliple evaluations, take the highest replica count
		if proposedEvaluation.TargetReplicas > evaluation.TargetReplicas {
			evaluation = proposedEvaluation
		}
	}

	// If all evaluations are invalid return error and return first evaluation error.
	if invalidEvaluationsCount >= len(gatheredMetrics) {
		return nil, fmt.Errorf("invalid evaluations (%v invalid out of %v), first error is: %v", invalidEvaluationsCount, len(gatheredMetrics), invalidEvaluationError)
	}
	return evaluation, nil
}

func (e *Evaluate) getEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error) {
	switch gatheredMetric.Spec.Type {
	case autoscaling.ObjectMetricSourceType:
		return e.Object.GetEvaluation(currentReplicas, gatheredMetric)
	case autoscaling.PodsMetricSourceType:
		return e.Pods.GetEvaluation(currentReplicas, gatheredMetric), nil
	case autoscaling.ResourceMetricSourceType:
		return e.Resource.GetEvaluation(currentReplicas, gatheredMetric)
	case autoscaling.ExternalMetricSourceType:
		return e.External.GetEvaluation(currentReplicas, gatheredMetric)
	default:
		return nil, fmt.Errorf("unknown metric source type %q", string(gatheredMetric.Spec.Type))
	}
}
