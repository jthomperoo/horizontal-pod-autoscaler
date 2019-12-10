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

package object_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jthomperoo/custom-pod-autoscaler/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/calculate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/object"
	"github.com/jthomperoo/horizontal-pod-autoscaler/fake"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
	objectmetric "github.com/jthomperoo/horizontal-pod-autoscaler/metric/object"
	"k8s.io/api/autoscaling/v2beta2"
	"k8s.io/apimachinery/pkg/api/resource"
)

func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
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
		expected        *evaluate.Evaluation
		expectedErr     error
		calculater      calculate.Calculater
		tolerance       float64
		currentReplicas int32
		gatheredMetric  *metric.Metric
	}{
		{
			"Invalid metric source",
			nil,
			errors.New("invalid object metric source: neither a value target nor an average value target was set"),
			nil,
			0,
			3,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Object: &v2beta2.ObjectMetricSource{},
				},
			},
		},
		{
			"Success, average value, beyond tolerance",
			&evaluate.Evaluation{
				TargetReplicas: 10,
			},
			nil,
			nil,
			0,
			5,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Object: &v2beta2.ObjectMetricSource{
						Target: v2beta2.MetricTarget{
							Type:         v2beta2.AverageValueMetricType,
							AverageValue: resource.NewMilliQuantity(50, resource.DecimalSI),
						},
					},
				},
				Object: &objectmetric.Metric{
					Utilization: 500,
				},
			},
		},
		{
			"Success, average value, within tolerance",
			&evaluate.Evaluation{
				TargetReplicas: 5,
			},
			nil,
			nil,
			0,
			5,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Object: &v2beta2.ObjectMetricSource{
						Target: v2beta2.MetricTarget{
							Type:         v2beta2.AverageValueMetricType,
							AverageValue: resource.NewMilliQuantity(50, resource.DecimalSI),
						},
					},
				},
				Object: &objectmetric.Metric{
					Utilization: 250,
				},
			},
		},
		{
			"Success, value",
			&evaluate.Evaluation{
				TargetReplicas: 3,
			},
			nil,
			&fake.Calculate{
				GetUsageRatioReplicaCountReactor: func(currentReplicas int32, usageRatio float64, readyPodCount int64) int32 {
					return 3
				},
			},
			0,
			5,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Object: &v2beta2.ObjectMetricSource{
						Target: v2beta2.MetricTarget{
							Type:  v2beta2.ValueMetricType,
							Value: resource.NewMilliQuantity(50, resource.DecimalSI),
						},
					},
				},
				Object: &objectmetric.Metric{
					ReadyPodCount: int64Ptr(2),
					Utilization:   250,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			evaluater := object.Evaluate{
				Calculater: test.calculater,
				Tolerance:  test.tolerance,
			}
			evaluation, err := evaluater.GetEvaluation(test.currentReplicas, test.gatheredMetric)
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
