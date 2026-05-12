package rescmp

import "k8s.io/kubernetes/pkg/scheduler/framework"

// AnyGreaterThanOnlyExisting returns true if a > b for any of a's resources.
// It only compares resource types that exist in both a and b.
func AnyGreaterThanOnlyExisting(a *framework.Resource, b *framework.Resource) bool {
	if a.Memory > b.Memory {
		return true
	}

	if a.MilliCPU > b.MilliCPU {
		return true
	}

	if a.EphemeralStorage > b.EphemeralStorage {
		return true
	}

	if a.AllowedPodNumber > b.AllowedPodNumber {
		return true
	}

	// Iterate over b so we only compare a to the existing scalar resources of b.
	for name, bValue := range b.ScalarResources {
		if aValue, ok := a.ScalarResources[name]; ok {
			if aValue > bValue {
				return true
			}
		}
	}

	return false
}
