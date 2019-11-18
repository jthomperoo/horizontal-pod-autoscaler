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

// Package metric provides metric gathering, in the same way that the Horizontal Pod Autoscaler gathers metrics,
// using the metrics APIs.
package metric

import (
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2beta2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	corelisters "k8s.io/client-go/listers/core/v1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	metricsclient "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
)

// Gatherer provides functionality for retrieving metrics on supplied metric specs.
type Gatherer struct {
	MetricsClient                 metricsclient.MetricsClient
	PodLister                     corelisters.PodLister
	CPUInitializationPeriod       time.Duration
	DelayOfInitialReadinessStatus time.Duration
}

// GetMetrics processes each MetricSpec provided, calculating metric values for each and combining them into a slice before returning them.
// Error will only be returned if all metrics are invalid, otherwise it will return the valid metrics.
func (c *Gatherer) GetMetrics(deployment *appsv1.Deployment, specs []autoscaling.MetricSpec, namespace string) ([]*CombinedMetric, error) {
	var combinedMetrics []*CombinedMetric
	var invalidMetricError error
	invalidMetricsCount := 0
	currentReplicas := int32(0)
	if deployment.Spec.Replicas != nil {
		currentReplicas = *deployment.Spec.Replicas
	}
	for _, spec := range specs {
		metric, err := c.getMetric(currentReplicas, spec, namespace, labels.Set(deployment.Labels).AsSelector())
		if err != nil {
			if invalidMetricsCount <= 0 {
				invalidMetricError = err
			}
			invalidMetricsCount++
			continue
		}
		combinedMetrics = append(combinedMetrics, metric)
	}

	// If all metrics are invalid return error and set condition on hpa based on first invalid metric.
	if invalidMetricsCount >= len(specs) {
		return nil, fmt.Errorf("invalid metrics (%v invalid out of %v), first error is: %v", invalidMetricsCount, len(specs), invalidMetricError)
	}

	return combinedMetrics, nil
}

func (c *Gatherer) getMetric(currentReplicas int32, spec autoscaling.MetricSpec, namespace string, selector labels.Selector) (*CombinedMetric, error) {
	switch spec.Type {
	case autoscaling.ObjectMetricSourceType:
		metricSelector, err := metav1.LabelSelectorAsSelector(spec.Object.Metric.Selector)
		if err != nil {
			return nil, fmt.Errorf("failed to get object metric: %v", err)
		}

		if spec.Object.Target.Type == autoscaling.ValueMetricType {
			objectMetric, err := c.getObjectMetric(spec.Object.Metric.Name, namespace, &spec.Object.DescribedObject, selector, metricSelector)
			if err != nil {
				return nil, fmt.Errorf("failed to get object metric: %v", err)
			}
			return &CombinedMetric{
				CurrentReplicas: currentReplicas,
				Spec:            spec,
				Object:          objectMetric,
			}, nil
		}

		if spec.Object.Target.Type == autoscaling.AverageValueMetricType {
			objectMetric, err := c.getObjectPerPodMetric(spec.Object.Metric.Name, namespace, &spec.Object.DescribedObject, selector)
			if err != nil {
				return nil, fmt.Errorf("failed to get object metric: %v", err)
			}
			return &CombinedMetric{
				CurrentReplicas: currentReplicas,
				Spec:            spec,
				Object:          objectMetric,
			}, nil
		}

		return nil, fmt.Errorf("invalid object metric source: neither a value target nor an average value target was set")

	case autoscaling.PodsMetricSourceType:
		metricSelector, err := metav1.LabelSelectorAsSelector(spec.Pods.Metric.Selector)
		if err != nil {
			return nil, fmt.Errorf("failed to get pods metric: %v", err)
		}

		podsMetric, err := c.getPodsMetric(spec.Pods.Metric.Name, namespace, selector, metricSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to get pods metric: %v", err)
		}
		return &CombinedMetric{
			CurrentReplicas: currentReplicas,
			Spec:            spec,
			Pods:            podsMetric,
		}, nil
	case autoscaling.ResourceMetricSourceType:
		if spec.Resource.Target.AverageValue != nil {
			resourceMetric, err := c.getRawResourceMetric(spec.Resource.Name, namespace, selector)
			if err != nil {
				return nil, fmt.Errorf("failed to get resource metric: %v", err)
			}
			return &CombinedMetric{
				CurrentReplicas: currentReplicas,
				Spec:            spec,
				Resource:        resourceMetric,
			}, nil
		}

		if spec.Resource.Target.AverageUtilization != nil {
			resourceMetric, err := c.getResourceMetric(spec.Resource.Name, namespace, selector)
			if err != nil {
				return nil, fmt.Errorf("failed to get resource metric: %v", err)
			}
			return &CombinedMetric{
				CurrentReplicas: currentReplicas,
				Spec:            spec,
				Resource:        resourceMetric,
			}, nil
		}

		return nil, fmt.Errorf("invalid resource metric source: neither a utilization target nor a value target was set")

	case autoscaling.ExternalMetricSourceType:
		if spec.External.Target.AverageValue != nil {
			externalMetric, err := c.getExternalPerPodMetrics(spec.External.Metric.Name, namespace, spec.External.Metric.Selector)
			if err != nil {
				return nil, fmt.Errorf("failed to get external metric: %v", err)
			}
			return &CombinedMetric{
				CurrentReplicas: currentReplicas,
				Spec:            spec,
				External:        externalMetric,
			}, nil
		}

		if spec.External.Target.AverageUtilization != nil {
			externalMetric, err := c.getExternalMetric(spec.External.Metric.Name, namespace, spec.External.Metric.Selector, selector)
			if err != nil {
				return nil, fmt.Errorf("failed to get external metric: %v", err)
			}
			return &CombinedMetric{
				CurrentReplicas: currentReplicas,
				Spec:            spec,
				External:        externalMetric,
			}, nil
		}
		return nil, fmt.Errorf("invalid external metric source: neither a value target nor an average value target was set")

	default:
		return nil, fmt.Errorf("unknown metric source type %q", string(spec.Type))
	}
}

func (c *Gatherer) getResourceMetric(resource v1.ResourceName, namespace string, selector labels.Selector) (*ResourceMetric, error) {
	// Get metrics
	metrics, timestamp, err := c.MetricsClient.GetResourceMetric(resource, namespace, selector)
	if err != nil {
		return nil, fmt.Errorf("unable to get metrics for resource %s: %v", resource, err)
	}

	// Get pods
	podList, err := c.PodLister.Pods(namespace).List(selector)
	if err != nil {
		return nil, fmt.Errorf("unable to get pods while calculating replica count: %v", err)
	}

	totalPods := len(podList)
	if totalPods == 0 {
		return nil, fmt.Errorf("No pods returned by selector while calculating replica count")
	}

	// Remove missing pod metrics
	readyPodCount, ignoredPods, missingPods := groupPods(podList, metrics, resource, c.CPUInitializationPeriod, c.DelayOfInitialReadinessStatus)
	removeMetricsForPods(metrics, ignoredPods)

	// Calculate requests - limits for pod resources
	requests, err := calculatePodRequests(podList, resource)
	if err != nil {
		return nil, err
	}

	return &ResourceMetric{
		PodMetricsInfo: metrics,
		Requests:       requests,
		ReadyPodCount:  int64(readyPodCount),
		IgnoredPods:    ignoredPods,
		MissingPods:    missingPods,
		TotalPods:      totalPods,
		Timestamp:      timestamp,
	}, nil
}

func (c *Gatherer) getRawResourceMetric(resource v1.ResourceName, namespace string, selector labels.Selector) (*ResourceMetric, error) {
	// Get metrics
	metrics, timestamp, err := c.MetricsClient.GetResourceMetric(resource, namespace, selector)
	if err != nil {
		return nil, fmt.Errorf("unable to get metrics for resource %s: %v", resource, err)
	}

	// Get pods
	podList, err := c.PodLister.Pods(namespace).List(selector)
	if err != nil {
		return nil, fmt.Errorf("unable to get pods while calculating replica count: %v", err)
	}

	totalPods := len(podList)
	if totalPods == 0 {
		return nil, fmt.Errorf("No pods returned by selector while calculating replica count")
	}

	// Remove missing pod metrics
	readyPodCount, ignoredPods, missingPods := groupPods(podList, metrics, resource, c.CPUInitializationPeriod, c.DelayOfInitialReadinessStatus)
	removeMetricsForPods(metrics, ignoredPods)

	return &ResourceMetric{
		PodMetricsInfo: metrics,
		ReadyPodCount:  int64(readyPodCount),
		IgnoredPods:    ignoredPods,
		MissingPods:    missingPods,
		TotalPods:      totalPods,
		Timestamp:      timestamp,
	}, nil
}

func (c *Gatherer) getPodsMetric(metricName string, namespace string, selector labels.Selector, metricSelector labels.Selector) (*PodsMetric, error) {
	// Get metrics
	metrics, timestamp, err := c.MetricsClient.GetRawMetric(metricName, namespace, selector, metricSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to get metric %s: %v", metricName, err)
	}

	// Get pods
	podList, err := c.PodLister.Pods(namespace).List(selector)
	if err != nil {
		return nil, fmt.Errorf("unable to get pods while calculating replica count: %v", err)
	}

	totalPods := len(podList)
	if totalPods == 0 {
		return &PodsMetric{
			ReadyPodCount: 0,
			TotalPods:     0,
			Timestamp:     timestamp,
		}, nil
	}

	// Remove missing pod metrics
	readyPodCount, ignoredPods, missingPods := groupPods(podList, metrics, v1.ResourceName(""), c.CPUInitializationPeriod, c.DelayOfInitialReadinessStatus)
	removeMetricsForPods(metrics, ignoredPods)

	return &PodsMetric{
		PodMetricsInfo: metrics,
		ReadyPodCount:  int64(readyPodCount),
		IgnoredPods:    ignoredPods,
		MissingPods:    missingPods,
		TotalPods:      totalPods,
		Timestamp:      timestamp,
	}, nil
}

func (c *Gatherer) getObjectMetric(metricName string, namespace string, objectRef *autoscaling.CrossVersionObjectReference, selector labels.Selector, metricSelector labels.Selector) (*ObjectMetric, error) {
	// Get metrics
	utilization, timestamp, err := c.MetricsClient.GetObjectMetric(metricName, namespace, objectRef, metricSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to get metric %s: %v on %s %s/%s", metricName, objectRef.Kind, namespace, objectRef.Name, err)
	}

	// Calculate number of ready pods
	readyPodCount, err := c.getReadyPodsCount(namespace, selector)
	if err != nil {
		return nil, fmt.Errorf("unable to calculate ready pods: %s", err)
	}

	return &ObjectMetric{
		Utilization:   utilization,
		ReadyPodCount: &readyPodCount,
		Timestamp:     timestamp,
	}, nil
}

func (c *Gatherer) getObjectPerPodMetric(metricName string, namespace string, objectRef *autoscaling.CrossVersionObjectReference, metricSelector labels.Selector) (*ObjectMetric, error) {
	// Get metrics
	utilization, timestamp, err := c.MetricsClient.GetObjectMetric(metricName, namespace, objectRef, metricSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to get metric %s: %v on %s %s/%s", metricName, objectRef.Kind, namespace, objectRef.Name, err)
	}

	return &ObjectMetric{
		Utilization: utilization,
		Timestamp:   timestamp,
	}, nil
}

func (c *Gatherer) getExternalMetric(metricName, namespace string, metricSelector *metav1.LabelSelector, podSelector labels.Selector) (*ExternalMetric, error) {
	// Convert selector to expected type
	metricLabelSelector, err := metav1.LabelSelectorAsSelector(metricSelector)
	if err != nil {
		return nil, err
	}

	// Get metrics
	metrics, timestamp, err := c.MetricsClient.GetExternalMetric(metricName, namespace, metricLabelSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to get external metric %s/%s/%+v: %s", namespace, metricName, metricSelector, err)
	}
	utilization := int64(0)
	for _, val := range metrics {
		utilization = utilization + val
	}

	// Calculate number of ready pods
	readyPodCount, err := c.getReadyPodsCount(namespace, podSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to calculate ready pods: %s", err)
	}

	return &ExternalMetric{
		Utilization:   utilization,
		ReadyPodCount: &readyPodCount,
		Timestamp:     timestamp,
	}, nil
}

func (c *Gatherer) getExternalPerPodMetrics(metricName, namespace string, metricSelector *metav1.LabelSelector) (*ExternalMetric, error) {
	// Convert selector to expected type
	metricLabelSelector, err := metav1.LabelSelectorAsSelector(metricSelector)
	if err != nil {
		return nil, err
	}

	// Get metrics
	metrics, timestamp, err := c.MetricsClient.GetExternalMetric(metricName, namespace, metricLabelSelector)
	if err != nil {
		return nil, fmt.Errorf("unable to get external metric %s/%s/%+v: %s", namespace, metricName, metricSelector, err)
	}

	// Calculate utilization total for pods
	utilization := int64(0)
	for _, val := range metrics {
		utilization = utilization + val
	}

	return &ExternalMetric{
		Utilization: utilization,
		Timestamp:   timestamp,
	}, nil
}

func (c *Gatherer) getReadyPodsCount(namespace string, selector labels.Selector) (int64, error) {
	// Get pods
	podList, err := c.PodLister.Pods(namespace).List(selector)
	if err != nil {
		return 0, fmt.Errorf("unable to get pods while calculating replica count: %v", err)
	}

	if len(podList) == 0 {
		return 0, nil
	}

	// Count number of ready pods
	readyPodCount := int64(0)
	for _, pod := range podList {
		if pod.Status.Phase == v1.PodRunning && podutil.IsPodReady(pod) {
			readyPodCount++
		}
	}

	return readyPodCount, nil
}

func groupPods(pods []*v1.Pod, metrics metricsclient.PodMetricsInfo, resource v1.ResourceName, cpuInitializationPeriod, delayOfInitialReadinessStatus time.Duration) (readyPodCount int, ignoredPods sets.String, missingPods sets.String) {
	missingPods = sets.NewString()
	ignoredPods = sets.NewString()
	for _, pod := range pods {
		if pod.DeletionTimestamp != nil || pod.Status.Phase == v1.PodFailed {
			continue
		}
		metric, found := metrics[pod.Name]
		if !found {
			missingPods.Insert(pod.Name)
			continue
		}
		if resource == v1.ResourceCPU {
			var ignorePod bool
			_, condition := podutil.GetPodCondition(&pod.Status, v1.PodReady)
			if condition == nil || pod.Status.StartTime == nil {
				ignorePod = true
			} else {
				// Pod still within possible initialisation period.
				if pod.Status.StartTime.Add(cpuInitializationPeriod).After(time.Now()) {
					// Ignore sample if pod is unready or one window of metric wasn't collected since last state transition.
					ignorePod = condition.Status == v1.ConditionFalse || metric.Timestamp.Before(condition.LastTransitionTime.Time.Add(metric.Window))
				} else {
					// Ignore metric if pod is unready and it has never been ready.
					ignorePod = condition.Status == v1.ConditionFalse && pod.Status.StartTime.Add(delayOfInitialReadinessStatus).After(condition.LastTransitionTime.Time)
				}
			}
			if ignorePod {
				ignoredPods.Insert(pod.Name)
				continue
			}
		}
		readyPodCount++
	}
	return
}

func calculatePodRequests(pods []*v1.Pod, resource v1.ResourceName) (map[string]int64, error) {
	requests := make(map[string]int64, len(pods))
	for _, pod := range pods {
		podSum := int64(0)
		for _, container := range pod.Spec.Containers {
			if containerRequest, ok := container.Resources.Requests[resource]; ok {
				podSum += containerRequest.MilliValue()
			} else {
				return nil, fmt.Errorf("missing request for %s", resource)
			}
		}
		requests[pod.Name] = podSum
	}
	return requests, nil
}

func removeMetricsForPods(metrics metricsclient.PodMetricsInfo, pods sets.String) {
	for _, pod := range pods.UnsortedList() {
		delete(metrics, pod)
	}
}
