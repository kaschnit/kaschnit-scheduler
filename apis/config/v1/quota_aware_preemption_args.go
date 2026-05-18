package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PreemptionConfig struct {
	// EnableAsyncPreemption is whether to enable async preemption.
	// Defaults to false.
	EnableAsyncPreemption bool `json:"enableAsyncPreemption"`
	// PercentageOfNodesToScore is the percentage of nodes to score.
	// This can be used to reduce the ndoe search space for the best node to preempt.
	// If not provided, 100% of nodes will be used.
	PercentageOfNodesToScore *float32 `json:"percentageOfNodesToScore"`
	// MaxNodesToScore is the maximum number of nodes to score.
	// This limits the configured PercentageOfNodesToScore.
	// If not provided, the absolute number of preemption candidates to score is
	// not bounded.
	MaxNodesToScore *int32 `json:"maxNodesToScore"`
}

// +kubebuilder:object:root=true
// +kubebuilder:skip

// QuotaAwarePreemptionArgs holds arguments used to configure QuotaAwarePreemptio plugin.
type QuotaAwarePreemptionArgs struct {
	metav1.TypeMeta `json:",inline"`

	// EnableAsyncPreemption is whether to enable async preemption.
	// Defaults to false.
	Preemption PreemptionConfig `json:"enableAsyncPreemption"`
}
