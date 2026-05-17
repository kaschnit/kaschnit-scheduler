package queue

import "k8s.io/apimachinery/pkg/util/sets"

// Queue reperesents a queue for pods.
type Queue struct {
	// Name is the name of the queue.
	Name string
	// Quota is the queue's quota.
	Quota *Quota
	// TargetQueues are the names of queues that can be preempted.
	TargetQueues sets.Set[string]
}

// Clone clones the [Queue].
func (q *Queue) Clone() *Queue {
	return &Queue{
		Name:  q.Name,
		Quota: q.Quota.Clone(),
	}
}
