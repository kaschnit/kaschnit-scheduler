package queue

import (
	"maps"
	"math"

	"github.com/kaschnit/kaschnit-scheduler/internal/resconv"
	"github.com/kaschnit/kaschnit-scheduler/internal/resmath"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Quota tracks the max and available quota.
type Quota struct {
	// Max is the available resources.
	Max *framework.Resource
	// Used is the used resources.
	Used *framework.Resource
	// PodsByName are pods that currently contribute to quota.
	PodsByName map[types.UID]*corev1.Pod
}

// NewQuota creates a new [Quota].
func NewQuota(max corev1.ResourceList) *Quota {
	quota := &Quota{
		Used:       framework.NewResource(nil),
		PodsByName: make(map[types.UID]*corev1.Pod),
	}

	quota.SetMax(max)

	return quota
}

// SetMax sets the quota's max.
func (q *Quota) SetMax(max corev1.ResourceList) {
	if max == nil {
		max = corev1.ResourceList{}
	}

	// By default, we set "unlimited" quota for max.
	// In [framework.NewResource], unset results in effectively 0 which is not what we want.
	// So we explicitly set all of the vanilla resource types to unlimited if not explicitly set.
	// We can get away without setting "scalar" resources (i.e. extended resources) because those
	// are differentiable based on presence.
	resmath.SetDefault(max, corev1.ResourceCPU, resource.NewMilliQuantity(math.MaxInt64, resource.DecimalSI))
	resmath.SetDefault(max, corev1.ResourceMemory, resource.NewQuantity(math.MaxInt64, resource.BinarySI))
	resmath.SetDefault(max, corev1.ResourceEphemeralStorage, resource.NewQuantity(math.MaxInt64, resource.BinarySI))
	resmath.SetDefault(max, corev1.ResourcePods, resource.NewQuantity(math.MaxInt64, resource.BinarySI))

	q.Max = framework.NewResource(max)
}

// AddPodIfNotPresent adds the pod to the quota if it's not part of the quota.
func (q *Quota) AddPodIfNotPresent(pod *corev1.Pod) {
	if pod == nil {
		return
	}

	_, wasTrackingPod := q.PodsByName[pod.UID]

	q.PodsByName[pod.UID] = pod

	if !wasTrackingPod {
		resmath.AddInPlace(q.Used, resconv.FromPod(pod))
	}
}

// DeletePodIfPresent removes the pod from the quota if it's part of the quota.
func (q *Quota) DeletePodIfPresent(pod *corev1.Pod) {
	if pod == nil {
		return
	}

	if _, wasTrackingPod := q.PodsByName[pod.UID]; wasTrackingPod {
		delete(q.PodsByName, pod.UID)
		resmath.SubtractInPlace(q.Used, resconv.FromPod(pod))
	}
}

// WouldPutOverMax returns true if request would put the quota over its max
// when added to the used amount.
func (q *Quota) WouldPutOverMax(request *framework.Resource) bool {
	return resmath.AnyGreaterThanOnlyExisting(resmath.Add(q.Used, request), q.Max)
}

// Clone clones the [Quota].
func (q *Quota) Clone() *Quota {
	newQuotaUsage := &Quota{
		PodsByName: maps.Clone(q.PodsByName),
	}

	if q.Max != nil {
		newQuotaUsage.Max = q.Max.Clone()
	}
	if q.Used != nil {
		newQuotaUsage.Used = q.Used.Clone()
	}

	return newQuotaUsage
}
