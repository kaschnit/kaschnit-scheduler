package cow

import "sync"

type sharedValue[V Value[V]] struct {
	value V
	rc    *refCounter
	lock  sync.Mutex
}

func newSharedValue[V Value[V]](value V) *sharedValue[V] {
	return &sharedValue[V]{
		value: value,
		rc:    newRefCounter(),
	}
}

func (lc *sharedValue[V]) get() V {
	lc.lock.Lock()
	defer lc.lock.Unlock()

	lc.rc.lock.Lock()
	if lc.rc.count > 1 {
		lc.rc.count--
		lc.rc.lock.Unlock()

		lc.value = lc.value.Clone()
		lc.rc = newRefCounter()
	} else {
		lc.rc.lock.Unlock()
	}

	return lc.value
}

func (lc *sharedValue[V]) copy() *sharedValue[V] {
	lc.lock.Lock()
	defer lc.lock.Unlock()

	lc.rc.lock.Lock()
	lc.rc.count++
	lc.rc.lock.Unlock()

	return &sharedValue[V]{
		value: lc.value,
		rc:    lc.rc,
	}
}
