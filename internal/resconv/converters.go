package resconv

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	reshelper "k8s.io/component-helpers/resource"
	fwk "k8s.io/kube-scheduler/framework"
	corev1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// ToResourceList converts [fwk.Resource] to [corev1.ResourceList].
func ToResourceList(r fwk.Resource) corev1.ResourceList {
	result := corev1.ResourceList{
		corev1.ResourceCPU:              *resource.NewMilliQuantity(r.GetMilliCPU(), resource.DecimalSI),
		corev1.ResourceMemory:           *resource.NewQuantity(r.GetMemory(), resource.BinarySI),
		corev1.ResourcePods:             *resource.NewQuantity(int64(r.GetAllowedPodNumber()), resource.BinarySI),
		corev1.ResourceEphemeralStorage: *resource.NewQuantity(r.GetEphemeralStorage(), resource.BinarySI),
	}
	for rName, rQuant := range r.GetScalarResources() {
		if corev1helper.IsHugePageResourceName(rName) {
			result[rName] = *resource.NewQuantity(rQuant, resource.BinarySI)
		} else {
			result[rName] = *resource.NewQuantity(rQuant, resource.DecimalSI)
		}
	}
	return result
}

// FromPod converts [corev1.Pod] to the [framework.Resource] its requests represent.
func FromPod(pod *corev1.Pod) *framework.Resource {
	result := &framework.Resource{}
	result.Add(reshelper.PodRequests(pod, reshelper.PodResourcesOptions{}))
	return result
}

// AddInPlace adds all of the provided resources to res.
// This function mutates res.
func AddInPlace(res *framework.Resource, others ...*framework.Resource) {
	for _, otherRes := range others {
		res.Memory += otherRes.Memory
		res.MilliCPU += otherRes.MilliCPU
		res.EphemeralStorage += otherRes.EphemeralStorage
		res.AllowedPodNumber += otherRes.AllowedPodNumber
		for name, value := range otherRes.ScalarResources {
			res.AddScalar(name, value)
		}
	}
}

// Add returns a sum all of the provided resources.
func Add(res *framework.Resource, others ...*framework.Resource) *framework.Resource {
	resCopy := res.Clone()

	AddInPlace(resCopy, others...)

	return resCopy
}

// AddInPlace subtracts all of the provided resources from res.
// This function mutates res.
func SubtractInPlace(res *framework.Resource, others ...*framework.Resource) {
	for _, otherRes := range others {
		res.Memory -= otherRes.Memory
		res.MilliCPU -= otherRes.MilliCPU
		res.EphemeralStorage -= otherRes.EphemeralStorage
		res.AllowedPodNumber -= otherRes.AllowedPodNumber
		for name, value := range otherRes.ScalarResources {
			res.AddScalar(name, -value)
		}
	}
}

// Subtract subtracts the provided resources from res.
// This function mutates res.
func Subtract(res *framework.Resource, others ...*framework.Resource) *framework.Resource {
	resCopy := res.Clone()

	SubtractInPlace(res, others...)

	return resCopy
}
