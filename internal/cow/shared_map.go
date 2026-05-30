package cow

// sharedMap is a reference-counted map.
// TODO: extract reference-counted operations from Map into sharedMap, similar to sharedValue.
type sharedMap[K comparable, V Value[V]] struct {
	// items is the underlying map.
	items map[K]*sharedValue[V]
	// rc is the reference count.
	*refCounter
}

// newSharedMap creates a new [sharedMap].
func newSharedMap[K comparable, V Value[V]](items map[K]*sharedValue[V]) *sharedMap[K, V] {
	return &sharedMap[K, V]{
		items:      items,
		refCounter: newRefCounter(),
	}
}
