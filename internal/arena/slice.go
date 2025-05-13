// Copyright 2020-2025 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package arena

import (
	"fmt"
	"unsafe"

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Slice is a slice that points into an arena.
//
// Unlike an ordinary slice, it does not contain pointers; in order to work
// correctly, it must be kept alive no longer than its owning arena.
type Slice[T any] struct {
	ptr      *T
	len, cap uint32
}

// SliceAddr is like [Slice], but its pointer is replaced with an address, so
// loading/storing values of this type issues no write barriers.
type SliceAddr[T any] struct {
	ptr      unsafe2.Addr[T]
	len, cap uint32
}

const (
	SliceSize  = int(unsafe.Sizeof(Slice[byte]{}))
	SliceAlign = int(unsafe.Alignof(Slice[byte]{}))
)

// SliceFromParts assembles a slice from its raw components.
func SliceFromParts[T any](ptr *T, len, cap uint32) Slice[T] {
	return Slice[T]{ptr, len, cap}
}

// Addr converts this slice into an address slice.
//
// See the caveats of [unsafe2.AddrOf].
func (s Slice[T]) Addr() SliceAddr[T] {
	return SliceAddr[T]{unsafe2.AddrOf(s.ptr), s.len, s.cap}
}

// Addr converts this address slice into a true [Slice].
//
// See the caveats of [unsafe2.Addr.AssertValid].
func (s SliceAddr[T]) AssertValid() Slice[T] {
	return Slice[T]{s.ptr.AssertValid(), s.len, s.cap}
}

// SliceOf allocates a slice for the given values.
func SliceOf[T any](a *Arena, values ...T) Slice[T] {
	s := NewSlice[T](a, len(values))
	copy(s.Raw(), values)
	return s
}

// NewSlice allocates a slice of the given length.
func NewSlice[T any](a *Arena, n int) Slice[T] {
	cap := sliceLayout[T](n)
	p := unsafe2.Cast[T](a.Alloc(cap))

	size, _ := unsafe2.Layout[T]()
	s := SliceFromParts(p, uint32(n), uint32(cap/size))
	return s
}

// Ptr returns this slice's pointer value.
func (s Slice[T]) Ptr() *T {
	return unsafe2.Cast[T](s.ptr)
}

// Len returns this slice's length.
func (s Slice[_]) Len() int {
	return int(s.len)
}

// SetLet directly sets the length of s.
func (s Slice[T]) SetLen(n int) Slice[T] {
	if dbg.Enabled && n > int(s.cap) {
		panic(fmt.Errorf("runtime error: SetLen(%v) with Cap() = %v", n, s.cap))
	}

	dbg.Log(nil, "set len", "%v->%d", s.Addr(), n)
	s.len = uint32(n)
	return s
}

// Cap returns this slice's capacity.
func (s Slice[_]) Cap() int {
	return int(s.cap)
}

// Load loads a value at the given index.
func (s Slice[T]) Load(n int) T {
	if dbg.Enabled {
		return s.Raw()[n]
	}
	return unsafe2.Load(s.Ptr(), n)
}

// Store stores a value at the given index.
func (s Slice[T]) Store(n int, v T) {
	if dbg.Enabled {
		s.Raw()[n] = v
	}
	unsafe2.Store(s.Ptr(), n, v)
}

// Raw returns the underlying slice for this slice.
//
// The return value of this function must never escape outside of this module.
func (s Slice[T]) Raw() []T {
	return unsafe2.Slice2(s.Ptr(), s.len, s.cap)
}

// Rest returns the portion of s between the length and the capacity.
//
// The return value of this function must never escape outside of this module.
func (s Slice[T]) Rest() []T {
	return unsafe2.Slice(unsafe2.Add(s.Ptr(), s.len), s.cap-s.len)
}

// Append appends the given elements to a slice, reallocating on the given
// arena if necessary.
func (s Slice[T]) Append(a *Arena, elems ...T) Slice[T] {
	var z T
	a.Log("append", "%p[%d:%d], %T x %d", s.ptr, s.len, s.cap, z, len(elems))

	if s.Cap()-s.Len() < len(elems) {
		s = s.Grow(a, len(elems))
	}

	copy(s.Rest(), elems)
	s.len += uint32(len(elems))
	return s
}

// AppendOne is an optimized version of append for one element.
//
//go:nosplit
func (s Slice[T]) AppendOne(a *Arena, elem T) Slice[T] {
	a.Log("append", "%p[%d:%d], %T x 1", s.ptr, s.len, s.cap, elem)

	if s.Len() == s.Cap() {
		s = s.Grow(a, 1)
	}

	unsafe2.Store(s.Ptr(), s.len, elem)
	s.len += 1
	return s
}

// Grow extends the capacity of this slice by n bytes.
func (s Slice[T]) Grow(a *Arena, n int) Slice[T] {
	var z T
	size, _ := unsafe2.Layout[T]()
	a.Log("grow", "%p[%d:%d], %d x %T", s.ptr, s.len, s.cap, n, z)

	if s.ptr == nil {
		cap := sliceLayout[T](n)
		s.ptr = unsafe2.Cast[T](a.Alloc(cap))
		s.cap = uint32(cap) / uint32(size)
		return s
	}

	oldSize := sliceLayout[T](s.Cap())
	newSize := sliceLayout[T](s.Cap() + n)

	p := a.realloc(newSize, oldSize, unsafe2.Cast[byte](s.ptr))
	s.ptr = unsafe2.Cast[T](p)
	s.cap = uint32(newSize) / uint32(size)
	return s
}

func sliceLayout[T any](n int) (size int) {
	size, align := unsafe2.Layout[T]()
	if align > Align {
		panic("fastpb: over-aligned object")
	}
	return suggestSize(size * n)
}

// Format implements [fmt.Formatter].
func (s Slice[T]) Format(state fmt.State, v rune) {
	if s.Ptr() == nil && (s.Len() != 0 || s.Cap() != 0) {
		fmt.Fprintf(state, "%v", s.Addr())
		return
	}

	fmt.Fprintf(state, fmt.FormatString(state, v), s.Raw())
}

// String implements [fmt.Stringer].
func (s SliceAddr[T]) String() string {
	return fmt.Sprintf("%v[%d:%d]", s.ptr, s.len, s.cap)
}
