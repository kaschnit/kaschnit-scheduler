package queue

import (
	"errors"
	"fmt"
	"math"
	"sync"

	configv1 "github.com/kaschnit/custom-scheduler/apis/config/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/kubernetes/pkg/scheduler/framework"
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

	queue, ok := pod.Labels[configv1.LabelKeyQueue]
	if !ok {
		// Ignore pod if it has no queue, it will not be tracked.
		return nil
	}

	return qm.queueByName[queue]
}

// Set creates or updates the quota for the queeu.
func (qm *Manager) SetQuota(queueName string, max corev1.ResourceList) {
	if max == nil {
		max = corev1.ResourceList{
			corev1.ResourceCPU:              *resource.NewMilliQuantity(math.MaxInt64, resource.DecimalSI),
			corev1.ResourceMemory:           *resource.NewQuantity(math.MaxInt64, resource.BinarySI),
			corev1.ResourceEphemeralStorage: *resource.NewQuantity(math.MaxInt64, resource.BinarySI),
			corev1.ResourcePods:             *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
		}
	}

	qm.Lock()
	defer qm.Unlock()

	q := qm.queueByName[queueName]
	if q == nil {
		qm.queueByName[queueName] = &Queue{
			Name:  queueName,
			Quota: NewQuota(max),
		}
	} else {
		q.Quota.Max = framework.NewResource(max)
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

	queueName, ok := pod.Labels[configv1.LabelKeyQueue]
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

	queueName, ok := pod.Labels[configv1.LabelKeyQueue]
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
	quotaMgrClone := NewManager()

	qm.RLock()
	defer qm.RUnlock()

	for queueName, queue := range qm.queueByName {
		quotaMgrClone.queueByName[queueName] = queue.Clone()
	}

	return quotaMgrClone
}
