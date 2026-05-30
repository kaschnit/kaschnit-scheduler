package cow

import (
	"iter"
	"maps"
	"sync"
)

// rcMap is a reference-counted map.
type rcMap[K comparable, V any] struct {
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

// newRCMap creates a new [rcMap].
func newRCMap[K comparable, V any](items map[K]V) *rcMap[K, V] {
	return &rcMap[K, V]{
		items: items,
		rc:    1,
	}
}

// Map is a copy-on-write (COW) map.
// This data structure is thread-safe.
type Map[K comparable, V any] struct {
	// rcMap is the reference-counted map.
	// This is copied on write if there's more than 1 reference.
	rcMap *rcMap[K, V]
	// lock synchronizes access to rcMap.
	lock sync.RWMutex
}

// NewMap creates a new [Map].
func NewMap[K comparable, V any]() *Map[K, V] {
	return &Map[K, V]{
		rcMap: newRCMap(make(map[K]V)),
	}
}

// NewMapFromItems creates a new [Map] with the given items.
func NewMapFromItems[K comparable, V any](items map[K]V) *Map[K, V] {
	if items == nil {
		// Never want underlying map to be nil
		return NewMap[K, V]()
	}

	return &Map[K, V]{
		rcMap: newRCMap(maps.Clone(items)),
	}
}

// Get gets the value associated with key.
// Returns the value and true if found.
// Returns zero-value and false if not found.
func (cm *Map[K, V]) Get(key K) (V, bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()

	val, ok := cm.rcMap.items[key]
	return val, ok
}

// Set associates the value with the key.
// It overwrites the existing value if the key exists.
// If the key does not exist it creates a new key/value pair.
func (cm *Map[K, V]) Set(key K, val V) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	cm.rcMap.lock.Lock()
	if cm.rcMap.rc > 1 { // More than 1 reference; perform copy-on-write.
		newCapacity := len(cm.rcMap.items)
		if _, exists := cm.rcMap.items[key]; !exists {
			// New item being added, want 1 more capacity
			newCapacity++
		}

		clonedItems := make(map[K]V, newCapacity)
		for k, v := range cm.rcMap.items {
			clonedItems[k] = v
		}

		cm.rcMap.rc--
		cm.rcMap.lock.Unlock()

		cm.rcMap = newRCMap(clonedItems)
	} else { // Only 1 reference, no copy needed, can mutate in place.
		cm.rcMap.lock.Unlock()
	}

	cm.rcMap.items[key] = val
}

// Delete deletes the value associated with the key from the map.
// Returns whether any value was actually removed (whether the value was present).
func (cm *Map[K, V]) Delete(key K) bool {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	cm.rcMap.lock.Lock()
	if cm.rcMap.rc > 1 { // More than 1 reference; perform copy-on-write.
		if _, exists := cm.rcMap.items[key]; !exists {
			// Avoid copy on no-op delete.
			cm.rcMap.lock.Unlock()
			return false
		}

		clonedItems := make(map[K]V, len(cm.rcMap.items)-1)
		for k, v := range cm.rcMap.items {
			if k != key {
				clonedItems[k] = v
			}
		}

		cm.rcMap.rc--
		cm.rcMap.lock.Unlock()

		cm.rcMap = newRCMap(clonedItems)
	} else { // Only 1 reference, no copy needed, can mutate in place.
		cm.rcMap.lock.Unlock()
	}

	delete(cm.rcMap.items, key)

	return true
}

// All returns an iterator over key-value pairs.
// See [maps.All].
func (cm *Map[K, V]) All() iter.Seq2[K, V] {
	// Iterate a clone.
	// Clone is cheap because of copy-on-write.
	cowClone := cm.Clone()

	return func(yield func(K, V) bool) {
		// Ensure we remove the reference when cowClone is unused.
		defer cowClone.Clear()

		// Safe to do without lock, because any other reference to the underlying map
		// will cause rc>1, so writes will cause a copy of the underlying map.
		for k, v := range cowClone.rcMap.items {
			if !yield(k, v) {
				return
			}
		}
	}
}

// Clear clears the map.
func (cm *Map[K, V]) Clear() {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	cm.rcMap.lock.Lock()
	cm.rcMap.rc--
	cm.rcMap.lock.Unlock()

	cm.rcMap = newRCMap(make(map[K]V))
}

// Clone creates a clone of the map.
func (cm *Map[K, V]) Clone() *Map[K, V] {
	cm.lock.RLock()
	defer cm.lock.RUnlock()

	cm.rcMap.lock.Lock()
	cm.rcMap.rc++
	cm.rcMap.lock.Unlock()

	return &Map[K, V]{
		rcMap: cm.rcMap,
	}
}
