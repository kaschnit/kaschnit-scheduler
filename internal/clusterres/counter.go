package clusterres

import (
	"sync"

	"github.com/kaschnit/kaschnit-scheduler/internal/alloc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Counter counts resources belonging to nodes.
type Counter struct {
	nodeIDs    sets.Set[types.UID]
	total      alloc.Resources
	lock       sync.RWMutex
	getNodeRes func(*corev1.Node) corev1.ResourceList
}

// NewAllocatableCounter creates a new [Counter] that counts
// allocatable resources of nodes.
func NewAllocatableCounter(nodes ...*corev1.Node) *Counter {
	counter := &Counter{
		nodeIDs: sets.New[types.UID](),
		total:   make(alloc.Resources),
		getNodeRes: func(node *corev1.Node) corev1.ResourceList {
			return node.Status.Allocatable
		},
	}

	counter.PutAll(nodes)

	return counter
}

// GetTotal gets the total count of resources counted.
func (counter *Counter) GetTotal() alloc.Resources {
	counter.lock.RLock()
	defer counter.lock.RUnlock()

	return counter.total.Clone()
}

// Put adds or updates the node in the counter.
func (counter *Counter) Put(node *corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	counter.deleteNoLock(node)
	counter.addNoLock(node)
}

// PutAll adds or updates each node in the counter.
func (counter *Counter) PutAll(nodes []*corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	for _, node := range nodes {
		counter.deleteNoLock(node)
		counter.addNoLock(node)
	}
}

// Delete removes a node from the counter.
func (counter *Counter) Delete(node *corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	counter.deleteNoLock(node)
}

func (counter *Counter) addNoLock(node *corev1.Node) {
	if node == nil || counter.nodeIDs.Has(node.UID) {
		return
	}

	counter.nodeIDs.Insert(node.UID)
	counter.total.Add(alloc.FromResourceList(counter.getNodeRes(node)))
}

func (counter *Counter) deleteNoLock(node *corev1.Node) {
	if node == nil || !counter.nodeIDs.Has(node.UID) {
		return
	}

	counter.nodeIDs.Delete(node.UID)
	counter.total.Sub(alloc.FromResourceList(counter.getNodeRes(node)))
}
