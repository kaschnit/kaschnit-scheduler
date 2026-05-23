package cluster

import (
	"sync"

	"github.com/kaschnit/kaschnit-scheduler/internal/resmath"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type ResourceCounter struct {
	nodeIDs sets.Set[types.UID]
	total   *framework.Resource
	lock    sync.Mutex
}

func NewResourceCounter(nodes ...*corev1.Node) *ResourceCounter {
	counter := &ResourceCounter{
		nodeIDs: sets.New[types.UID](),
		total:   framework.NewResource(nil),
	}

	counter.PutAll(nodes)

	return counter
}

func (counter *ResourceCounter) TotalResources() *framework.Resource {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	return counter.total.Clone()
}

func (counter *ResourceCounter) Put(node *corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	counter.removeNoLock(node)
	counter.addNoLock(node)
}

func (counter *ResourceCounter) PutAll(nodes []*corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	for _, node := range nodes {
		counter.removeNoLock(node)
		counter.addNoLock(node)
	}
}

func (counter *ResourceCounter) Delete(node *corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	counter.removeNoLock(node)
}

func (counter *ResourceCounter) addNoLock(node *corev1.Node) {
	if node == nil || counter.nodeIDs.Has(node.UID) {
		return
	}

	counter.nodeIDs.Insert(node.UID)
	resmath.AddInPlace(counter.total, framework.NewResource(node.Status.Allocatable))
}

func (counter *ResourceCounter) removeNoLock(node *corev1.Node) {
	if node == nil {
		return
	}

	counter.nodeIDs.Delete(node.UID)
	resmath.SubtractInPlace(counter.total, framework.NewResource(node.Status.Allocatable))
}
