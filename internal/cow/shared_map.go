package cow

import "sync"

// sharedMap is a reference-counted map.
type sharedMap[K comparable, V Value[V]] struct {
	// items is the underlying map.
	items map[K]*lazyClone[V]
	// rc is the reference count.
	rc int64
	// lock synchronizes access to rc.
	// It does not need to synchronize access to items, since items should
	// be read-only if there is more than 1 reference to it; and creation
	// of new references are externally synchronized (by [Map.lock]).
	lock sync.Mutex
}

// newSharedMap creates a new [sharedMap].
func newSharedMap[K comparable, V Value[V]](items map[K]*lazyClone[V]) *sharedMap[K, V] {
	return &sharedMap[K, V]{
		items: items,
		rc:    1,
	}
}
