package queue

import (
	"github.com/kaschnit/custom-scheduler/internal/rescmp"
	"github.com/kaschnit/custom-scheduler/internal/resconv"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Quota tracks the max and available quota.
type Quota struct {
	// Queue is the name of the queue.
	Queue string
	// Max is the available resources.
	Max *framework.Resource
	// Used is the used resources.
	Used *framework.Resource
	// Track the pods that currently contribute to quota.
	// This helps with idempotency in quota tracking.
	pods sets.Set[string]
}

func newQuota(queue string, max corev1.ResourceList) *Quota {
	return &Quota{
		Queue: queue,
		Max:   framework.NewResource(max),
		Used:  framework.NewResource(nil),
		pods:  sets.New[string](),
	}
}

// AddPodIfNotPresent adds the pod to the quota if it's not part of the quota.
func (q *Quota) AddPodIfNotPresent(pod *corev1.Pod) error {
	key, err := framework.GetPodKey(pod)
	if err != nil {
		return err
	}

	if q.pods.Has(key) {
		return nil
	}

	q.pods.Insert(key)
	resconv.AddFwkInPlace(q.Used, resconv.ExtractFwkFromPod(pod))

	return nil
}

// DeletePodIfPresent removes the pod from the quota if it's part of the quota.
func (q *Quota) DeletePodIfPresent(pod *corev1.Pod) error {
	key, err := framework.GetPodKey(pod)
	if err != nil {
		return err
	}

	if !q.pods.Has(key) {
		return nil
	}

	q.pods.Delete(key)
	resconv.SubtractFwkInPlace(q.Used, resconv.ExtractFwkFromPod(pod))

	return nil
}

// WouldPutOverMax returns true if request would put the quota over its max
// when added to the used amount.
func (q *Quota) WouldPutOverMax(request *framework.Resource) bool {
	return rescmp.AnyGreaterThanOnlyExisting(resconv.AddFwk(q.Used, request), q.Max)
}

// Clone clones the [Quota].
func (q *Quota) Clone() *Quota {
	newQuotaUsage := &Quota{
		pods: sets.New[string](),
	}

	if q.Max != nil {
		newQuotaUsage.Max = q.Max.Clone()
	}
	if q.Used != nil {
		newQuotaUsage.Used = q.Used.Clone()
	}

	for pod := range q.pods {
		newQuotaUsage.pods.Insert(pod)
	}

	return newQuotaUsage
}
