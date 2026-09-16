package queue

import (
	"sync"

	"github.com/kaschnit/kaschnit-scheduler/apis/scheduling"
	"github.com/kaschnit/kaschnit-scheduler/internal/alloc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Queue reperesents a queue for pods.
type Queue struct {
	// name is the name of the queue.
	name string
	// Quota is the queue's quota.
	quota *Quota
	// labels are this queue's labels.
	labels labels.Labels
	// preemptionConfig is theis queue's preemption configuration.
	preemptionCfg PreemptionConfig

	lock sync.RWMutex
}

// New creates a new queue with the provided options.
func New(name string, opts ...QueueOption) *Queue {
	q := &Queue{
		name:          name,
		quota:         NewQuota(nil),
		labels:        labels.Set{},
		preemptionCfg: PreemptionConfig{},
	}

	q.ApplyOpts(opts...)

	return q
}

// Name returns the queue's name.
func (q *Queue) Name() string {
	if q == nil {
		return ""
	}

	q.lock.RLock()
	defer q.lock.RUnlock()

	return q.name
}

// Quota returns the queue's quota.
func (q *Queue) Quota() *Quota {
	if q == nil {
		return nil
	}

	q.lock.RLock()
	defer q.lock.RUnlock()

	return q.quota
}

// PreemptionConfig returns the queue's preemption config.
func (q *Queue) PreemptionConfig() PreemptionConfig {
	if q == nil {
		return PreemptionConfig{}
	}

	q.lock.RLock()
	defer q.lock.RUnlock()

	return q.preemptionCfg
}

// CanPodBePreemptedByOthers returns true if pod belonging to q is allowed to be preempted other pods.
// This is based only on the configuration of q and pod, not other queues or pods.
// Returns false if pod doesn't belong to q, regardless of configurations.
func (q *Queue) CanPodBePreemptedByOthers(pod *corev1.Pod) bool {
	if q == nil || pod == nil {
		return false
	}

	q.lock.RLock()
	defer q.lock.RUnlock()

	// Does pod belong to q?
	if pod.Labels[scheduling.LabelKeyQueue] != q.name {
		return false
	}

	// Can q be preempted at all?
	if !q.preemptionCfg.PreemptedBy.CanBePreempted() {
		return false
	}

	// Does q allow pod to be preempted by others?
	if !q.preemptionCfg.PreemptedBy.ToPods.Matches(labels.Set(pod.Labels)) {
		return false
	}

	return true
}

// Labels returns the queue's labels.
func (q *Queue) Labels() labels.Labels {
	if q == nil {
		return nil
	}

	q.lock.RLock()
	defer q.lock.RUnlock()

	if q.labels == nil {
		return make(labels.Set)
	}

	return q.labels
}

// ApplyOpts applies the queue options, mutation the queue.
func (q *Queue) ApplyOpts(opts ...QueueOption) {
	if q == nil {
		return
	}

	q.lock.Lock()
	defer q.lock.Unlock()

	for _, opt := range opts {
		opt(q)
	}
}

// Clone clones the [Queue].
func (q *Queue) Clone() *Queue {
	if q == nil {
		return nil
	}

	q.lock.RLock()
	defer q.lock.RUnlock()

	return &Queue{
		name:          q.name, // copy by value
		quota:         q.quota.Clone(),
		labels:        q.labels,        // treated as immutable
		preemptionCfg: q.preemptionCfg, // treated as immutable
	}
}

// QueueOption is an option that can be applied to configure [Queue].
type QueueOption func(*Queue)

// WithQuotaMax configures the max quota of the queue.
func WithQuotaMax(max alloc.Resources) QueueOption {
	return func(q *Queue) {
		if q == nil {
			return
		}

		q.quota.SetMax(max)
	}
}

// WithLabels sets the labels of the queue.
func WithLabels(lbls labels.Labels) QueueOption {
	return func(q *Queue) {
		if q == nil {
			return
		}

		if lbls == nil {
			lbls = make(labels.Set)
		}

		q.labels = lbls
	}
}

// WithPreemptionConfig sets the preemption config on the queue.
func WithPreemptionConfig(config PreemptionConfig) QueueOption {
	return func(q *Queue) {
		if q == nil {
			return
		}

		q.preemptionCfg = config
	}
}
