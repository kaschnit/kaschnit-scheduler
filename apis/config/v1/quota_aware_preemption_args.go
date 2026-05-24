package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PreemptionConfig struct {
	// MaxCandidateNodes is the maximum number of victim nodes to collect for preemption.
	// This can be used to reduce the node search space for the best node to preempt.
	// If not provided, the absolute number of preemption victim nodes to score is
	// not bounded.
	MaxCandidateNodes *int32 `json:"maxCandidateNodes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:skip

// QuotaAwarePreemptionArgs holds arguments used to configure QuotaAwarePreemptio plugin.
type QuotaAwarePreemptionArgs struct {
	metav1.TypeMeta `json:",inline"`

	// Preemption is the preemption config.
	Preemption *PreemptionConfig `json:"preemption,omitempty"`
}
