package quotaawarepreempt

import (
	"context"
	"fmt"
	"sync"

	configv1 "github.com/kaschnit/custom-scheduler/apis/config/v1"
	"github.com/kaschnit/custom-scheduler/internal/podutil"
	"github.com/kaschnit/custom-scheduler/internal/resconv"
	"github.com/kaschnit/custom-scheduler/quotaawarepreempt/queue"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	corev1helpers "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/klog/v2"
	fwk "k8s.io/kube-scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/preemption"
	schedruntime "k8s.io/kubernetes/pkg/scheduler/framework/runtime"
	"k8s.io/kubernetes/pkg/scheduler/metrics"
)

const (
	// PluginName is the name of the scheduling plugin.
	PluginName = "QuotaAwarePreemption"
)

// Plugin is a kube-scheduler framework plugin for quota-aware preemption.
type Plugin struct {
	sync.RWMutex
	queueMgr queue.Manager
	logger   klog.Logger
	fh       fwk.Handle
	args     configv1.QuotaAwarePreemptionArgs
}

var (
	_ fwk.PreFilterPlugin     = (*Plugin)(nil)
	_ fwk.PreFilterExtensions = (*Plugin)(nil)
	_ fwk.PostFilterPlugin    = (*Plugin)(nil)
	_ fwk.ReservePlugin       = (*Plugin)(nil)
	_ fwk.EnqueueExtensions   = (*Plugin)(nil)
)

// NewPlugin initializes a new [Plugin] and returns it.
func NewPlugin(ctx context.Context, rawArgs runtime.Object, fh fwk.Handle) (fwk.Plugin, error) {
	logger := klog.FromContext(ctx).WithValues("plugin", PluginName)

	var args configv1.QuotaAwarePreemptionArgs
	if err := schedruntime.DecodeInto(rawArgs, &args); err != nil {
		return nil, err
	}

	plugin := Plugin{
		queueMgr: *queue.NewManager(),
		logger:   logger,
		fh:       fh,
		args:     args,
	}

	logger.Info("Setting up pod informer for plugin")

	podInformer := fh.SharedInformerFactory().Core().V1().Pods().Informer()
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
				AddFunc:    plugin.informerAddPod,
				UpdateFunc: plugin.informerUpdatePod,
				DeleteFunc: plugin.informerDeletePod,
			},
		},
	); err != nil {
		return nil, err
	}

	logger.Info("Setting up queues for plugin")

	// Init quotas from scheduler config.
	// TODO: move to a custom resource with informer to make this nicer.
	//	Custom resource will allow updating quotas without restarting the scheduler.
	//	It will also allow easier configuration generally.
	for name, queue := range args.Queues {
		plugin.queueMgr.SetQuota(name, queue.Quota.Max)
	}

	logger.Info("Initialized plugin")

	return &plugin, nil
}

// Name returns name of the plugin.
func (plugin *Plugin) Name() string {
	return PluginName
}

// PreFilter implements [framework.PreFilterPlugin].
func (plugin *Plugin) PreFilter(
	ctx context.Context,
	state fwk.CycleState,
	pod *corev1.Pod,
	nodes []fwk.NodeInfo,
) (*fwk.PreFilterResult, *fwk.Status) {
	stateMgr := NewStateManager(state)

	qSnapshot := &QueueSnapshotState{
		QueueMgr: plugin.queueMgr.Clone(),
	}
	stateMgr.WriteQueueSnapshot(qSnapshot)

	podReq := resconv.FromPod(pod)

	podQ := qSnapshot.QueueMgr.Get(pod)
	if podQ == nil {
		stateMgr.WriteRequestedResource(&RequstedResourceState{
			Request: *podReq,
		})
		return nil, fwk.NewStatus(fwk.Success)
	}

	nodeList, err := plugin.fh.SnapshotSharedLister().NodeInfos().List()
	if err != nil {
		return nil, fwk.NewStatus(fwk.Error, fmt.Sprintf("Error getting the node list: %v", err))
	}

	var nominatedReqInQuota framework.Resource
	for _, node := range nodeList {
		nominatedPods := plugin.fh.NominatedPodsForNode(node.Node().Name)
		for _, nomPodInfo := range nominatedPods {
			if nomPodInfo.GetPod().UID == pod.UID {
				continue
			}

			nomQ := qSnapshot.QueueMgr.Get(nomPodInfo.GetPod())
			if nomQ != nil {
				nomResourceRequest := resconv.ToResourceList(resconv.FromPod(nomPodInfo.GetPod()))
				// If they are subject to the same quota and nomPod is scheduled ahead of (higher priority than) pod,
				// nomPod will be added to the nominatedReqInQuota.
				// If they aren't subject to the same quota and the usage of nomQuota does not exceed min,
				// p will be added to the totalNominatedResource.
				if nomQ.Name == podQ.Name && corev1helpers.PodPriority(nomPodInfo.GetPod()) >= corev1helpers.PodPriority(pod) {
					nominatedReqInQuota.Add(nomResourceRequest)
				}
			}
		}
	}

	stateMgr.WriteRequestedResource(&RequstedResourceState{
		Request:             *podReq,
		NominatedReqInQuota: nominatedReqInQuota,
	})

	if podQ.Quota.WouldPutOverMax(resconv.Add(&nominatedReqInQuota, podReq)) {
		return nil, fwk.NewStatus(fwk.Unschedulable,
			fmt.Sprintf("Not eligible for scheduling because queue %s exceeds quota (used=%+v, max=%+v)",
				podQ.Name, podQ.Quota.Used, podQ.Quota.Max))
	}

	return nil, fwk.NewStatus(fwk.Success, "")
}

// PreFilterExtensions implements [framework.PreFilterPlugin].
func (plugin *Plugin) PreFilterExtensions() fwk.PreFilterExtensions {
	return plugin
}

// PostFilter implements [framework.PostFilterPlugin].
func (plugin *Plugin) PostFilter(
	ctx context.Context,
	state fwk.CycleState,
	pod *corev1.Pod,
	m fwk.NodeToStatusReader,
) (*fwk.PostFilterResult, *fwk.Status) {
	logger := klog.FromContext(klog.NewContext(ctx, plugin.logger))
	logger.Info("Running PostFilter",
		"pod", klog.KObj(pod))

	defer metrics.PreemptionAttempts.Inc()

	evaluator := preemption.NewEvaluator(
		plugin.Name(),
		plugin.fh,
		&preemptor{
			logger:   plugin.logger,
			fh:       plugin.fh,
			stateMgr: NewStateManager(state),
		},
		plugin.args.Preemption.EnableAsyncPreemption,
	)

	result, status := evaluator.Preempt(ctx, state, pod, m)
	logger.Info("Got preemption evaluation result for pod",
		"pod", klog.KObj(pod),
		"result", result,
		"status", status)

	return result, status
}

// AddPod implements [framework.PreFilterExtensions].
func (plugin *Plugin) AddPod(
	ctx context.Context,
	state fwk.CycleState,
	podToSchedule *corev1.Pod,
	podInfoToAdd fwk.PodInfo,
	nodeInfo fwk.NodeInfo,
) *fwk.Status {
	logger := klog.FromContext(klog.NewContext(ctx, plugin.logger)).WithValues(
		"podToSchedule", klog.KObj(podToSchedule),
		"podToAdd", klog.KObj(podInfoToAdd.GetPod()))

	stateMgr := NewStateManager(state)

	quotaSnapshot, err := stateMgr.ReadQueueSnapshot()
	if err != nil {
		logger.Error(err, "Failed to read quotaSnapshotState from cycleState")
		return fwk.NewStatus(fwk.Error, err.Error())
	}

	if err := quotaSnapshot.QueueMgr.AddPodIfNotPresent(podInfoToAdd.GetPod()); err != nil {
		logger.Error(err, "Failed to add Pod to its associated quota usage",
			"pod", klog.KObj(podInfoToAdd.GetPod()))
	}

	return fwk.NewStatus(fwk.Success, "")
}

// RemovePod implements [framework.PreFilterExtensions].
func (plugin *Plugin) RemovePod(
	ctx context.Context,
	state fwk.CycleState,
	podToSchedule *corev1.Pod,
	podInfoToRemove fwk.PodInfo,
	nodeInfo fwk.NodeInfo,
) *fwk.Status {
	logger := klog.FromContext(klog.NewContext(ctx, plugin.logger)).WithValues(
		"podToSchedule", klog.KObj(podToSchedule),
		"podToRemove", klog.KObj(podInfoToRemove.GetPod()))

	stateMgr := NewStateManager(state)

	quotaSnapshot, err := stateMgr.ReadQueueSnapshot()
	if err != nil {
		logger.Error(err, "Failed to read quotaSnapshotState from cycleState")
		return fwk.NewStatus(fwk.Error, err.Error())
	}

	if err := quotaSnapshot.QueueMgr.DeletePodIfPresent(podInfoToRemove.GetPod()); err != nil {
		logger.Error(err, "Failed to delete Pod from its associated quota usage",
			"pod", klog.KObj(podInfoToRemove.GetPod()))
	}

	return fwk.NewStatus(fwk.Success, "")
}

// Reserve implements [framework.ReservePlugin].
func (plugin *Plugin) Reserve(ctx context.Context, state fwk.CycleState, pod *corev1.Pod, nodeName string) *fwk.Status {
	logger := klog.FromContext(klog.NewContext(ctx, plugin.logger)).WithValues("ExtensionPoint", "Reserve")

	if err := plugin.queueMgr.AddPodIfNotPresent(pod); err != nil {
		logger.Error(err, "Failed to add Pod to its associated queue quota", "pod", klog.KObj(pod))
		return fwk.NewStatus(fwk.Error, err.Error())
	}

	return fwk.NewStatus(fwk.Success, "")
}

// Unreserve implements [framework.ReservePlugin].
func (plugin *Plugin) Unreserve(ctx context.Context, state fwk.CycleState, pod *corev1.Pod, nodeName string) {
	logger := klog.FromContext(klog.NewContext(ctx, plugin.logger)).WithValues("ExtensionPoint", "Reserve")

	if err := plugin.queueMgr.DeletePodIfPresent(pod); err != nil {
		logger.Error(err, "Failed to remove Pod from its associated queue quota", "pod", klog.KObj(pod))
	}
}

// EventsToRegister implements [framework.EnqueueExtensions].
func (plugin *Plugin) EventsToRegister(_ context.Context) ([]fwk.ClusterEventWithHint, error) {
	// Return the events that may cause pods that this plugin failed to becomes schedulable.
	// This seems like it might have a bug related which causes events to not move pods off of the
	// unschedulable queue.
	// See: https://github.com/kubernetes/kubernetes/issues/110175
	// See: https://github.com/kubernetes/kubernetes/issues/87850
	return []fwk.ClusterEventWithHint{
		// Changes to a pod may cause previously unschedulable pods to become schedulable.
		{
			Event: fwk.ClusterEvent{
				Resource:   fwk.Pod,
				ActionType: fwk.Update | fwk.Delete,
			},
			QueueingHintFn: func(logger klog.Logger, pod *corev1.Pod, oldObj, newObj interface{}) (fwk.QueueingHint, error) {
				oldPod, _ := oldObj.(*corev1.Pod)
				newPod, _ := newObj.(*corev1.Pod)

				logger.Info("Running QueueingHintFn function to evaluate pod",
					"prevUnschedulablePod", klog.KObj(pod),
					"eventRelatedOldPod", klog.KObj(oldPod),
					"eventRelatedNewPod", klog.KObj(newPod))

				return fwk.Queue, nil
			},
		},
		// TODO: Add cluster event for quotas if we make quotas dynamic.
		// 	If quotas are dynamic, than any changes to quotas may cause
		// 	previously-unschedulable pods to become schedulable.
	}, nil
}

func (plugin *Plugin) informerAddPod(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		logger.Info("failed to handle pod added, got unexpected object",
			"obj", obj)
	}

	if err := plugin.queueMgr.AddPodIfNotPresent(pod); err != nil {
		logger.Error(err, "Failed to add Pod to its associated quota",
			"pod", klog.KObj(pod))
	}
}

func (plugin *Plugin) informerUpdatePod(oldObj, newObj any) {
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

	if err := plugin.queueMgr.DeletePodIfPresent(newPod); err != nil {
		logger.Error(err, "Failed to delete Pod from its associated quota",
			"pod", klog.KObj(newPod))
	}
}

func (plugin *Plugin) informerDeletePod(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		logger.Info("failed to handle pod added, got unexpected object",
			"obj", obj)
	}

	if err := plugin.queueMgr.DeletePodIfPresent(pod); err != nil {
		logger.Error(err, "Failed to delete Pod from its associated quota",
			"pod", klog.KObj(pod))
	}
}
