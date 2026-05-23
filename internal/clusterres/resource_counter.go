package clusterres

import (
	"sync"

	"github.com/kaschnit/kaschnit-scheduler/internal/resmath"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type Counter struct {
	nodeIDs sets.Set[types.UID]
	total   *framework.Resource
	lock    sync.RWMutex
}

func NewCounter(nodes ...*corev1.Node) *Counter {
	counter := &Counter{
		nodeIDs: sets.New[types.UID](),
		total:   framework.NewResource(nil),
	}

	counter.PutAll(nodes)

	return counter
}

func (counter *Counter) GetTotal() *framework.Resource {
	counter.lock.RLock()
	defer counter.lock.RUnlock()

	return counter.total.Clone()
}

func (counter *Counter) Put(node *corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	counter.removeNoLock(node)
	counter.addNoLock(node)
}

func (counter *Counter) PutAll(nodes []*corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	for _, node := range nodes {
		counter.removeNoLock(node)
		counter.addNoLock(node)
	}
}

func (counter *Counter) Delete(node *corev1.Node) {
	counter.lock.Lock()
	defer counter.lock.Unlock()

	counter.removeNoLock(node)
}

func (counter *Counter) addNoLock(node *corev1.Node) {
	if node == nil || counter.nodeIDs.Has(node.UID) {
		return
	}

	counter.nodeIDs.Insert(node.UID)
	resmath.AddInPlace(counter.total, framework.NewResource(node.Status.Allocatable))
}

func (counter *Counter) removeNoLock(node *corev1.Node) {
	if node == nil {
		return
	}

	counter.nodeIDs.Delete(node.UID)
	resmath.SubtractInPlace(counter.total, framework.NewResource(node.Status.Allocatable))
}
