package v1

import (
	"github.com/kaschnit/custom-scheduler/apis/scheduling"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// LabelKeyPrefix is the prefix of the labels for this plugin.
	LabelKeyPrefix = "quota." + scheduling.GroupName + "/"
	// LabelKeyQueue is the name of the label whose value is the queue.
	LabelKeyQueue = LabelKeyPrefix + "queue"
	// LabelKeyPreemptor specifies whether this pod can preempt other pods.
	// If unspecified, empty, or invalid, defaults to false (this pod cannot preempt).
	LabelKeyPreemptor = LabelKeyPrefix + "preemptor"
	// LabelKeyVictim specifies whether this pod can be preempted by other pods.
	// If unspecified, empty, or invalid, defaults to false (this pod cannot be preempted).
	LabelKeyVictim = LabelKeyPrefix + "victim"
)

type QueueConfig struct {
	// Quota is the quota config for this queue.
	Quota QueueQuotaConfig `json:"quota"`
}

type QueueQuotaConfig struct {
	// Max is the quota for this queue.
	Max corev1.ResourceList `json:"max"`
}

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

	// Queues are the queues by name.
	Queues map[string]QueueConfig `json:"queues"`
}
