package queue

import (
	"github.com/kaschnit/kaschnit-scheduler/internal/alloc"
	"github.com/kaschnit/kaschnit-scheduler/internal/labelutil"
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
	// victimSelector is the selector for victim queues.
	victimSelector labelutil.Matcher
}

// New creates a new queue with the provided options.
func New(name string, opts ...QueueOption) *Queue {
	q := &Queue{
		name:           name,
		quota:          NewQuota(nil),
		labels:         labels.Set{},
		victimSelector: labels.Nothing(),
	}

	q.ApplyOpts(opts...)

	return q
}

// Name returns the queue's name.
func (q *Queue) Name() string {
	return q.name
}

// Quota returns the queue's quota.
func (q *Queue) Quota() *Quota {
	return q.quota
}

func (q *Queue) IsVictimOf(other *Queue) bool {
	return other.VictimSelector().Matches(q.Labels())
}

// Labels returns the queue's labels.
func (q *Queue) Labels() labels.Labels {
	if q == nil || q.labels == nil {
		return make(labels.Set)
	}

	return q.labels
}

// VictimSelector returns the queue's victim queue selector.
func (q *Queue) VictimSelector() labelutil.Matcher {
	if q == nil || q.victimSelector == nil {
		return labels.Nothing()
	}

	return q.victimSelector
}

// ApplyOpts applies the queue options, mutation the queue.
func (q *Queue) ApplyOpts(opts ...QueueOption) {
	for _, opt := range opts {
		opt(q)
	}
}

// Clone clones the [Queue].
func (q *Queue) Clone() *Queue {
	if q == nil {
		return nil
	}

	return &Queue{
		name:           q.name,
		quota:          q.Quota().Clone(),
		labels:         q.labels,
		victimSelector: q.victimSelector,
	}
}

// QueueOption is an option that can be applied to configure [Queue].
type QueueOption func(*Queue)

// WithQuotaMax configures the max quota of the queue.
func WithQuotaMax(max alloc.Resources) QueueOption {
	return func(q *Queue) {
		q.quota.Max = max
	}
}

// WithLabels sets the labels of the queue.
func WithLabels(lbls labels.Labels) QueueOption {
	return func(q *Queue) {
		if lbls == nil {
			lbls = make(labels.Set)
		}

		q.labels = lbls
	}
}

// WithVictimSelector sets the victim selector of the queue.
func WithVictimSelector(victimSelector labelutil.Matcher) QueueOption {
	return func(q *Queue) {
		if victimSelector == nil {
			victimSelector = labels.Nothing()
		}

		q.victimSelector = victimSelector
	}
}
