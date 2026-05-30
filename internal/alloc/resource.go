package alloc

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/sets"
	reshelper "k8s.io/component-helpers/resource"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// ResourceName is the name of a resource.
type ResourceName string

// Common [ResourceName]s.
const (
	ResourceNameCPU    ResourceName = "cpu"
	ResourceNameMemory ResourceName = "memory"
	ResourceNamePods   ResourceName = "pods"
)

// Resources is a group of named resources.
type Resources map[ResourceName]resource.Quantity

// FromResourceList converts rl to a [Resources].
func FromResourceList(rl corev1.ResourceList) Resources {
	r := make(Resources, len(rl))
	for name, qty := range rl {
		r[ResourceName(name)] = qty.DeepCopy()
	}
	return r
}

// FromPodReq converts pod's requests to a [Resources].
func FromPodReq(pod *corev1.Pod) Resources {
	if pod == nil {
		return make(Resources)
	}

	rl := reshelper.PodRequests(pod, reshelper.PodResourcesOptions{})
	rl[corev1.ResourcePods] = *resource.NewQuantity(1, resource.BinarySI)

	return FromResourceList(rl)
}

// FromNodeAllocatable converts node's allocatable resource to a [Resources].
func FromNodeAllocatable(node *corev1.Node) Resources {
	if node == nil {
		return make(Resources)
	}

	return FromResourceList(node.Status.Allocatable)
}

// Add adds the values of other resources to this one.
// This mutates r.
func (r Resources) Add(other Resources) {
	if other == nil {
		return
	}

	for name, otherQty := range other {
		if currentQty, exists := r[name]; exists {
			currentQty.Add(otherQty)
			r[name] = currentQty
		} else {
			r[name] = otherQty.DeepCopy()
		}
	}
}

// Sub subtracts the values of other resources from this one.
// This mutates r.
func (r Resources) Sub(other Resources) {
	if other == nil {
		return
	}

	for name, otherQty := range other {
		if currentQty, exists := r[name]; exists {
			currentQty.Sub(otherQty)
			r[name] = currentQty
		} else {
			r[name] = otherQty.DeepCopy()
		}
	}
}

// Negate flips the sign of every resource in r.
// This mutates r.
func (r Resources) Negate() {
	for k, q := range r {
		negQty := resource.Quantity{}
		negQty.Sub(q)
		r[k] = negQty
	}
}

// SetIfNotPresent sets the quantity for the given resource if it's not present.
func (r Resources) SetIfNotPresent(name ResourceName, qty resource.Quantity) {
	if _, ok := r[name]; !ok {
		r[name] = qty
	}
}

// SetMinExisting sets the resources on r to the smaller of that resource
// between r and other. It only affects resources that exist in r.
// This mutates r.
func (r Resources) SetMinExisting(other Resources) {
	for name, maxQty := range other {
		if maxQty.Cmp(r[name]) < 0 {
			r[name] = maxQty.DeepCopy()
		}
	}
}

// TakeMinExisting takes the minimum of resources between r and other.
// If a resource defined in r is not defined in other, it is left as is.
// This does not mutate r.
func (r Resources) TakeMinExisting(other Resources) Resources {
	newRes := r.Clone()
	newRes.SetMinExisting(other)
	return newRes
}

// Plus returns these resources added to the other.
// This does not mutate r.
func (r Resources) Plus(other Resources) Resources {
	if r == nil {
		return other.Clone()
	}

	newRes := r.Clone()
	newRes.Add(other)
	return newRes
}

// Minus returns these resources minus the other.
// This does not mutate r.
func (r Resources) Minus(other Resources) Resources {
	if r == nil {
		return other.Clone()
	}

	newRes := r.Clone()
	newRes.Sub(other)
	return newRes
}

// Negated returns a copy of r with each resource's sign flipped.
// This does not mutate r.
func (r Resources) Negated() Resources {
	newRes := r.Clone()
	newRes.Negate()
	return newRes
}

// AllGreater checks if all resources of r are greater than other.
// Nonexistent quantities are treated as 0.
func (r Resources) AllGreater(other Resources) bool {
	if r == nil {
		r = make(Resources)
	}
	if other == nil {
		other = make(Resources)
	}

	for name := range getResNames(r, other) {
		qty1 := r[name]
		qty2 := other[name]
		if qty1.Cmp(qty2) < 0 {
			return false
		}
	}

	return true
}

// AllGreaterIntersecting checks if all resources of r are greater than other for
// only resources that are present in both r and other.
func (r Resources) AllGreaterIntersecting(other Resources) bool {
	if r == nil {
		r = make(Resources)
	}
	if other == nil {
		other = make(Resources)
	}

	for name, qty := range r {
		if otherQty, exists := other[name]; exists {
			if qty.Cmp(otherQty) < 0 {
				return false
			}
		}
	}

	return true
}

// AnyGreaterIntersecting checks if any resources of r are greater than other for
// only resources that are present in both r and other.
func (r Resources) AnyGreaterIntersecting(other Resources) bool {
	if r == nil {
		r = make(Resources)
	}
	if other == nil {
		other = make(Resources)
	}

	for name, qty := range r {
		if otherQty, exists := other[name]; exists {
			if qty.Cmp(otherQty) > 0 {
				return true
			}
		}
	}

	return false
}

// AllLess checks if all resources of r are less than other.
// Nonexistent resources are treated as 0.
func (r Resources) AllLess(other Resources) bool {
	if r == nil {
		r = make(Resources)
	}
	if other == nil {
		other = make(Resources)
	}

	for name := range getResNames(r, other) {
		qty1 := r[name]
		qty2 := other[name]
		if qty1.Cmp(qty2) < 0 {
			return false
		}
	}

	return true
}

// AllLessIntersecting checks if all resources of r are less than other for
// only resources that are present in both r and other.
func (r Resources) AllLessIntersecting(other Resources) bool {
	if r == nil {
		r = make(Resources)
	}
	if other == nil {
		other = make(Resources)
	}

	for name, qty := range r {
		if otherQty, exists := other[name]; exists {
			if qty.Cmp(otherQty) > 0 {
				return false
			}
		}
	}

	return true
}

// AnyLessIntersecting checks if any resources of r are less than other for
// only resources that are present in both r and other.
func (r Resources) AnyLessIntersecting(other Resources) bool {
	if r == nil {
		r = make(Resources)
	}
	if other == nil {
		other = make(Resources)
	}

	for name, qty := range r {
		if otherQty, exists := other[name]; exists {
			if qty.Cmp(otherQty) < 0 {
				return true
			}
		}
	}

	return false
}

// Equal checks if r is equal to other.
// This is false if either resource contains a key that the other one doesn't.
func (r Resources) Equal(other Resources) bool {
	if r == nil {
		r = make(Resources)
	}
	if other == nil {
		other = make(Resources)
	}

	for name := range getResNames(r, other) {
		qty1, ok := r[name]
		if !ok {
			return false
		}

		qty2, ok := other[name]
		if !ok {
			return false
		}

		if !qty1.Equal(qty2) {
			return false
		}
	}

	return true
}

// EqualIntersecting checks if r is equal to other for only resources
// that are present in both r and other.
func (r Resources) EqualIntersecting(other Resources) bool {
	if other == nil {
		other = make(Resources)
	}

	for name, qty := range r {
		if otherQty, exists := other[name]; exists {
			if !qty.Equal(otherQty) {
				return false
			}
		}
	}

	return true
}

// Equivalent checks if r is equivalent to other for only resources.
// This treats nonexisting quantities the same as zero quantity.
func (r Resources) Equivalent(other Resources) bool {
	if r == nil {
		r = make(Resources)
	}
	if other == nil {
		other = make(Resources)
	}

	for name := range getResNames(r, other) {
		qty1 := r[name]
		qty2 := other[name]
		if !qty1.Equal(qty2) {
			return false
		}
	}

	return true
}

// ToResourceList converts r to [corev1.ResourceList].
func (r Resources) ToResourceList() corev1.ResourceList {
	if r == nil {
		return nil
	}

	rl := make(corev1.ResourceList, len(r))
	for name, qty := range r {
		rl[corev1.ResourceName(name)] = qty.DeepCopy()
	}
	return rl
}

// ToFrameworkResource converts r to [framework.Resource].
func (r Resources) ToFrameworkResource() *framework.Resource {
	if r == nil {
		return nil
	}

	return framework.NewResource(r.ToResourceList())
}

// Clone creates a deep copy of r.
func (r Resources) Clone() Resources {
	if r == nil {
		return nil
	}

	cloned := make(Resources, len(r))
	for name, qty := range r {
		cloned[name] = qty.DeepCopy()
	}
	return cloned
}

// String converts r to a string representation.
func (r Resources) String() string {
	names := make([]ResourceName, 0, len(r))
	for name := range r {
		names = append(names, name)
	}
	slices.SortFunc(names, func(name1, name2 ResourceName) int {
		return cmp.Compare(name1, name2)
	})

	var sb strings.Builder
	sb.WriteString("{")
	for i, name := range names {
		if i > 0 {
			sb.WriteString(", ")
		}
		qty := r[name]
		fmt.Fprintf(&sb, "%s: %s", name, &qty)
	}
	sb.WriteString("}")

	return sb.String()
}

func getResNames(resources ...Resources) sets.Set[ResourceName] {
	names := make(sets.Set[ResourceName])
	for _, res := range resources {
		if res == nil {
			continue
		}

		for name := range res {
			names.Insert(name)
		}
	}

	return names
}
