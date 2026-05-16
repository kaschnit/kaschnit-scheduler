package queue

type Queue struct {
	// Name is the name of the queue.
	Name string
	// Quota is the queue's quota.
	Quota *Quota
}

// Clone clones the [Queue].
func (q *Queue) Clone() *Queue {
	return &Queue{
		Name:  q.Name,
		Quota: q.Quota.Clone(),
	}
}
