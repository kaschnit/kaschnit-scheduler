package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type QueueQuotaConfig struct {
	// Max is the max quota for this queue.
	// +optional
	Max corev1.ResourceList `json:"max,omitempty"`
}

type QueueQuotaStatus struct {
	// EffectiveMax is the effective max resource for this queue.
	// This takes into account unset resources, cluster capacity, etc.
	// +optional
	EffectiveMax corev1.ResourceList `json:"effectiveMax,omitempty"`
	// Used is the used quota for this queue.
	// +optional
	Used corev1.ResourceList `json:"used,omitempty"`
}

type QueuePreemptionConfig struct {
	// VictimQueues are the queues that this queue can preempt.
	// If this is omitted, the queue cannot preempt.
	// An explicity empty selector indicates the queue can preempt all queues.
	// +optional
	VictimQueues *metav1.LabelSelector `json:"victimQueues,omitempty"`
}

type QueueSpec struct {
	// Quota is the queue's quota settings.
	// +optional
	Quota QueueQuotaConfig `json:"quota,omitempty"`
	// Preemption is the queues' preemption settings.
	// +optional
	Preemption QueuePreemptionConfig `json:"preemption,omitempty"`
}

type QueueStatus struct {
	// Quota is the queue's quota status.
	// +optional
	Quota QueueQuotaStatus `json:"quota,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

type Queue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec is the queue's specification.
	Spec QueueSpec `json:"spec,omitempty"`
	// Status is the queue's status.
	Status QueueStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items are the queues in this list.
	Items []Queue `json:"items"`
}
