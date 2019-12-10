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

package pods_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jthomperoo/custom-pod-autoscaler/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/calculate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/pods"
	"github.com/jthomperoo/horizontal-pod-autoscaler/fake"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
	metricpods "github.com/jthomperoo/horizontal-pod-autoscaler/metric/pods"
	"k8s.io/api/autoscaling/v2beta2"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
)

func TestGetEvaluation(t *testing.T) {
	var tests = []struct {
		description     string
		expected        *evaluate.Evaluation
		calculater      calculate.Calculater
		currentReplicas int32
		gatheredMetric  *metric.Metric
	}{
		{
			"Calculate 5 replicas, 2 ready pods, 1 ignored and 1 missing",
			&evaluate.Evaluation{
				TargetReplicas: 5,
			},
			&fake.Calculate{
				GetPlainMetricReplicaCountReactor: func(metrics metrics.PodMetricsInfo, currentReplicas int32, targetUtilization, readyPodCount int64, missingPods, ignoredPods sets.String) int32 {
					return 5
				},
			},
			4,
			&metric.Metric{
				CurrentReplicas: 4,
				Spec: v2beta2.MetricSpec{
					Pods: &v2beta2.PodsMetricSource{
						Target: v2beta2.MetricTarget{
							Value: resource.NewMilliQuantity(50, resource.DecimalSI),
						},
					},
				},
				Pods: &metricpods.Metric{
					PodMetricsInfo: metrics.PodMetricsInfo{},
					ReadyPodCount:  2,
					IgnoredPods:    sets.String{"ignored": {}},
					MissingPods:    sets.String{"missing": {}},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			eval := pods.Evaluate{
				Calculater: test.calculater,
			}
			result := eval.GetEvaluation(test.currentReplicas, test.gatheredMetric)
			if !cmp.Equal(test.expected, result) {
				t.Errorf("evaluation mismatch (-want +got):\n%s", cmp.Diff(test.expected, result))
			}
		})
	}
}
