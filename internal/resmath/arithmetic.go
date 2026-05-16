package resmath

import "k8s.io/kubernetes/pkg/scheduler/framework"

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
