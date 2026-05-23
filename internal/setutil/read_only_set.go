package setutil

import "k8s.io/apimachinery/pkg/util/sets"

// ReadOnlySet is a read-only interface for [sets.Set].
type ReadOnlySet[T comparable] interface {
	Has(item T) bool
	HasAll(items ...T) bool
	HasAny(items ...T) bool
	Clone() sets.Set[T]
	Difference(other sets.Set[T]) sets.Set[T]
	SymmetricDifference(other sets.Set[T]) sets.Set[T]
	Union(other sets.Set[T]) sets.Set[T]
	Intersection(other sets.Set[T]) sets.Set[T]
	IsSuperset(other sets.Set[T]) bool
	Equal(other sets.Set[T]) bool
	UnsortedList() []T
	Len() int
}
