package cow

import "sync"

type lazyClone[V Value[V]] struct {
	value  V
	cloned bool
	lock   sync.Mutex
}

func newLazyClone[V Value[V]](value V) *lazyClone[V] {
	return &lazyClone[V]{
		value:  value,
		cloned: false,
	}
}

func newDirectLazyClone[V Value[V]](value V) *lazyClone[V] {
	return &lazyClone[V]{
		value:  value,
		cloned: true,
	}
}

func (lc *lazyClone[V]) get() V {
	lc.lock.Lock()
	defer lc.lock.Unlock()

	if !lc.cloned {
		lc.value = lc.value.Clone()
		lc.cloned = true
	}

	return lc.value
}

func (lc *lazyClone[V]) fork() *lazyClone[V] {
	lc.lock.Lock()
	defer lc.lock.Unlock()

	return newLazyClone(lc.value)
}
