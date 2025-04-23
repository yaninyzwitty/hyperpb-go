package sync2

import "sync"

// Pool is like sync.Pool, but strongly typed to make the interface a bit
// less messy.
type Pool[T any] struct {
	New   func() *T // Called to construct new values.
	Reset func(*T)  // Called to reset values before re-use.

	impl sync.Pool
}

// Get returns a cached value of type T, and a function that should be called
// once the use of the value is complete.
//
// Use like this:
//
//	v, drop := cache.Get()
//	defer drop()
func (p *Pool[T]) Get() (v *T, drop func()) {
	v, _ = p.impl.Get().(*T)
	if v == nil {
		switch p.New {
		case nil:
			v = new(T)
		default:
			v = p.New()
		}
	}

	return v, func() {
		if p.Reset != nil {
			p.Reset(v)
		}
		p.impl.Put(v)
	}
}
