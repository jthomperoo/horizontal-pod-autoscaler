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

// Package evaluate handles the decision making part of the Horizontal Pod Autoscaler, being fed metrics
// gathered by the metric gatherer and using these to calculate how many replicas a resource should have.
package evaluate

import (
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/jthomperoo/custom-pod-autoscaler/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
	autoscaling "k8s.io/api/autoscaling/v2beta2"
	metricsclient "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
)

// Evaluator provides functionality for deciding how many replicas a resource should have based on provided metrics.
type Evaluator struct {
	Tolerance float64
}

// GetEvaluation takes in metrics and outputs an evaluation decision
func (e *Evaluator) GetEvaluation(gatheredMetrics []*metric.CombinedMetric) (*evaluate.Evaluation, error) {
	var evaluation *evaluate.Evaluation
	var invalidEvaluationError error
	invalidEvaluationsCount := 0

	for _, gatheredMetric := range gatheredMetrics {
		proposedEvaluation, err := e.getEvaluation(gatheredMetric.CurrentReplicas, gatheredMetric)
		if err != nil {
			return nil, err
		}
		if evaluation == nil {
			evaluation = proposedEvaluation
			continue
		}
		// Mutliple evaluations, take the highest replica count
		if *proposedEvaluation.TargetReplicas > *evaluation.TargetReplicas {
			evaluation = proposedEvaluation
		}
	}

	// If all evaluations are invalid return error and return first evaluation error.
	if invalidEvaluationsCount >= len(gatheredMetrics) {
		return nil, fmt.Errorf("invalid evaluations (%v invalid out of %v), first error is: %v", invalidEvaluationsCount, len(gatheredMetrics), invalidEvaluationError)
	}
	return evaluation, nil
}

func (e *Evaluator) getEvaluation(currentReplicas int32, gatheredMetric *metric.CombinedMetric) (*evaluate.Evaluation, error) {
	switch gatheredMetric.Spec.Type {
	case autoscaling.ObjectMetricSourceType:
		return e.getObjectEvaluation(currentReplicas, gatheredMetric)
	case autoscaling.PodsMetricSourceType:
		return e.getPodsEvaluation(currentReplicas, gatheredMetric), nil
	case autoscaling.ResourceMetricSourceType:
		return e.getResourceEvaluation(currentReplicas, gatheredMetric)
	case autoscaling.ExternalMetricSourceType:
		return e.getExternalEvaluation(currentReplicas, gatheredMetric)
	default:
		return nil, fmt.Errorf("unknown metric source type %q", string(gatheredMetric.Spec.Type))
	}
}

func (e *Evaluator) getObjectEvaluation(currentReplicas int32, gatheredMetric *metric.CombinedMetric) (*evaluate.Evaluation, error) {
	if gatheredMetric.Spec.Object.Target.Type == autoscaling.ValueMetricType {
		usageRatio := float64(gatheredMetric.Object.Utilization) / float64(gatheredMetric.Spec.Object.Target.Value.MilliValue())
		replicaCount := e.getUsageRatioReplicaCount(currentReplicas, usageRatio, *gatheredMetric.Object.ReadyPodCount)
		return &evaluate.Evaluation{
			TargetReplicas: &replicaCount,
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
			TargetReplicas: &replicaCount,
		}, nil
	}
	return nil, fmt.Errorf("invalid object metric source: neither a value target nor an average value target was set")
}

func (e *Evaluator) getPodsEvaluation(currentReplicas int32, gatheredMetric *metric.CombinedMetric) *evaluate.Evaluation {
	targetReplicas := e.getPlainMetricReplicaCount(
		gatheredMetric.Pods.PodMetricsInfo,
		currentReplicas,
		gatheredMetric.Spec.Pods.Target.Value.MilliValue(),
		gatheredMetric.Pods.ReadyPodCount,
		gatheredMetric.Resource.MissingPods,
		gatheredMetric.Resource.IgnoredPods,
	)
	return &evaluate.Evaluation{
		TargetReplicas: &targetReplicas,
	}
}

func (e *Evaluator) getResourceEvaluation(currentReplicas int32, gatheredMetric *metric.CombinedMetric) (*evaluate.Evaluation, error) {
	if gatheredMetric.Spec.Resource.Target.AverageValue != nil {
		replicaCount := e.getPlainMetricReplicaCount(
			gatheredMetric.Resource.PodMetricsInfo,
			currentReplicas,
			gatheredMetric.Spec.Object.Target.AverageValue.MilliValue(),
			gatheredMetric.Resource.ReadyPodCount,
			gatheredMetric.Resource.MissingPods,
			gatheredMetric.Resource.IgnoredPods,
		)
		return &evaluate.Evaluation{
			TargetReplicas: &replicaCount,
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

		rebalanceIgnored := len(ignoredPods) > 0 && usageRatio > 1.0
		if !rebalanceIgnored && len(missingPods) == 0 {
			if math.Abs(1.0-usageRatio) <= e.Tolerance {
				// return the current replicas if the change would be too small
				return &evaluate.Evaluation{
					TargetReplicas: &currentReplicas,
				}, nil
			}
			targetReplicas := int32(math.Ceil(usageRatio * float64(readyPodCount)))
			// if we don't have any unready or missing pods, we can calculate the new replica count now
			return &evaluate.Evaluation{
				TargetReplicas: &targetReplicas,
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
			return nil, err
		}

		if math.Abs(1.0-newUsageRatio) <= e.Tolerance || (usageRatio < 1.0 && newUsageRatio > 1.0) || (usageRatio > 1.0 && newUsageRatio < 1.0) {
			// return the current replicas if the change would be too small,
			// or if the new usage ratio would cause a change in scale direction
			return &evaluate.Evaluation{
				TargetReplicas: &currentReplicas,
			}, nil
		}

		// return the result, where the number of replicas considered is
		// however many replicas factored into our calculation
		targetReplicas := int32(math.Ceil(newUsageRatio * float64(len(metrics))))
		return &evaluate.Evaluation{
			TargetReplicas: &targetReplicas,
		}, nil
	}

	return nil, fmt.Errorf("invalid resource metric source: neither a utilization target nor a value target was set")
}

func (e *Evaluator) getExternalEvaluation(currentReplicas int32, gatheredMetric *metric.CombinedMetric) (*evaluate.Evaluation, error) {
	if gatheredMetric.Spec.External.Target.AverageValue != nil {
		utilization := gatheredMetric.External.Utilization
		targetUtilizationPerPod := gatheredMetric.Spec.External.Target.AverageValue.MilliValue()
		replicaCount := currentReplicas
		usageRatio := float64(utilization) / (float64(targetUtilizationPerPod) * float64(replicaCount))
		if math.Abs(1.0-usageRatio) > e.Tolerance {
			// update number of replicas if the change is large enough
			replicaCount = int32(math.Ceil(float64(utilization) / float64(targetUtilizationPerPod)))
		}
		return &evaluate.Evaluation{
			TargetReplicas: &replicaCount,
		}, nil
	}

	if gatheredMetric.Spec.External.Target.AverageUtilization != nil {
		replicaCount := currentReplicas

		utilization := gatheredMetric.External.Utilization
		targetUtilization := gatheredMetric.Spec.External.Target.Value.MilliValue()

		readyPodCount := gatheredMetric.External.ReadyPodCount

		usageRatio := float64(utilization) / float64(targetUtilization)
		replicaCount = e.getUsageRatioReplicaCount(currentReplicas, usageRatio, *readyPodCount)
		return &evaluate.Evaluation{
			TargetReplicas: &replicaCount,
		}, nil
	}
	return nil, fmt.Errorf("invalid external metric source: neither a value target nor an average value target was set")
}

func (e *Evaluator) getUsageRatioReplicaCount(currentReplicas int32, usageRatio float64, readyPodCount int64) int32 {
	var replicaCount int32
	if currentReplicas != 0 {
		if math.Abs(1.0-usageRatio) <= e.Tolerance {
			// return the current replicas if the change would be too small
			return currentReplicas
		}
		replicaCount = int32(math.Ceil(usageRatio * float64(readyPodCount)))
	} else {
		// Scale to zero or n pods depending on usageRatio
		replicaCount = int32(math.Ceil(usageRatio))
	}

	return replicaCount
}

func (e *Evaluator) getPlainMetricReplicaCount(metrics metricsclient.PodMetricsInfo,
	currentReplicas int32,
	targetUtilization int64,
	readyPodCount int64,
	missingPods,
	ignoredPods sets.String) int32 {

	usageRatio, _ := metricsclient.GetMetricUtilizationRatio(metrics, targetUtilization)

	rebalanceIgnored := len(ignoredPods) > 0 && usageRatio > 1.0

	if !rebalanceIgnored && len(missingPods) == 0 {
		if math.Abs(1.0-usageRatio) <= e.Tolerance {
			// return the current replicas if the change would be too small
			return currentReplicas
		}

		// if we don't have any unready or missing pods, we can calculate the new replica count now
		return int32(math.Ceil(usageRatio * float64(readyPodCount)))
	}

	if len(missingPods) > 0 {
		if usageRatio < 1.0 {
			// on a scale-down, treat missing pods as using 100% of the resource request
			for podName := range missingPods {
				metrics[podName] = metricsclient.PodMetric{Value: targetUtilization}
			}
		} else {
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
	newUsageRatio, _ := metricsclient.GetMetricUtilizationRatio(metrics, targetUtilization)

	if math.Abs(1.0-newUsageRatio) <= e.Tolerance || (usageRatio < 1.0 && newUsageRatio > 1.0) || (usageRatio > 1.0 && newUsageRatio < 1.0) {
		// return the current replicas if the change would be too small,
		// or if the new usage ratio would cause a change in scale direction
		return currentReplicas
	}

	// return the result, where the number of replicas considered is
	// however many replicas factored into our calculation
	return int32(math.Ceil(newUsageRatio * float64(len(metrics))))
}
