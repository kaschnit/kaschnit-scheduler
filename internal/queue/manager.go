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

// Manager manages quotas per queue.
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

// Set creates or updates the quota for the queeu.
func (qm *Manager) Set(queue *Queue) {
	qm.Lock()
	defer qm.Unlock()

	qm.set(queue)
}

func (qm *Manager) set(queue *Queue) {
	if queue != nil {
		qm.queueByName[queue.Name] = queue
	}
}

func (qm *Manager) Update(name string, mutate func(current *Queue) error) error {
	qm.Lock()
	defer qm.Unlock()

	current := qm.getByName(name)

	return mutate(current)
}

func (qm *Manager) Delete(name string) {
	qm.Lock()
	defer qm.Unlock()

	qm.delete(name)
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

func (qm *Manager) delete(name string) {
	delete(qm.queueByName, name)
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

	return q.Quota.AddPodIfNotPresent(pod)
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

	return q.Quota.DeletePodIfPresent(pod)
}

// Clone creates a clone of the [Manager].
func (qm *Manager) Clone() *Manager {
	qmClone := NewManager()

	qm.RLock()
	defer qm.RUnlock()

	for _, q := range qm.queueByName {
		qmClone.set(q.Clone())
	}

	return qmClone
}
