package cluster

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type ResourceTracker struct {
	counter *ResourceCounter
}

func NewResourceTracker(
	ctx context.Context,
	informerFactory informers.SharedInformerFactory,
) (*ResourceTracker, error) {
	counter := ResourceTracker{
		counter: NewResourceCounter(),
	}

	nodeInformer := informerFactory.Core().V1().Nodes().Informer()
	if _, err := nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    counter.addNode,
		UpdateFunc: counter.updateNode,
		DeleteFunc: counter.deleteNode,
	}); err != nil {
		return nil, err
	}

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	return &counter, nil
}

func (tracker *ResourceTracker) GetTotal() *framework.Resource {
	return tracker.counter.TotalResources()
}

func (tracker *ResourceTracker) addNode(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	node, ok := obj.(*corev1.Node)
	if !ok {
		logger.Info("failed to handle node added, got unexpected object",
			"obj", obj)
	}

	tracker.counter.Put(node)
}

func (tracker *ResourceTracker) updateNode(oldObj, newObj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	oldNode, ok := oldObj.(*corev1.Node)
	if !ok {
		logger.Info("failed to handle node updated, got unexpected old object",
			"oldObj", oldObj)
	}

	newNode, ok := newObj.(*corev1.Node)
	if !ok {
		logger.Info("failed to handle node updated, got unexpected new object",
			"newObj", newObj)
	}

	needsUpdate := false
	if len(oldNode.Status.Allocatable) == len(newNode.Status.Allocatable) {
		// Gather all resource names.
		allResNames := make([]corev1.ResourceName, 0,
			len(oldNode.Status.Allocatable)+len(newNode.Status.Allocatable))
		for resName := range oldNode.Status.Allocatable {
			allResNames = append(allResNames, resName)
		}
		for resName := range newNode.Status.Allocatable {
			allResNames = append(allResNames, resName)
		}

		// Majority of the time we can avoid updating the node counter.
		// Check if any resource allocatable has changed.
		for _, resName := range allResNames {
			oldQuant, ok := oldNode.Status.Allocatable[resName]
			if !ok {
				needsUpdate = true
				break
			}

			newQuant, ok := oldNode.Status.Allocatable[resName]
			if !ok {
				needsUpdate = true
				break
			}

			if !newQuant.Equal(oldQuant) {
				needsUpdate = true
				break
			}
		}
	} else {
		needsUpdate = true
	}

	if needsUpdate {
		tracker.counter.Put(newNode)
	}
}

func (tracker *ResourceTracker) deleteNode(obj any) {
	ctx := context.Background()
	logger := klog.FromContext(ctx)

	node, ok := obj.(*corev1.Node)
	if !ok {
		logger.Info("failed to handle node deleted, got unexpected object",
			"obj", obj)
	}

	tracker.counter.Delete(node)
}
