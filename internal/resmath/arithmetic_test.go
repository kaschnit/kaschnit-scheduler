package resmath_test

import (
	"math"
	"testing"

	"github.com/kaschnit/kaschnit-scheduler/internal/resmath"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

func TestAdd(t *testing.T) {
	cpuOnlyRes := framework.Resource{MilliCPU: 1}
	cpuMemRes1 := framework.Resource{MilliCPU: 5, Memory: 3}
	cpuMemRes2 := framework.Resource{MilliCPU: 4, Memory: 100}
	gpuRes1 := framework.Resource{ScalarResources: map[corev1.ResourceName]int64{"nvidia.com/gpu": 8}}
	gpuRes2 := framework.Resource{ScalarResources: map[corev1.ResourceName]int64{"nvidia.com/gpu": 1}}

	t.Run("sum resources with some overlap", func(t *testing.T) {
		result := *resmath.Add(&cpuOnlyRes, &cpuMemRes1)
		assert.Equal(t, framework.Resource{
			MilliCPU: 6,
			Memory:   3,
		}, result)
	})

	t.Run("sum resources with strict overlap", func(t *testing.T) {
		result := *resmath.Add(&cpuMemRes1, &cpuMemRes2)
		assert.Equal(t, framework.Resource{
			MilliCPU: 9,
			Memory:   103,
		}, result)
	})

	t.Run("sum multiple resources with overlap and no overlap", func(t *testing.T) {
		result := *resmath.Add(&cpuOnlyRes, &cpuMemRes1, &gpuRes1, &gpuRes2)
		assert.Equal(t, framework.Resource{
			MilliCPU:        6,
			Memory:          3,
			ScalarResources: map[corev1.ResourceName]int64{"nvidia.com/gpu": 9},
		}, result)
	})
}

func TestSubtract(t *testing.T) {
	// TODO
}

func TestTakeEffectiveMax(t *testing.T) {
	testCases := []struct {
		name           string
		max            *framework.Resource
		totalAvailable *framework.Resource
		expected       *framework.Resource
	}{
		{
			name: "kitchen sink",
			max: &framework.Resource{
				MilliCPU:         123,
				EphemeralStorage: 100,
				Memory:           75,
				AllowedPodNumber: math.MaxInt64,
				ScalarResources: map[corev1.ResourceName]int64{
					"nvidia.com/gpu":                 5,
					"amd.com/gpu":                    22,
					"kaschnit.github.io/special-cpu": 7,
				},
			},
			totalAvailable: &framework.Resource{
				MilliCPU:         200, // Greater than max
				EphemeralStorage: 50,  // Less than max
				// No Memory specified here (specified in max)
				AllowedPodNumber: 12, // Not specified in max
				ScalarResources: map[corev1.ResourceName]int64{
					"nvidia.com/gpu": 30, // Greater than max
					// No amd.com/gpu specified here (specified in max)
					"kaschnit.github.io/special-cpu": 2,  // Less than max
					"kaschnit.github.io/special-gpu": 12, // Not specified in max
				},
			},
			expected: &framework.Resource{
				MilliCPU:         123, // max; because max < totalAvailable
				EphemeralStorage: 50,  // totalAvailable; because totalAvailable < max
				Memory:           0,   // 0; because unspecified in totalAvailable means 0
				AllowedPodNumber: 12,  // totalAvailable; because unspecified in max
				ScalarResources: map[corev1.ResourceName]int64{
					"nvidia.com/gpu":                 5,  // max; because max <totalAvailable
					"amd.com/gpu":                    0,  // 0; because unspecified in totalAvailable means 0
					"kaschnit.github.io/special-cpu": 2,  // totalAvailable; because totalAvailable < max
					"kaschnit.github.io/special-gpu": 12, // totalAvailable; because unspecified in max
				},
			},
		},
		{
			name: "none available",
			max: &framework.Resource{
				MilliCPU:         123,
				EphemeralStorage: 100,
				Memory:           75,
				AllowedPodNumber: math.MaxInt64,
				ScalarResources: map[corev1.ResourceName]int64{
					"nvidia.com/gpu":                 5,
					"amd.com/gpu":                    22,
					"kaschnit.github.io/special-cpu": 7,
				},
			},
			totalAvailable: &framework.Resource{},
			expected: &framework.Resource{
				ScalarResources: map[corev1.ResourceName]int64{
					"nvidia.com/gpu":                 0,
					"amd.com/gpu":                    0,
					"kaschnit.github.io/special-cpu": 0,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			effectiveMax := resmath.TakeEffectiveMax(testCase.max, testCase.totalAvailable)
			assert.Equal(t, testCase.expected, effectiveMax)
		})
	}
}
