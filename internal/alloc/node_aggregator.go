package alloc

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// NodeAggregator aggregates resources across nodes.
type NodeAggregator struct {
	nodeIDs    sets.Set[types.UID]
	total      Resources
	lock       sync.RWMutex
	getNodeRes func(*corev1.Node) corev1.ResourceList
}

// NewNodeAllocatableAggregator creates a new [NodeAggregator] that aggregates
// allocatable resources of nodes.
func NewNodeAllocatableAggregator(nodes ...*corev1.Node) *NodeAggregator {
	counter := &NodeAggregator{
		nodeIDs: sets.New[types.UID](),
		total:   make(Resources),
		getNodeRes: func(node *corev1.Node) corev1.ResourceList {
			return node.Status.Allocatable
		},
	}

	counter.PutAll(nodes)

	return counter
}

// GetTotal gets the total count of resources counted.
func (counter *NodeAggregator) GetTotal() Resources {
	counter.lock.RLock()
	defer counter.lock.RUnlock()

	return counter.total.Clone()
}

// Put adds or updates the node in the counter.
func (counter *NodeAggregator) Put(node *corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	counter.deleteNoLock(node)
	counter.addNoLock(node)
}

// PutAll adds or updates each node in the counter.
func (counter *NodeAggregator) PutAll(nodes []*corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	for _, node := range nodes {
		counter.deleteNoLock(node)
		counter.addNoLock(node)
	}
}

// Delete removes a node from the counter.
func (counter *NodeAggregator) Delete(node *corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	counter.deleteNoLock(node)
}

func (counter *NodeAggregator) addNoLock(node *corev1.Node) {
	if node == nil || counter.nodeIDs.Has(node.UID) {
		return
	}

	counter.nodeIDs.Insert(node.UID)
	counter.total.Add(FromResourceList(counter.getNodeRes(node)))
}

func (counter *NodeAggregator) deleteNoLock(node *corev1.Node) {
	if node == nil || !counter.nodeIDs.Has(node.UID) {
		return
	}

	counter.nodeIDs.Delete(node.UID)
	counter.total.Sub(FromResourceList(counter.getNodeRes(node)))
}
