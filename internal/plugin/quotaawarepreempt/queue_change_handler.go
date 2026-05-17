package quotaawarepreempt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	schedv1 "github.com/kaschnit/kaschnit-scheduler/apis/scheduling/v1"
	schedv1client "github.com/kaschnit/kaschnit-scheduler/internal/generated/clients/scheduling/typed/scheduling/v1"
	schedv1informers "github.com/kaschnit/kaschnit-scheduler/internal/generated/informers/externalversions/scheduling/v1"
	"github.com/kaschnit/kaschnit-scheduler/internal/plugin/quotaawarepreempt/queue"
	"github.com/kaschnit/kaschnit-scheduler/internal/resconv"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// queueChangeHandler is used to handle queue changes to keep the plugin in sync.
type queueChangeHandler struct {
	queueMgr *queue.Manager
}

func newQueueChangeHandler(
	ctx context.Context,
	client schedv1client.SchedulingV1Interface,
	informer schedv1informers.QueueInformer,
	queueMgr *queue.Manager,
) (*queueChangeHandler, error) {
	logger := klog.FromContext(ctx)
	handler := queueChangeHandler{
		queueMgr: queueMgr,
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				for q := range queueMgr.QueueIter() {
					statusPatch := schedv1.Queue{
						Status: schedv1.QueueStatus{
							Quota: schedv1.QueueQuotaStatus{
								Used: resconv.ToResourceList(q.Quota.Used),
							},
						},
					}

					patchBytes, err := json.Marshal(statusPatch)
					if err != nil {
						logger.Error(err, "failed to convert status patch to bytes",
							"statusPatch", statusPatch)
						continue
					}

					if _, err := client.Queues().Patch(
						ctx,
						q.Name,
						types.MergePatchType,
						patchBytes,
						metav1.PatchOptions{},
						"status",
					); err != nil {
						logger.Error(err, "failed to patch status",
							"statusPatch", statusPatch)
						continue
					}
				}
			}
		}
	}()

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

	logger.Info("handling queue added",
		"queue", klog.KObj(queueObj))

	// TODO: stop clobbering the quota, this breaks things
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

	logger.Info("handling queue updated",
		"oldQueue", klog.KObj(oldQueue),
		"newQueue", klog.KObj(newQueue))

	if newQueue == nil {
		handler.queueMgr.Delete(oldQueue.Name)
		return
	}

	// TODO: stop clobbering the quota, this breaks things.
	// It's resetting the used quota being tracked.
	// We probably want queueMgr.Set to be less dumb, or have
	// distinct "Set" and "Update" methods or "UpdateMaxQuota" or smth.
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

	logger.Info("handling queue deleted",
		"queue", klog.KObj(queueObj))

	handler.queueMgr.Delete(queueObj.Name)

	// TODO: update other queues to stop targeting this one since
	// this queue no longer exists.
}
