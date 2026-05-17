package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type QueueQuotaConfig struct {
	// Max is the quota for this queue.
	// +optional
	Max corev1.ResourceList `json:"max"`
}

type QueuePreemptionConfig struct {
	// TargetQueues are the queues that this queue can preempt.
	// If this is omitted, the queue cannot preempt.
	// An explicity empty selector indicates the queue can preempt all queues.
	// +optional
	TargetQueues *metav1.LabelSelector `json:"targetQueues,omitempty"`
}

type QueueSpec struct {
	// Quota is the queue's quota settings.
	// +optional
	Quota QueueQuotaConfig `json:"quota,omitempty"`
	// Preemption is the queues' preemption settings.
	// +optional
	Preemption QueuePreemptionConfig `json:"preemption,omitempty"`
}

type QueueStatus struct{}

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
