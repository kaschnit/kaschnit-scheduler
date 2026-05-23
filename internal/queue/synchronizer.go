package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	schedv1 "github.com/kaschnit/kaschnit-scheduler/apis/scheduling/v1"
	"github.com/kaschnit/kaschnit-scheduler/internal/cluster"
	schedv1client "github.com/kaschnit/kaschnit-scheduler/internal/generated/clients/scheduling/typed/scheduling/v1"
	schedinformers "github.com/kaschnit/kaschnit-scheduler/internal/generated/informers/externalversions"
	"github.com/kaschnit/kaschnit-scheduler/internal/podutil"
	"github.com/kaschnit/kaschnit-scheduler/internal/resconv"
	"github.com/kaschnit/kaschnit-scheduler/internal/resmath"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type Synchronizer struct {
	queueMgr        *Manager
	queueClient     schedv1client.QueueInterface
	resourceCounter *cluster.ResourceTracker
}

func NewSynchronizer(
	ctx context.Context,
	queueMgr *Manager,
	informerFactory informers.SharedInformerFactory,
	queueClient schedv1client.QueueInterface,
	schedInformerFactory schedinformers.SharedInformerFactory,
) (*Synchronizer, error) {
	syn := Synchronizer{
		queueMgr:    queueMgr,
		queueClient: queueClient,
	}

	queueInformer := schedInformerFactory.Scheduling().V1().Queues().Informer()
	if _, err := queueInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    syn.addQueue,
		UpdateFunc: syn.updateQueue,
		DeleteFunc: syn.deleteQueue,
	}); err != nil {
		return nil, err
	}

	schedInformerFactory.Start(ctx.Done())
	schedInformerFactory.WaitForCacheSync(ctx.Done())

	podInformer := informerFactory.Core().V1().Pods().Informer()
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
				AddFunc:    syn.addPod,
				UpdateFunc: syn.updatePod,
				DeleteFunc: syn.deletePod,
			},
		},
	); err != nil {
		return nil, err
	}

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	resourceCounter, err := cluster.NewResourceTracker(ctx, informerFactory)
	if err != nil {
		return nil, err
	}

	syn.resourceCounter = resourceCounter

	go syn.statusUpdateLoop(ctx)

	return &syn, nil
}

func (syn *Synchronizer) statusUpdateLoop(ctx context.Context) {
	logger := klog.FromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			clusterTotalRes := syn.resourceCounter.GetTotal()

			for q := range syn.queueMgr.QueueIter() {
				effectiveMaxQuota := resmath.TakeEffectiveMax(q.Quota.Max, clusterTotalRes)

				statusPatch := schedv1.Queue{
					Status: schedv1.QueueStatus{
						Quota: schedv1.QueueQuotaStatus{
							// TODO: account for scalar resources in EffectiveMax.
							//	We can use a node informer to watch all resources known
							//	to nodes and set them in the EffectiveMax list if unset.
							// TODO: account for cluster capacity in EffectiveMax.
							//	We can use a node informer to watch all nodes and set
							//	the "effective max" to the sum of all resources.
							EffectiveMax: resconv.ToResourceList(effectiveMaxQuota),
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

				if _, err := syn.queueClient.Patch(
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

func (syn *Synchronizer) addQueue(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	queueObj, ok := obj.(*schedv1.Queue)
	if !ok {
		logger.Info("failed to handle queue added, got unexpected object",
			"obj", obj)
	}

	logger.Info("handling queue added",
		"queue", klog.KObj(queueObj))

	syn.queueMgr.Set(&Queue{
		Name:  queueObj.Name,
		Quota: NewQuota(queueObj.Spec.Quota.Max),
		// TODO: compute target queues on add.
		// Compute all queues that this queue targets, and recompute the
		// target queues of all other queues.
		TargetQueues: nil,
	})

	fmt.Printf("Queue Added: %s\n", queueObj.GetName())
}

func (syn *Synchronizer) updateQueue(oldObj, newObj any) {
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

	if err := syn.queueMgr.Update(newQueue.Name, func(current *Queue) error {
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

func (syn *Synchronizer) deleteQueue(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	queueObj, ok := obj.(*schedv1.Queue)
	if !ok {
		logger.Info("failed to handle queue deleted, got unexpected object",
			"obj", obj)
	}

	logger.Info("handling queue deleted",
		"queue", klog.KObj(queueObj))

	syn.queueMgr.Delete(queueObj.Name)

	// TODO: update other queues to stop targeting this one since
	// this queue no longer exists.
}

func (syn *Synchronizer) addPod(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		logger.Info("failed to handle pod added, got unexpected object",
			"obj", obj)
	}

	if err := syn.queueMgr.AddPodIfNotPresent(pod); err != nil {
		logger.Error(err, "Failed to add Pod to its associated quota",
			"pod", klog.KObj(pod))
	}
}

func (syn *Synchronizer) updatePod(oldObj, newObj any) {
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

	if err := syn.queueMgr.DeletePodIfPresent(newPod); err != nil {
		logger.Error(err, "Failed to delete Pod from its associated quota",
			"pod", klog.KObj(newPod))
	}
}

func (syn *Synchronizer) deletePod(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		logger.Info("failed to handle pod deleted, got unexpected object",
			"obj", obj)
	}

	if err := syn.queueMgr.DeletePodIfPresent(pod); err != nil {
		logger.Error(err, "Failed to delete Pod from its associated quota",
			"pod", klog.KObj(pod))
	}
}
