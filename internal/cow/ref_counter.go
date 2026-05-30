package cow

import "sync"

type refCounter struct {
	// count is the reference count.
	count int64
	// lock synchronizes access to rc.
	lock sync.Mutex
}

func newRefCounter() *refCounter {
	return &refCounter{
		count: 1,
	}
}
