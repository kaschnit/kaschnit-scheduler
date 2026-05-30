package cow

import (
	"iter"
	"sync"
)

// Map is a copy-on-write (COW) map that uses reference counting to determine when writes are needed.
//
// The entire underlying data is shallow-copied on write when there is more than one reference to it.
// If there is only 1 reference, it is not copied on write.
// Each underlying value is cloned only when needed.
//
// Map provides an interface that looks like a deep-copyable data structure,
// but only data that needs to be deep-copied is actually deep-copied when it needs to be.
//
// Map can be used with non-pointer values, but the benefits of lazy cloning are more
// apparent when working with pointers. Non-pointer values may have worse performance than an eager
// deep copy would, since they're first shallow-copied on write, and again cloned on first shared read.
//
// All receiver methods of Map are thread-safe.
type Map[K comparable, V Value[V]] struct {
	// shared is the shared counted map.
	// This is copied on write if there's more than 1 reference.
	shared *sharedMap[K, V]
	// lock synchronizes access to shared.
	lock sync.RWMutex
}

// NewMap creates a new [Map].
func NewMap[K comparable, V Value[V]]() *Map[K, V] {
	return &Map[K, V]{
		shared: newSharedMap(make(map[K]*sharedValue[V])),
	}
}

// Get gets the value associated with key.
// Returns the value and true if found.
// Returns zero-value and false if not found.
func (rcm *Map[K, V]) Get(key K) (V, bool) {
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
func (rcm *Map[K, V]) Put(key K, val V) {
	rcm.lock.Lock()
	defer rcm.lock.Unlock()

	rcm.shared.lock.Lock()
	if rcm.shared.count > 1 { // More than 1 reference; perform copy-on-write.
		newCapacity := len(rcm.shared.items)
		if _, exists := rcm.shared.items[key]; !exists {
			// New item being added, want 1 more capacity
			newCapacity++
		}

		clonedItems := make(map[K]*sharedValue[V], newCapacity)
		for k, v := range rcm.shared.items {
			clonedItems[k] = v.copy()
		}

		rcm.shared.count--
		rcm.shared.lock.Unlock()

		rcm.shared = newSharedMap(clonedItems)
	} else { // Only 1 reference, no copy needed, can mutate in place.
		rcm.shared.lock.Unlock()
	}

	rcm.shared.items[key] = newSharedValue(val)
}

// Delete deletes the value associated with the key from the map.
// Returns whether any value was actually removed (whether the value was present).
func (rcm *Map[K, V]) Delete(key K) bool {
	rcm.lock.Lock()
	defer rcm.lock.Unlock()

	rcm.shared.lock.Lock()

	_, exists := rcm.shared.items[key]
	if !exists {
		rcm.shared.lock.Unlock()
		return false
	}

	if rcm.shared.count > 1 { // More than 1 reference; perform copy-on-write.
		clonedItems := make(map[K]*sharedValue[V], len(rcm.shared.items)-1)
		for k, v := range rcm.shared.items {
			if k != key {
				clonedItems[k] = v.copy()
			}
		}

		rcm.shared.count--
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
func (rcm *Map[K, V]) All() iter.Seq2[K, V] {
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

// All returns an iterator over the values.
// See [maps.Values].
func (rcm *Map[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		// Iterate a clone.
		// Clone is cheap because of copy-on-write.
		// Only create the clone inside the closure so it's not cloned unless
		// the iterator is actually used.
		cowClone := rcm.Clone()

		// Ensure we remove the reference when cowClone is unused.
		defer cowClone.Clear()

		// Safe to do without lock, because any other reference to the underlying map
		// will cause rc>1, so writes will cause a copy of the underlying map.
		for _, v := range cowClone.shared.items {
			if !yield(v.get()) {
				return
			}
		}
	}
}

// RefCount returns the number of references to the underlying data.
func (rcm *Map[K, V]) RefCount() int64 {
	rcm.lock.RLock()
	defer rcm.lock.RUnlock()

	return int64(rcm.shared.count)
}

// Clear clears the map.
func (rcm *Map[K, V]) Clear() {
	rcm.lock.Lock()
	defer rcm.lock.Unlock()

	rcm.shared.lock.Lock()
	rcm.shared.count--
	rcm.shared.lock.Unlock()

	rcm.shared = newSharedMap(make(map[K]*sharedValue[V]))
}

// Clone creates a clone of the map.
func (rcm *Map[K, V]) Clone() *Map[K, V] {
	rcm.lock.RLock()
	defer rcm.lock.RUnlock()

	rcm.shared.lock.Lock()
	rcm.shared.count++
	rcm.shared.lock.Unlock()

	return &Map[K, V]{
		shared: rcm.shared,
	}
}

// ToMap returns a copy of the underlying map.
func (rcm *Map[K, V]) ToMap() map[K]V {
	rcm.lock.RLock()
	defer rcm.lock.RUnlock()

	result := make(map[K]V, len(rcm.shared.items))
	for k, v := range rcm.shared.items {
		result[k] = v.get()
	}

	return result
}
