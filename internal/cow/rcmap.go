package cow

import (
	"iter"
	"sync"
)

// sharedMap is a reference-counted map.
type sharedMap[K comparable, V any] struct {
	// items is the underlying map.
	items map[K]V
	// rc is the reference count.
	rc int64
	// lock synchronizes access to rc.
	// It does not need to synchronize access to items, since items should
	// be read-only if there is more than 1 reference to it; and creation
	// of new references are externally synchronized (by [Map.lock]).
	lock sync.Mutex
}

// newSharedMap creates a new [sharedMap].
func newSharedMap[K comparable, V any](items map[K]V) *sharedMap[K, V] {
	return &sharedMap[K, V]{
		items: items,
		rc:    1,
	}
}

// RCMap is a copy-on-write (COW) map.
// The entire underlying data is copied on write when there is more
// than 1 reference to it.
// This data structure is thread-safe.
type RCMap[K comparable, V any] struct {
	// shared is the reference-counted map.
	// This is copied on write if there's more than 1 reference.
	shared     *sharedMap[K, V]
	cloneValue func(V) V
	// lock synchronizes access to shared.
	lock sync.RWMutex
}

// RCMapOpt are options for [RCMap].
type RCMapOpt[K comparable, V any] func(*RCMap[K, V])

// WithCloneValue defines how to clone a value in the map when performing copy-on-write.
func WithCloneValue[K comparable, V any](cloneValue func(V) V) RCMapOpt[K, V] {
	return func(r *RCMap[K, V]) {
		if cloneValue == nil {
			cloneValue = func(v V) V { return v }
		}

		r.cloneValue = cloneValue
	}
}

// NewRCMap creates a new [RCMap].
func NewRCMap[K comparable, V any](opts ...RCMapOpt[K, V]) *RCMap[K, V] {
	defaultOpts := []RCMapOpt[K, V]{
		WithCloneValue[K, V](nil),
	}

	rcm := &RCMap[K, V]{
		shared: newSharedMap(make(map[K]V)),
	}
	for _, opt := range defaultOpts {
		opt(rcm)
	}

	for _, opt := range opts {
		opt(rcm)
	}

	return rcm
}

// Get gets the value associated with key.
// Returns the value and true if found.
// Returns zero-value and false if not found.
func (rcm *RCMap[K, V]) Get(key K) (V, bool) {
	rcm.lock.RLock()
	defer rcm.lock.RUnlock()

	val, ok := rcm.shared.items[key]
	return val, ok
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

		clonedItems := make(map[K]V, newCapacity)
		for k, v := range rcm.shared.items {
			clonedItems[k] = rcm.cloneValue(v)
		}

		rcm.shared.rc--
		rcm.shared.lock.Unlock()

		rcm.shared = newSharedMap(clonedItems)
	} else { // Only 1 reference, no copy needed, can mutate in place.
		rcm.shared.lock.Unlock()
	}

	rcm.shared.items[key] = val
}

// Delete deletes the value associated with the key from the map.
// Returns whether any value was actually removed (whether the value was present).
func (rcm *RCMap[K, V]) Delete(key K) bool {
	rcm.lock.Lock()
	defer rcm.lock.Unlock()

	rcm.shared.lock.Lock()
	if rcm.shared.rc > 1 { // More than 1 reference; perform copy-on-write.
		if _, exists := rcm.shared.items[key]; !exists {
			// Avoid copy on no-op delete.
			rcm.shared.lock.Unlock()
			return false
		}

		clonedItems := make(map[K]V, len(rcm.shared.items)-1)
		for k, v := range rcm.shared.items {
			if k != key {
				clonedItems[k] = rcm.cloneValue(v)
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
	// Iterate a clone.
	// Clone is cheap because of copy-on-write.
	cowClone := rcm.Clone()

	return func(yield func(K, V) bool) {
		// Ensure we remove the reference when cowClone is unused.
		defer cowClone.Clear()

		// Safe to do without lock, because any other reference to the underlying map
		// will cause rc>1, so writes will cause a copy of the underlying map.
		for k, v := range cowClone.shared.items {
			if !yield(k, v) {
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

	rcm.shared = newSharedMap(make(map[K]V))
}

// Clone creates a clone of the map.
func (rcm *RCMap[K, V]) Clone() *RCMap[K, V] {
	rcm.lock.RLock()
	defer rcm.lock.RUnlock()

	rcm.shared.lock.Lock()
	rcm.shared.rc++
	rcm.shared.lock.Unlock()

	return &RCMap[K, V]{
		shared:     rcm.shared,
		cloneValue: rcm.cloneValue,
	}
}

// ToMap returns a copy of the underlying map.
func (rcm *RCMap[K, V]) ToMap() map[K]V {
	rcm.lock.RLock()
	defer rcm.lock.RUnlock()

	result := make(map[K]V, len(rcm.shared.items))
	for key, val := range rcm.shared.items {
		result[key] = rcm.cloneValue(val)
	}

	return result
}
