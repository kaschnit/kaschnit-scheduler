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

	SubtractInPlace(resCopy, others...)

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

func TakeEffectiveMaxInPlace(
	max *framework.Resource,
	totalAvailable *framework.Resource,
) {
	max.Memory = min(max.Memory, totalAvailable.Memory)
	max.MilliCPU = min(max.MilliCPU, totalAvailable.MilliCPU)
	max.EphemeralStorage = min(max.EphemeralStorage, totalAvailable.EphemeralStorage)
	max.AllowedPodNumber = min(max.AllowedPodNumber, totalAvailable.AllowedPodNumber)
	for name, totalValue := range totalAvailable.ScalarResources {
		if maxValue, ok := max.ScalarResources[name]; ok {
			// If max defines this resource, its limit is the smaller of the defined max
			// and the total available.
			max.ScalarResources[name] = min(maxValue, totalValue)
		} else {
			// If max doesn't define this resource, its limit is the total available.
			max.ScalarResources[name] = totalValue
		}
	}
	for name := range max.ScalarResources {
		if _, ok := totalAvailable.ScalarResources[name]; ok {
			// If max defines this resource, its limit is the smaller of the defined max
			// and the total available.
			// However, we already calculated this in the previous loop.
		} else {
			// If total available is not defined, it means there are none available.
			// Therefore effective max is 0.
			max.ScalarResources[name] = 0
		}
	}
}

func TakeEffectiveMax(
	max *framework.Resource,
	totalAvailable *framework.Resource,
) *framework.Resource {
	maxCopy := max.Clone()

	TakeEffectiveMaxInPlace(maxCopy, totalAvailable)

	return maxCopy
}
