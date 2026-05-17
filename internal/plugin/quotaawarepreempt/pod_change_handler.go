package quotaawarepreempt

import (
	"context"

	"github.com/kaschnit/kaschnit-scheduler/internal/plugin/quotaawarepreempt/queue"
	"github.com/kaschnit/kaschnit-scheduler/internal/podutil"
	corev1 "k8s.io/api/core/v1"
	informerv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// podChangeHandler is used to handle pod changes to keep the plugin in sync.
type podChangeHandler struct {
	queueMgr *queue.Manager
}

func newPodChangeHandler(
	informer informerv1.PodInformer,
	queueMgr *queue.Manager,
) (*podChangeHandler, error) {
	handler := podChangeHandler{
		queueMgr: queueMgr,
	}

	podInformer := informer.Informer()
	if _, err := podInformer.AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj any) bool {
				switch t := obj.(type) {
				case *corev1.Pod:
					return len(t.Spec.NodeName) > 0
				case cache.DeletedFinalStateUnknown:
					if pod, ok := t.Obj.(*corev1.Pod); ok {
						return len(pod.Spec.NodeName) > 0
					}
					return false
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    handler.addPod,
				UpdateFunc: handler.updatePod,
				DeleteFunc: handler.deletePod,
			},
		},
	); err != nil {
		return nil, err
	}

	return &handler, nil
}

func (handler *podChangeHandler) addPod(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		logger.Info("failed to handle pod added, got unexpected object",
			"obj", obj)
	}

	if err := handler.queueMgr.AddPodIfNotPresent(pod); err != nil {
		logger.Error(err, "Failed to add Pod to its associated quota",
			"pod", klog.KObj(pod))
	}
}

func (handler *podChangeHandler) updatePod(oldObj, newObj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	oldPod, ok := oldObj.(*corev1.Pod)
	if !ok {
		logger.Info("failed to handle pod updated, got unexpected old object",
			"oldObj", oldObj)
	}

	newPod, ok := newObj.(*corev1.Pod)
	if !ok {
		logger.Info("failed to handle pod updated, got unexpected new object",
			"newObj", newObj)
	}

	if podutil.IsTerminal(oldPod.Status.Phase) || podutil.IsNonTerminal(newPod.Status.Phase) {
		return
	}

	if err := handler.queueMgr.DeletePodIfPresent(newPod); err != nil {
		logger.Error(err, "Failed to delete Pod from its associated quota",
			"pod", klog.KObj(newPod))
	}
}

func (handler *podChangeHandler) deletePod(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		logger.Info("failed to handle pod added, got unexpected object",
			"obj", obj)
	}

	if err := handler.queueMgr.DeletePodIfPresent(pod); err != nil {
		logger.Error(err, "Failed to delete Pod from its associated quota",
			"pod", klog.KObj(pod))
	}
}
