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

package resource

import (
	"fmt"
	"math"

	"github.com/jthomperoo/custom-pod-autoscaler/v2/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/evaluate/calculate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
	metricsclient "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
)

// Evaluator (resource) produces an evaluation based on a resource metric provided
type Evaluator interface {
	GetEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error)
}

// Evaluate (resource) calculates a replica count evaluation, using the tolerance and calculater provided
type Evaluate struct {
	Calculater calculate.Calculater
	Tolerance  float64
}

// GetEvaluation calculates an evaluation based on the metric provided and the current number of replicas
func (e *Evaluate) GetEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error) {
	if gatheredMetric.Spec.Resource.Target.AverageValue != nil {
		replicaCount := e.Calculater.GetPlainMetricReplicaCount(
			gatheredMetric.Resource.PodMetricsInfo,
			currentReplicas,
			gatheredMetric.Spec.Resource.Target.AverageValue.MilliValue(),
			gatheredMetric.Resource.ReadyPodCount,
			gatheredMetric.Resource.MissingPods,
			gatheredMetric.Resource.IgnoredPods,
		)
		return &evaluate.Evaluation{
			TargetReplicas: replicaCount,
		}, nil
	}

	if gatheredMetric.Spec.Resource.Target.AverageUtilization != nil {
		metrics := gatheredMetric.Resource.PodMetricsInfo
		requests := gatheredMetric.Resource.Requests
		targetUtilization := *gatheredMetric.Spec.Resource.Target.AverageUtilization
		ignoredPods := gatheredMetric.Resource.IgnoredPods
		missingPods := gatheredMetric.Resource.MissingPods
		readyPodCount := gatheredMetric.Resource.ReadyPodCount

		usageRatio, _, _, err := metricsclient.GetResourceUtilizationRatio(metrics, requests, targetUtilization)
		if err != nil {
			return nil, err
		}

		// usageRatio = SUM(pod metrics) / SUM(pod requests) / targetUtilization
		// usageRatio = averageUtilization / targetUtilization
		// usageRatio ~ 1.0 == no scale
		// usageRatio > 1.0 == scale up
		// usageRatio < 1.0 == scale down

		rebalanceIgnored := len(ignoredPods) > 0 && usageRatio > 1.0
		if !rebalanceIgnored && len(missingPods) == 0 {
			if math.Abs(1.0-usageRatio) <= e.Tolerance {
				// return the current replicas if the change would be too small
				return &evaluate.Evaluation{
					TargetReplicas: currentReplicas,
				}, nil
			}
			targetReplicas := int32(math.Ceil(usageRatio * float64(readyPodCount)))
			// if we don't have any unready or missing pods, we can calculate the new replica count now
			return &evaluate.Evaluation{
				TargetReplicas: targetReplicas,
			}, nil
		}

		if len(missingPods) > 0 {
			if usageRatio < 1.0 {
				// on a scale-down, treat missing pods as using 100% of the resource request
				for podName := range missingPods {
					metrics[podName] = metricsclient.PodMetric{Value: requests[podName]}
				}
			} else if usageRatio > 1.0 {
				// on a scale-up, treat missing pods as using 0% of the resource request
				for podName := range missingPods {
					metrics[podName] = metricsclient.PodMetric{Value: 0}
				}
			}
		}

		if rebalanceIgnored {
			// on a scale-up, treat unready pods as using 0% of the resource request
			for podName := range ignoredPods {
				metrics[podName] = metricsclient.PodMetric{Value: 0}
			}
		}

		// re-run the utilization calculation with our new numbers
		newUsageRatio, _, _, err := metricsclient.GetResourceUtilizationRatio(metrics, requests, targetUtilization)
		if err != nil {
			// NOTE - Unsure if this can be triggered.
			return nil, err
		}

		if math.Abs(1.0-newUsageRatio) <= e.Tolerance || (usageRatio < 1.0 && newUsageRatio > 1.0) || (usageRatio > 1.0 && newUsageRatio < 1.0) {
			// return the current replicas if the change would be too small,
			// or if the new usage ratio would cause a change in scale direction
			return &evaluate.Evaluation{
				TargetReplicas: currentReplicas,
			}, nil
		}

		// return the result, where the number of replicas considered is
		// however many replicas factored into our calculation
		targetReplicas := int32(math.Ceil(newUsageRatio * float64(len(metrics))))
		return &evaluate.Evaluation{
			TargetReplicas: targetReplicas,
		}, nil
	}

	return nil, fmt.Errorf("invalid resource metric source: neither a utilization target nor a value target was set")
}
