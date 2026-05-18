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
	queueMgr    *queue.Manager
	queueClient schedv1client.QueueInterface
}

func newQueueChangeHandler(
	ctx context.Context,
	queueClient schedv1client.QueueInterface,
	queueInformer schedv1informers.QueueInformer,
	queueMgr *queue.Manager,
) (*queueChangeHandler, error) {
	handler := queueChangeHandler{
		queueMgr:    queueMgr,
		queueClient: queueClient,
	}

	if _, err := queueInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    handler.addQueue,
		UpdateFunc: handler.updateQueue,
		DeleteFunc: handler.deleteQueue,
	}); err != nil {
		return nil, err
	}

	go handler.statusUpdateLoop(ctx)

	return &handler, nil
}

func (handler *queueChangeHandler) statusUpdateLoop(ctx context.Context) {
	logger := klog.FromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			for q := range handler.queueMgr.QueueIter() {
				statusPatch := schedv1.Queue{
					Status: schedv1.QueueStatus{
						Quota: schedv1.QueueQuotaStatus{
							// TODO: account for scalar resources in EffectiveMax.
							//	We can use a node informer to watch all resources known
							//	to nodes and set them in the EffectiveMax list if unset.
							// TODO: account for cluster capacity in EffectiveMax.
							//	We can use a node informer to watch all nodes and set
							//	the "effective max" to the sum of all resources.
							EffectiveMax: resconv.ToResourceList(q.Quota.Max),
							Used:         resconv.ToResourceList(q.Quota.Used),
						},
					},
				}

				patchBytes, err := json.Marshal(statusPatch)
				if err != nil {
					logger.Error(err, "failed to convert status patch to bytes",
						"statusPatch", statusPatch)
					continue
				}

				if _, err := handler.queueClient.Patch(
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

	if err := handler.queueMgr.Update(newQueue.Name, func(current *queue.Queue) error {
		if current == nil {
			return fmt.Errorf("attempted to update queue that does not exist")
		}

		current.Quota.SetMax(newQueue.Spec.Quota.Max)

		return nil
	}); err != nil {
		logger.Error(err, "failed to update queue",
			"queue", newQueue.Name)
	}
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
