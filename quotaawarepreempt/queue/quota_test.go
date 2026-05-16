package queue_test

import (
	"testing"

	"github.com/kaschnit/custom-scheduler/quotaawarepreempt/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

func TestQuota(t *testing.T) {
	t.Run("quota counting", func(t *testing.T) {
		quota := queue.NewQuota(nil)
		pod1 := newPodWithReq(corev1.ResourceList{corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI)})
		pod2 := newPodWithReq(corev1.ResourceList{corev1.ResourceCPU: *resource.NewQuantity(2, resource.DecimalSI)})

		t.Run("add pods to quota", func(t *testing.T) {
			// Add pod1 cpu
			err := quota.AddPodIfNotPresent(pod1)
			require.NoError(t, err)
			assert.Equal(t, framework.Resource{MilliCPU: 1000}, *quota.Used)

			// Add pod2 cpu
			err = quota.AddPodIfNotPresent(pod2)
			require.NoError(t, err)
			assert.Equal(t, framework.Resource{MilliCPU: 3000}, *quota.Used)

			t.Run("idempotent", func(t *testing.T) {
				// Add pod1 cpu again does nothing
				err := quota.AddPodIfNotPresent(pod1)
				require.NoError(t, err)
				assert.Equal(t, framework.Resource{MilliCPU: 3000}, *quota.Used) // No change
			})
		})

		t.Run("delete pods from quota", func(t *testing.T) {
			// Delete pod1 cpu
			err := quota.DeletePodIfPresent(pod1)
			require.NoError(t, err)
			assert.Equal(t, framework.Resource{MilliCPU: 2000}, *quota.Used)

			t.Run("idempotent", func(t *testing.T) {
				// Remove pod1 again does nothing
				err := quota.DeletePodIfPresent(pod1)
				require.NoError(t, err)
				assert.Equal(t, framework.Resource{MilliCPU: 2000}, *quota.Used) // No change
			})

			t.Run("delete random pod does nothing", func(t *testing.T) {
				podNotInQuota := newPodWithReq(corev1.ResourceList{corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI)})
				err := quota.DeletePodIfPresent(podNotInQuota)
				require.NoError(t, err)
				assert.Equal(t, framework.Resource{MilliCPU: 2000}, *quota.Used) // No change
			})

			// Delete pod2 cpu
			err = quota.DeletePodIfPresent(pod2)
			require.NoError(t, err)
			assert.Equal(t, framework.Resource{}, *quota.Used)

			t.Run("delete random pod from empty quota does nothing", func(t *testing.T) {
				podNotInQuota := newPodWithReq(corev1.ResourceList{corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI)})
				err := quota.DeletePodIfPresent(podNotInQuota)
				require.NoError(t, err)
				assert.Equal(t, framework.Resource{}, *quota.Used) // No change
			})
		})
	})

	t.Run("error on no pod UID", func(t *testing.T) {
		quota := queue.NewQuota(nil)
		pod := newPodWithReq(corev1.ResourceList{corev1.ResourceCPU: *resource.NewQuantity(1, resource.DecimalSI)})
		pod.UID = ""

		err := quota.AddPodIfNotPresent(pod)
		require.Error(t, err)
	})
}

func newPodWithReq(req corev1.ResourceList) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-" + string(uuid.NewUUID()),
			Namespace: "default",
			UID:       uuid.NewUUID(),
		},
		Spec: corev1.PodSpec{
			Resources: &corev1.ResourceRequirements{
				Requests: req,
			},
		},
	}
}
