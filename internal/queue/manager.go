package queue

import (
	"errors"
	"fmt"
	"iter"
	"sync"

	"github.com/kaschnit/kaschnit-scheduler/apis/scheduling"

	corev1 "k8s.io/api/core/v1"
)

var (
	// ErrAddPodToQuota indicates an error adding pod to quota.
	ErrAddPodToQuota = errors.New("failed to add pod to quota")
	// ErrRemovePodFromQuota indicates an error removing pod from quota.
	ErrRemovePodFromQuota = errors.New("failed to remove pod from quota")
)

// Manager manages queues.
type Manager struct {
	sync.RWMutex
	queueByName map[string]*Queue
}

// NewManager creates a new [Manager].
func NewManager() *Manager {
	return &Manager{
		queueByName: make(map[string]*Queue),
	}
}

// Get gets the quota related to the pod, based on the pod's queue.
// If the pod is nil or has no queue, returns nil.
func (qm *Manager) Get(pod *corev1.Pod) *Queue {
	qm.RLock()
	defer qm.RUnlock()

	return qm.get(pod)
}

func (qm *Manager) get(pod *corev1.Pod) *Queue {
	if pod == nil {
		return nil
	}

	name, ok := pod.Labels[scheduling.LabelKeyQueue]
	if !ok {
		// Ignore pod if it has no queue, it will not be tracked.
		return nil
	}

	return qm.getByName(name)
}

func (qm *Manager) GetByName(name string) *Queue {
	qm.RLock()
	defer qm.RUnlock()

	return qm.getByName(name)
}

func (qm *Manager) getByName(name string) *Queue {
	return qm.queueByName[name]
}

// Put creates or updates the quota for the queue.
func (qm *Manager) Put(name string, opts ...QueueOption) {
	qm.Lock()
	defer qm.Unlock()

	qm.put(New(name, opts...))
}

func (qm *Manager) put(q *Queue) {
	if q == nil {
		return
	}

	qm.queueByName[q.Name()] = q
}

// Update mutates the queue with the given name.
// It will create the queue if it does not exist.
func (qm *Manager) Update(name string, opts ...QueueOption) {
	if len(opts) == 0 {
		return
	}

	qm.Lock()
	defer qm.Unlock()

	q := qm.getByName(name)
	if q == nil {
		qm.put(New(name, opts...))
		return
	}

	q.ApplyOpts(opts...)
}

func (qm *Manager) Delete(name string) {
	qm.Lock()
	defer qm.Unlock()

	qm.delete(name)
}

func (qm *Manager) delete(name string) {
	delete(qm.queueByName, name)
}

func (qm *Manager) QueueIter() iter.Seq[*Queue] {
	mgrClone := qm.Clone()

	return func(yield func(*Queue) bool) {
		for _, q := range mgrClone.queueByName {
			if !yield(q) {
				return
			}
		}
	}
}

// AddPodIfNotPresent adds the pod to the quota if the pod has a quota.
func (qm *Manager) AddPodIfNotPresent(pod *corev1.Pod) error {
	qm.Lock()
	defer qm.Unlock()

	return qm.addPodIfNotPresentNoLock(pod)
}

func (qm *Manager) addPodIfNotPresentNoLock(pod *corev1.Pod) error {
	if pod == nil {
		return nil
	}

	queueName, ok := pod.Labels[scheduling.LabelKeyQueue]
	if !ok {
		// Ignore pod if it has no queue, it will not be tracked.
		return nil
	}

	q, ok := qm.queueByName[queueName]
	if !ok {
		return fmt.Errorf("%w: queue '%s' does not exist", ErrAddPodToQuota, queueName)
	}

	q.Quota().AddPodIfNotPresent(pod)

	return nil
}

// DeletePodIfPresent removes the pod to the quota if the pod has a quota.
func (qm *Manager) DeletePodIfPresent(pod *corev1.Pod) error {
	qm.Lock()
	defer qm.Unlock()

	return qm.deletePodIfPresentNoLock(pod)
}

func (qm *Manager) deletePodIfPresentNoLock(pod *corev1.Pod) error {
	if pod == nil {
		return nil
	}

	queueName, ok := pod.Labels[scheduling.LabelKeyQueue]
	if !ok {
		// Ignore pod if it has no queue, it will not be tracked.
		return nil
	}

	q, ok := qm.queueByName[queueName]
	if !ok {
		return fmt.Errorf("%w: queue '%s' does not exist", ErrRemovePodFromQuota, queueName)
	}

	q.Quota().DeletePodIfPresent(pod)

	return nil
}

// Clone creates a clone of the [Manager].
func (qm *Manager) Clone() *Manager {
	qmClone := NewManager()

	qm.RLock()
	defer qm.RUnlock()

	for name, q := range qm.queueByName {
		qmClone.queueByName[name] = q.Clone()
	}

	return qmClone
}
