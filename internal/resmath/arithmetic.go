package resmath

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

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

// SetDefault sets the quantity for the given resource if it's not present.
func SetDefault(
	res corev1.ResourceList,
	name corev1.ResourceName,
	quantity *resource.Quantity,
) {
	if _, ok := res[name]; !ok {
		res[name] = *quantity
	}
}
