package cow

import (
	"iter"
	"sync"
)

// RCMap is a copy-on-write (COW) map.
// The entire underlying data is copied on write when there is more
// than 1 reference to it.
// This data structure is thread-safe.
type RCMap[K comparable, V Value[V]] struct {
	// shared is the reference-counted map.
	// This is copied on write if there's more than 1 reference.
	shared *sharedMap[K, V]
	// lock synchronizes access to shared.
	lock sync.RWMutex
}

// NewRCMap creates a new [RCMap].
func NewRCMap[K comparable, V Value[V]]() *RCMap[K, V] {
	return &RCMap[K, V]{
		shared: newSharedMap(make(map[K]*lazyClone[V])),
	}
}

// Get gets the value associated with key.
// Returns the value and true if found.
// Returns zero-value and false if not found.
func (rcm *RCMap[K, V]) Get(key K) (V, bool) {
	rcm.lock.RLock()
	defer rcm.lock.RUnlock()

	val, ok := rcm.shared.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	return val.get(), true
}

// Put associates the value with the key.
// It overwrites the existing value if the key exists.
// If the key does not exist it creates a new key/value pair.
func (rcm *RCMap[K, V]) Put(key K, val V) {
	rcm.lock.Lock()
	defer rcm.lock.Unlock()

	rcm.shared.lock.Lock()
	if rcm.shared.rc > 1 { // More than 1 reference; perform copy-on-write.
		newCapacity := len(rcm.shared.items)
		if _, exists := rcm.shared.items[key]; !exists {
			// New item being added, want 1 more capacity
			newCapacity++
		}

		clonedItems := make(map[K]*lazyClone[V], newCapacity)
		for k, v := range rcm.shared.items {
			clonedItems[k] = v.fork()
		}

		rcm.shared.rc--
		rcm.shared.lock.Unlock()

		rcm.shared = newSharedMap(clonedItems)
	} else { // Only 1 reference, no copy needed, can mutate in place.
		rcm.shared.lock.Unlock()
	}

	rcm.shared.items[key] = newDirectLazyClone(val)
}

// Delete deletes the value associated with the key from the map.
// Returns whether any value was actually removed (whether the value was present).
func (rcm *RCMap[K, V]) Delete(key K) bool {
	rcm.lock.Lock()
	defer rcm.lock.Unlock()

	rcm.shared.lock.Lock()

	_, exists := rcm.shared.items[key]
	if !exists {
		rcm.shared.lock.Unlock()
		return false
	}

	if rcm.shared.rc > 1 { // More than 1 reference; perform copy-on-write.

		clonedItems := make(map[K]*lazyClone[V], len(rcm.shared.items)-1)
		for k, v := range rcm.shared.items {
			if k != key {
				clonedItems[k] = v.fork()
			}
		}

		rcm.shared.rc--
		rcm.shared.lock.Unlock()

		rcm.shared = newSharedMap(clonedItems)
	} else { // Only 1 reference, no copy needed, can mutate in place.
		rcm.shared.lock.Unlock()
	}

	delete(rcm.shared.items, key)

	return true
}

// All returns an iterator over key-value pairs.
// See [maps.All].
func (rcm *RCMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		// Iterate a clone.
		// Clone is cheap because of copy-on-write.
		// Only create the clone inside the closure so it's not cloned unless
		// the iterator is actually used.
		cowClone := rcm.Clone()

		// Ensure we remove the reference when cowClone is unused.
		defer cowClone.Clear()

		// Safe to do without lock, because any other reference to the underlying map
		// will cause rc>1, so writes will cause a copy of the underlying map.
		for k, v := range cowClone.shared.items {
			if !yield(k, v.get()) {
				return
			}
		}
	}
}

// RefCount returns the number of references to the underlying data.
func (rcm *RCMap[K, V]) RefCount() int64 {
	rcm.lock.RLock()
	defer rcm.lock.RUnlock()

	return int64(rcm.shared.rc)
}

// Clear clears the map.
func (rcm *RCMap[K, V]) Clear() {
	rcm.lock.Lock()
	defer rcm.lock.Unlock()

	rcm.shared.lock.Lock()
	rcm.shared.rc--
	rcm.shared.lock.Unlock()

	rcm.shared = newSharedMap(make(map[K]*lazyClone[V]))
}

// Clone creates a clone of the map.
func (rcm *RCMap[K, V]) Clone() *RCMap[K, V] {
	rcm.lock.RLock()
	defer rcm.lock.RUnlock()

	rcm.shared.lock.Lock()
	rcm.shared.rc++
	rcm.shared.lock.Unlock()

	return &RCMap[K, V]{
		shared: rcm.shared,
	}
}

// ToMap returns a copy of the underlying map.
func (rcm *RCMap[K, V]) ToMap() map[K]V {
	rcm.lock.RLock()
	defer rcm.lock.RUnlock()

	result := make(map[K]V, len(rcm.shared.items))
	for k, v := range rcm.shared.items {
		result[k] = v.get()
	}

	return result
}
