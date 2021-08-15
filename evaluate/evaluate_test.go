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

package evaluate_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	cpaevaluate "github.com/jthomperoo/custom-pod-autoscaler/v2/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/calculate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/external"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/object"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/pods"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/resource"
	"github.com/jthomperoo/horizontal-pod-autoscaler/fake"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
	"k8s.io/api/autoscaling/v2beta2"
)

func TestNewEvaluate(t *testing.T) {
	var tests = []struct {
		description string
		expected    evaluate.Evaluater
		tolerance   float64
	}{
		{
			"Set up all sub evaluaters",
			&evaluate.Evaluate{
				External: &external.Evaluate{
					Tolerance: 5,
					Calculater: &calculate.ReplicaCalculate{
						Tolerance: 5,
					},
				},
				Object: &object.Evaluate{
					Tolerance: 5,
					Calculater: &calculate.ReplicaCalculate{
						Tolerance: 5,
					},
				},
				Pods: &pods.Evaluate{
					Calculater: &calculate.ReplicaCalculate{
						Tolerance: 5,
					},
				},
				Resource: &resource.Evaluate{
					Tolerance: 5,
					Calculater: &calculate.ReplicaCalculate{
						Tolerance: 5,
					},
				},
			},
			5,
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			evaluater := evaluate.NewEvaluate(test.tolerance)
			if !cmp.Equal(test.expected, evaluater) {
				t.Errorf("evaluate mismatch (-want +got):\n%s", cmp.Diff(test.expected, evaluater))
			}
		})
	}
}

func TestGetEvaluation(t *testing.T) {
	equateErrorMessage := cmp.Comparer(func(x, y error) bool {
		if x == nil || y == nil {
			return x == nil && y == nil
		}
		return x.Error() == y.Error()
	})

	var tests = []struct {
		description     string
		expected        *cpaevaluate.Evaluation
		expectedErr     error
		resource        resource.Evaluator
		object          object.Evaluator
		pods            pods.Evaluator
		external        external.Evaluator
		gatheredMetrics []*metric.Metric
	}{
		{
			"Single unknown metric type",
			nil,
			errors.New(`invalid evaluations (1 invalid out of 1), first error is: unknown metric source type "invalid"`),
			nil,
			nil,
			nil,
			nil,
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: "invalid",
					},
				},
			},
		},
		{
			"Single object metric, fail to evaluate",
			nil,
			errors.New("invalid evaluations (1 invalid out of 1), first error is: fail to evaluate"),
			nil,
			&fake.ObjectEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return nil, errors.New("fail to evaluate")
				},
			},
			nil,
			nil,
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ObjectMetricSourceType,
					},
				},
			},
		},
		{
			"Single object metric, success 3 replicas",
			&cpaevaluate.Evaluation{
				TargetReplicas: 3,
			},
			nil,
			nil,
			&fake.ObjectEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return &cpaevaluate.Evaluation{
						TargetReplicas: 3,
					}, nil
				},
			},
			nil,
			nil,
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ObjectMetricSourceType,
					},
				},
			},
		},
		{
			"Single pods metric, success 7 replicas",
			&cpaevaluate.Evaluation{
				TargetReplicas: 7,
			},
			nil,
			nil,
			nil,
			&fake.PodsEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) *cpaevaluate.Evaluation {
					return &cpaevaluate.Evaluation{
						TargetReplicas: 7,
					}
				},
			},
			nil,
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.PodsMetricSourceType,
					},
				},
			},
		},
		{
			"Single resource metric, fail to evaluate",
			nil,
			errors.New("invalid evaluations (1 invalid out of 1), first error is: fail to evaluate"),
			&fake.ResourceEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return nil, errors.New("fail to evaluate")
				},
			},
			nil,
			nil,
			nil,
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ResourceMetricSourceType,
					},
				},
			},
		},
		{
			"Single resource metric, success 9 replicas",
			&cpaevaluate.Evaluation{
				TargetReplicas: 9,
			},
			nil,
			&fake.ResourceEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return &cpaevaluate.Evaluation{
						TargetReplicas: 9,
					}, nil
				},
			},
			nil,
			nil,
			nil,
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ResourceMetricSourceType,
					},
				},
			},
		},
		{
			"Single external metric, fail to evaluate",
			nil,
			errors.New("invalid evaluations (1 invalid out of 1), first error is: fail to evaluate"),
			nil,
			nil,
			nil,
			&fake.ExternalEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return nil, errors.New("fail to evaluate")
				},
			},
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ExternalMetricSourceType,
					},
				},
			},
		},
		{
			"Single external metric, success 2 replicas",
			&cpaevaluate.Evaluation{
				TargetReplicas: 2,
			},
			nil,
			nil,
			nil,
			nil,
			&fake.ExternalEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return &cpaevaluate.Evaluation{
						TargetReplicas: 2,
					}, nil
				},
			},
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ExternalMetricSourceType,
					},
				},
			},
		},
		{
			"One of resource, object and external metric all invalid",
			nil,
			errors.New("invalid evaluations (3 invalid out of 3), first error is: fail to evaluate"),
			&fake.ResourceEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return nil, errors.New("fail to evaluate")
				},
			},
			&fake.ObjectEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return nil, errors.New("fail to evaluate")
				},
			},
			nil,
			&fake.ExternalEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return nil, errors.New("fail to evaluate")
				},
			},
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ObjectMetricSourceType,
					},
				},
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ResourceMetricSourceType,
					},
				},
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ExternalMetricSourceType,
					},
				},
			},
		},
		{
			"One of each metric, 2 success, 2 invalid, take the highest",
			&cpaevaluate.Evaluation{
				TargetReplicas: 5,
			},
			nil,
			&fake.ResourceEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return nil, errors.New("fail to evaluate")
				},
			},
			&fake.ObjectEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return &cpaevaluate.Evaluation{
						TargetReplicas: 5,
					}, nil
				},
			},
			&fake.PodsEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) *cpaevaluate.Evaluation {
					return &cpaevaluate.Evaluation{
						TargetReplicas: 1,
					}
				},
			},
			&fake.ExternalEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return nil, errors.New("fail to evaluate")
				},
			},
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ObjectMetricSourceType,
					},
				},
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ResourceMetricSourceType,
					},
				},
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.PodsMetricSourceType,
					},
				},
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ExternalMetricSourceType,
					},
				},
			},
		},
		{
			"Once of each metric, all success, take the highest",
			&cpaevaluate.Evaluation{
				TargetReplicas: 9,
			},
			nil,
			&fake.ResourceEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return &cpaevaluate.Evaluation{
						TargetReplicas: 5,
					}, nil
				},
			},
			&fake.ObjectEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return &cpaevaluate.Evaluation{
						TargetReplicas: -25,
					}, nil
				},
			},
			&fake.PodsEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) *cpaevaluate.Evaluation {
					return &cpaevaluate.Evaluation{
						TargetReplicas: 3,
					}
				},
			},
			&fake.ExternalEvaluater{
				GetEvaluationReactor: func(currentReplicas int32, gatheredMetric *metric.Metric) (*cpaevaluate.Evaluation, error) {
					return &cpaevaluate.Evaluation{
						TargetReplicas: 9,
					}, nil
				},
			},
			[]*metric.Metric{
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ObjectMetricSourceType,
					},
				},
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ResourceMetricSourceType,
					},
				},
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.PodsMetricSourceType,
					},
				},
				{
					Spec: v2beta2.MetricSpec{
						Type: v2beta2.ExternalMetricSourceType,
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			evaluater := evaluate.Evaluate{
				External: test.external,
				Object:   test.object,
				Pods:     test.pods,
				Resource: test.resource,
			}
			evaluation, err := evaluater.GetEvaluation(test.gatheredMetrics)
			if !cmp.Equal(&err, &test.expectedErr, equateErrorMessage) {
				t.Errorf("error mismatch (-want +got):\n%s", cmp.Diff(test.expectedErr, err, equateErrorMessage))
				return
			}
			if !cmp.Equal(test.expected, evaluation) {
				t.Errorf("evaluation mismatch (-want +got):\n%s", cmp.Diff(test.expected, evaluation))
			}
		})
	}
}
