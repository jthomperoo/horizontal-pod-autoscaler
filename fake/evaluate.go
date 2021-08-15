package fake

import (
	"github.com/jthomperoo/custom-pod-autoscaler/v2/evaluate"
	"github.com/jthomperoo/horizontal-pod-autoscaler/metric"
)

// ObjectEvaluater (fake) provides a way to insert functionality into an ObjectEvaluater
type ObjectEvaluater struct {
	GetEvaluationReactor func(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error)
}

// GetEvaluation calls the fake Evaluater function
func (f *ObjectEvaluater) GetEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error) {
	return f.GetEvaluationReactor(currentReplicas, gatheredMetric)
}

// PodsEvaluater (fake) provides a way to insert functionality into an PodsEvaluater
type PodsEvaluater struct {
	GetEvaluationReactor func(currentReplicas int32, gatheredMetric *metric.Metric) *evaluate.Evaluation
}

// GetEvaluation calls the fake Evaluater function
func (f *PodsEvaluater) GetEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) *evaluate.Evaluation {
	return f.GetEvaluationReactor(currentReplicas, gatheredMetric)
}

// ResourceEvaluater (fake) provides a way to insert functionality into an ResourceEvaluater
type ResourceEvaluater struct {
	GetEvaluationReactor func(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error)
}

// GetEvaluation calls the fake Evaluater function
func (f *ResourceEvaluater) GetEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error) {
	return f.GetEvaluationReactor(currentReplicas, gatheredMetric)
}

// ExternalEvaluater (fake) provides a way to insert functionality into an ExternalEvaluater
type ExternalEvaluater struct {
	GetEvaluationReactor func(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error)
}

// GetEvaluation calls the fake Evaluater function
func (f *ExternalEvaluater) GetEvaluation(currentReplicas int32, gatheredMetric *metric.Metric) (*evaluate.Evaluation, error) {
	return f.GetEvaluationReactor(currentReplicas, gatheredMetric)
}
