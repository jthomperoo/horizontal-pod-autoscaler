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

package resource_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jthomperoo/custom-pod-autoscaler/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/calculate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/resource"
	"github.com/jthomperoo/horizontal-pod-autoscaler/fake"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
	resourcemetric "github.com/jthomperoo/horizontal-pod-autoscaler/metric/resource"
	"k8s.io/api/autoscaling/v2beta2"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
)

func int32Ptr(i int32) *int32 {
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
			errors.New("invalid resource metric source: neither a utilization target nor a value target was set"),
			nil,
			0,
			3,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Resource: &v2beta2.ResourceMetricSource{},
				},
			},
		},
		{
			"Success, average value",
			&evaluate.Evaluation{
				TargetReplicas: 6,
			},
			nil,
			&fake.Calculate{
				GetPlainMetricReplicaCountReactor: func(metrics metrics.PodMetricsInfo, currentReplicas int32, targetUtilization, readyPodCount int64, missingPods, ignoredPods sets.String) int32 {
					return 6
				},
			},
			0,
			5,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Resource: &v2beta2.ResourceMetricSource{
						Target: v2beta2.MetricTarget{
							AverageValue: k8sresource.NewMilliQuantity(50, k8sresource.DecimalSI),
						},
					},
				},
				Resource: &resourcemetric.Metric{
					PodMetricsInfo: metrics.PodMetricsInfo{},
					ReadyPodCount:  3,
					IgnoredPods:    sets.String{"ignored": {}},
					MissingPods:    sets.String{"missing": {}},
				},
			},
		},
		{
			"Fail, average utilization, no metrics for pods",
			nil,
			errors.New(`no metrics returned matched known pods`),
			nil,
			0,
			3,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Resource: &v2beta2.ResourceMetricSource{
						Target: v2beta2.MetricTarget{
							AverageUtilization: int32Ptr(15),
						},
					},
				},
				Resource: &resourcemetric.Metric{
					PodMetricsInfo: metrics.PodMetricsInfo{},
					Requests:       map[string]int64{},
					ReadyPodCount:  3,
					IgnoredPods:    sets.String{"ignored": {}},
					MissingPods:    sets.String{"missing": {}},
				},
			},
		},
		{
			"Success, average utilization, no ignored pods, no missing pods, within tolerance, no scale change",
			&evaluate.Evaluation{
				TargetReplicas: 2,
			},
			nil,
			nil,
			0,
			2,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Resource: &v2beta2.ResourceMetricSource{
						Target: v2beta2.MetricTarget{
							AverageUtilization: int32Ptr(50),
						},
					},
				},
				Resource: &resourcemetric.Metric{
					PodMetricsInfo: metrics.PodMetricsInfo{
						"pod-1": metrics.PodMetric{
							Value: 5,
						},
						"pod-2": metrics.PodMetric{
							Value: 5,
						},
					},
					Requests: map[string]int64{
						"pod-1": 10,
						"pod-2": 10,
					},
					ReadyPodCount: 2,
					IgnoredPods:   sets.String{},
					MissingPods:   sets.String{},
				},
			},
		},
		{
			"Success, average utilization, no ignored pods, no missing pods, beyond tolerance, scale up",
			&evaluate.Evaluation{
				TargetReplicas: 8,
			},
			nil,
			nil,
			0,
			2,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Resource: &v2beta2.ResourceMetricSource{
						Target: v2beta2.MetricTarget{
							AverageUtilization: int32Ptr(50),
						},
					},
				},
				Resource: &resourcemetric.Metric{
					PodMetricsInfo: metrics.PodMetricsInfo{
						"pod-1": metrics.PodMetric{
							Value: 20,
						},
						"pod-2": metrics.PodMetric{
							Value: 20,
						},
					},
					Requests: map[string]int64{
						"pod-1": 10,
						"pod-2": 10,
					},
					ReadyPodCount: 2,
					IgnoredPods:   sets.String{},
					MissingPods:   sets.String{},
				},
			},
		},
		{
			"Success, average utilization, no ignored pods, no missing pods, beyond tolerance, scale down",
			&evaluate.Evaluation{
				TargetReplicas: 1,
			},
			nil,
			nil,
			0,
			2,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Resource: &v2beta2.ResourceMetricSource{
						Target: v2beta2.MetricTarget{
							AverageUtilization: int32Ptr(50),
						},
					},
				},
				Resource: &resourcemetric.Metric{
					PodMetricsInfo: metrics.PodMetricsInfo{
						"pod-1": metrics.PodMetric{
							Value: 2,
						},
						"pod-2": metrics.PodMetric{
							Value: 2,
						},
					},
					Requests: map[string]int64{
						"pod-1": 10,
						"pod-2": 10,
					},
					ReadyPodCount: 2,
					IgnoredPods:   sets.String{},
					MissingPods:   sets.String{},
				},
			},
		},
		{
			"Success, average utilization, no ignored pods, 2 missing pods, beyond tolerance, scale up",
			&evaluate.Evaluation{
				TargetReplicas: 8,
			},
			nil,
			nil,
			0,
			4,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Resource: &v2beta2.ResourceMetricSource{
						Target: v2beta2.MetricTarget{
							AverageUtilization: int32Ptr(50),
						},
					},
				},
				Resource: &resourcemetric.Metric{
					PodMetricsInfo: metrics.PodMetricsInfo{
						"pod-1": metrics.PodMetric{
							Value: 20,
						},
						"pod-2": metrics.PodMetric{
							Value: 20,
						},
					},
					Requests: map[string]int64{
						"pod-1":     10,
						"pod-2":     10,
						"missing-1": 10,
						"missing-2": 10,
					},
					ReadyPodCount: 2,
					IgnoredPods:   sets.String{},
					MissingPods: sets.String{
						"missing-1": {},
						"missing-2": {},
					},
				},
			},
		},
		{
			"Success, average utilization, no ignored pods, 2 missing pods, beyond tolerance, scale down",
			&evaluate.Evaluation{
				TargetReplicas: 2,
			},
			nil,
			nil,
			0,
			4,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Resource: &v2beta2.ResourceMetricSource{
						Target: v2beta2.MetricTarget{
							AverageUtilization: int32Ptr(50),
						},
					},
				},
				Resource: &resourcemetric.Metric{
					PodMetricsInfo: metrics.PodMetricsInfo{
						"pod-1": metrics.PodMetric{
							Value: 1,
						},
						"pod-2": metrics.PodMetric{
							Value: 1,
						},
					},
					Requests: map[string]int64{
						"pod-1":     20,
						"pod-2":     20,
						"missing-1": 3,
						"missing-2": 3,
					},
					ReadyPodCount: 2,
					IgnoredPods:   sets.String{},
					MissingPods: sets.String{
						"missing-1": {},
						"missing-2": {},
					},
				},
			},
		},
		{
			"Success, average utilization, 2 ignored pods, 2 missing pods, beyond tolerance, scale up",
			&evaluate.Evaluation{
				TargetReplicas: 12,
			},
			nil,
			nil,
			0,
			4,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Resource: &v2beta2.ResourceMetricSource{
						Target: v2beta2.MetricTarget{
							AverageUtilization: int32Ptr(50),
						},
					},
				},
				Resource: &resourcemetric.Metric{
					PodMetricsInfo: metrics.PodMetricsInfo{
						"pod-1": metrics.PodMetric{
							Value: 20,
						},
						"pod-2": metrics.PodMetric{
							Value: 20,
						},
					},
					Requests: map[string]int64{
						"pod-1":     10,
						"pod-2":     10,
						"missing-1": 5,
						"missing-2": 5,
						"ignored-1": 5,
						"ignored-2": 5,
					},
					ReadyPodCount: 2,
					IgnoredPods: sets.String{
						"ignored-1": {},
						"ignored-2": {},
					},
					MissingPods: sets.String{
						"missing-1": {},
						"missing-2": {},
					},
				},
			},
		},
		{
			"Success, average utilization, 2 ignored pods, 2 missing pods, within tolerance, no scale change",
			&evaluate.Evaluation{
				TargetReplicas: 4,
			},
			nil,
			nil,
			0.5,
			4,
			&metric.Metric{
				Spec: v2beta2.MetricSpec{
					Resource: &v2beta2.ResourceMetricSource{
						Target: v2beta2.MetricTarget{
							AverageUtilization: int32Ptr(50),
						},
					},
				},
				Resource: &resourcemetric.Metric{
					PodMetricsInfo: metrics.PodMetricsInfo{
						"pod-1": metrics.PodMetric{
							Value: 20,
						},
						"pod-2": metrics.PodMetric{
							Value: 20,
						},
					},
					Requests: map[string]int64{
						"pod-1":     10,
						"pod-2":     10,
						"missing-1": 10,
						"missing-2": 10,
						"ignored-1": 10,
						"ignored-2": 10,
					},
					ReadyPodCount: 2,
					IgnoredPods: sets.String{
						"ignored-1": {},
						"ignored-2": {},
					},
					MissingPods: sets.String{
						"missing-1": {},
						"missing-2": {},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			evaluater := resource.Evaluate{
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
