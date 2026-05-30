package cow

// Value is a value allowed in copy-on-write data structures.
type Value[V any] interface {
	// Clone clones this value.
	Clone() V
}
