package queue

import (
	"errors"
	"fmt"
	"math"
	"sync"

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

// QuotaManager manages quotas per queue.
type QuotaManager struct {
	sync.RWMutex
	quotaByQueue map[string]*Quota
}

// NewQuotaManager creates a new [QuotaManager].
func NewQuotaManager() *QuotaManager {
	return &QuotaManager{
		quotaByQueue: make(map[string]*Quota),
	}
}

// Get gets the quota related to the pod, based on the pod's queue.
// If the pod is nil or has no queue, returns nil.
func (qm *QuotaManager) Get(pod *corev1.Pod) *Quota {
	qm.RLock()
	defer qm.RUnlock()

	return qm.get(pod)
}

func (qm *QuotaManager) get(pod *corev1.Pod) *Quota {
	if pod == nil {
		return nil
	}

	queue, ok := pod.Labels[LabelKeyQueue]
	if !ok {
		// Ignore pod if it has no queue, it will not be tracked.
		return nil
	}

	return qm.quotaByQueue[queue]
}

// Set creates or updates the quota.
func (qm *QuotaManager) Set(queue string, max corev1.ResourceList) {
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

	quota := qm.quotaByQueue[queue]
	if quota == nil {
		qm.quotaByQueue[queue] = newQuota(queue, max)
	} else {
		quota.Max = framework.NewResource(max)
	}
}

// AddPodIfNotPresent adds the pod to the quota if the pod has a quota.
func (qm *QuotaManager) AddPodIfNotPresent(pod *corev1.Pod) error {
	qm.Lock()
	defer qm.Unlock()

	return qm.addPodIfNotPresent(pod)
}

func (qm *QuotaManager) addPodIfNotPresent(pod *corev1.Pod) error {
	if pod == nil {
		return nil
	}

	queue, ok := pod.Labels[LabelKeyQueue]
	if !ok {
		// Ignore pod if it has no queue, it will not be tracked.
		return nil
	}

	quota, ok := qm.quotaByQueue[queue]
	if !ok {
		return fmt.Errorf("%w: queue '%s' does not exist", ErrAddPodToQuota, queue)
	}

	return quota.AddPodIfNotPresent(pod)
}

// DeletePodIfPresent removes the pod to the quota if the pod has a quota.
func (qm *QuotaManager) DeletePodIfPresent(pod *corev1.Pod) error {
	qm.Lock()
	defer qm.Unlock()

	return qm.deletePodIfPresent(pod)
}

func (qm *QuotaManager) deletePodIfPresent(pod *corev1.Pod) error {
	if pod == nil {
		return nil
	}

	queue, ok := pod.Labels[LabelKeyQueue]
	if !ok {
		// Ignore pod if it has no queue, it will not be tracked.
		return nil
	}

	quota, ok := qm.quotaByQueue[queue]
	if !ok {
		return fmt.Errorf("%w: queue '%s' does not exist", ErrRemovePodFromQuota, queue)
	}

	return quota.DeletePodIfPresent(pod)
}

// Clone creates a clone of the [QuotaManager].
func (qm *QuotaManager) Clone() *QuotaManager {
	quotaMgrClone := NewQuotaManager()

	qm.RLock()
	defer qm.RUnlock()

	for queue, quota := range qm.quotaByQueue {
		quotaMgrClone.quotaByQueue[queue] = quota.Clone()
	}

	return quotaMgrClone
}
