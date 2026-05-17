package queue

import (
	"math"

	"github.com/kaschnit/kaschnit-scheduler/internal/resconv"
	"github.com/kaschnit/kaschnit-scheduler/internal/resmath"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Quota tracks the max and available quota.
type Quota struct {
	// Max is the available resources.
	Max *framework.Resource
	// Used is the used resources.
	Used *framework.Resource
	// Track the pods that currently contribute to quota.
	// This helps with idempotency in quota tracking.
	pods sets.Set[string]
}

// NewQuota creates a new [Quota].
func NewQuota(max corev1.ResourceList) *Quota {
	if max == nil {
		max = corev1.ResourceList{
			corev1.ResourceCPU:              *resource.NewMilliQuantity(math.MaxInt64, resource.DecimalSI),
			corev1.ResourceMemory:           *resource.NewQuantity(math.MaxInt64, resource.BinarySI),
			corev1.ResourceEphemeralStorage: *resource.NewQuantity(math.MaxInt64, resource.BinarySI),
			corev1.ResourcePods:             *resource.NewQuantity(math.MaxInt64, resource.DecimalSI),
		}
	}

	return &Quota{
		Max:  framework.NewResource(max),
		Used: framework.NewResource(nil),
		pods: sets.New[string](),
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
	resmath.AddInPlace(q.Used, resconv.FromPod(pod))

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
	resmath.SubtractInPlace(q.Used, resconv.FromPod(pod))

	return nil
}

// WouldPutOverMax returns true if request would put the quota over its max
// when added to the used amount.
func (q *Quota) WouldPutOverMax(request *framework.Resource) bool {
	return resmath.AnyGreaterThanOnlyExisting(resmath.Add(q.Used, request), q.Max)
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
