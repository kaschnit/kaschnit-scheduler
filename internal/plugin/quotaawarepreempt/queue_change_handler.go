package quotaawarepreempt

import (
	"context"
	"fmt"

	schedv1 "github.com/kaschnit/kaschnit-scheduler/apis/scheduling/v1"
	schedv1informers "github.com/kaschnit/kaschnit-scheduler/internal/generated/informers/externalversions/scheduling/v1"
	"github.com/kaschnit/kaschnit-scheduler/internal/plugin/quotaawarepreempt/queue"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// queueChangeHandler is used to handle queue changes to keep the plugin in sync.
type queueChangeHandler struct {
	queueMgr *queue.Manager
}

func newQueueChangeHandler(
	informer schedv1informers.QueueInformer,
	queueMgr *queue.Manager,
) (*queueChangeHandler, error) {
	handler := queueChangeHandler{
		queueMgr: queueMgr,
	}

	// TODO: start a goroutine which updates queue status periodically

	queueInformer := informer.Informer()
	if _, err := queueInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    handler.addQueue,
		UpdateFunc: handler.updateQueue,
		DeleteFunc: handler.deleteQueue,
	}); err != nil {
		return nil, err
	}

	return &handler, nil
}

func (handler *queueChangeHandler) addQueue(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	queueObj, ok := obj.(*schedv1.Queue)
	if !ok {
		logger.Info("failed to handle queue added, got unexpected object",
			"obj", obj)
	}

	handler.queueMgr.Set(&queue.Queue{
		Name:  queueObj.Name,
		Quota: queue.NewQuota(queueObj.Spec.Quota.Max),
		// TODO: compute target queues on add.
		// Compute all queues that this queue targets, and recompute the
		// target queues of all other queues.
		TargetQueues: nil,
	})

	fmt.Printf("Queue Added: %s\n", queueObj.GetName())
}

func (handler *queueChangeHandler) updateQueue(oldObj, newObj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	oldQueue, ok := oldObj.(*schedv1.Queue)
	if !ok {
		logger.Info("failed to handle pod updated, got unexpected old object",
			"oldObj", oldObj)
	}

	newQueue, ok := newObj.(*schedv1.Queue)
	if !ok {
		logger.Info("failed to handle pod updated, got unexpected new object",
			"newObj", newObj)
	}

	if newQueue == nil {
		handler.queueMgr.Delete(oldQueue.Name)
		return
	}

	handler.queueMgr.Set(&queue.Queue{
		Name:  newQueue.Name,
		Quota: queue.NewQuota(newQueue.Spec.Quota.Max),
		// TODO: recompute target queues on update.
		// If this queue's target selector were changed: update this queue's targets.
		// If this queue's labels were changed: update other queue's targets.
		TargetQueues: nil,
	})
}

func (handler *queueChangeHandler) deleteQueue(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	queueObj, ok := obj.(*schedv1.Queue)
	if !ok {
		logger.Info("failed to handle queue deleted, got unexpected object",
			"obj", obj)
	}

	handler.queueMgr.Delete(queueObj.Name)

	// TODO: update other queues to stop targeting this one since
	// this queue no longer exists.
}
