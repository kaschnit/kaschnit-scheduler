package resmath_test

import (
	"testing"

	"github.com/kaschnit/kaschnit-scheduler/internal/resmath"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

func TestArithmetic(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
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
	})

	t.Run("Subtract", func(t *testing.T) {
		// TODO
	})
}
