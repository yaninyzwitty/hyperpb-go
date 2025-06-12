// Copyright 2025 Buf Technologies, Inc.
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

package slice

import (
	"fmt"

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
)

// Slice is a slice that points into an arena.
//
// Unlike an ordinary slice, it does not contain pointers; in order to work
// correctly, it must be kept alive no longer than its owning arena.
type Slice[T any] struct {
	ptr      *T
	len, cap uint32
}

// FromParts assembles a slice from its raw components.
func FromParts[T any](ptr *T, len, cap uint32) Slice[T] {
	return Slice[T]{ptr, len, cap}
}

// Addr converts this slice into an address slice.
//
// See the caveats of [unsafe2.AddrOf].
func (s Slice[T]) Addr() Addr[T] {
	return Addr[T]{unsafe2.AddrOf(s.ptr), s.len, s.cap}
}

// Of allocates a slice for the given values.
func Of[T any](a *arena.Arena, values ...T) Slice[T] {
	s := Make[T](a, len(values))
	copy(s.Raw(), values)
	return s
}

// Make allocates a slice of the given length.
func Make[T any](a *arena.Arena, n int) Slice[T] {
	cap := sliceLayout[T](n)
	p := unsafe2.Cast[T](a.Alloc(cap))

	size := layout.Size[T]()
	s := FromParts(p, uint32(n), uint32(cap/size))
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
func (s Slice[T]) Append(a *arena.Arena, elems ...T) Slice[T] {
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
func (s Slice[T]) AppendOne(a *arena.Arena, elem T) Slice[T] {
	a.Log("append", "%p[%d:%d], %T x 1", s.ptr, s.len, s.cap, elem)

	if s.Len() == s.Cap() {
		s = s.Grow(a, 1)
	}

	unsafe2.Store(s.Ptr(), s.len, elem)
	s.len += 1
	return s
}

// Grow extends the capacity of this slice by n bytes.
func (s Slice[T]) Grow(a *arena.Arena, n int) Slice[T] {
	var z T
	size := layout.Size[T]()
	a.Log("grow", "%p[%d:%d], %d x %T", s.ptr, s.len, s.cap, n, z)

	if s.ptr == nil {
		cap := sliceLayout[T](n)
		s.ptr = unsafe2.Cast[T](a.Alloc(cap))
		s.cap = uint32(cap) / uint32(size)
		return s
	}

	oldSize := sliceLayout[T](s.Cap())
	newSize := sliceLayout[T](s.Cap() + n)

	p := a.Realloc(newSize, oldSize, unsafe2.Cast[byte](s.ptr))
	s.ptr = unsafe2.Cast[T](p)
	s.cap = uint32(newSize) / uint32(size)
	return s
}

// Format implements [fmt.Formatter].
func (s Slice[T]) Format(state fmt.State, v rune) {
	if s.Ptr() == nil && (s.Len() != 0 || s.Cap() != 0) {
		fmt.Fprintf(state, "%v", s.Addr())
		return
	}

	fmt.Fprintf(state, fmt.FormatString(state, v), s.Raw())
}

// Addr is like [Slice], but its pointer is replaced with an address, so
// loading/storing values of this type issues no write barriers.
type Addr[T any] struct {
	Ptr      unsafe2.Addr[T]
	Len, Cap uint32
}

// AssertValid converts this address slice into a true [Slice].
//
// See the caveats of [unsafe2.Addr.AssertValid].
func (s Addr[T]) AssertValid() Slice[T] {
	return Slice[T]{s.Ptr.ClearSignBit().AssertValid(), s.Len, s.Cap}
}

// Untyped converts this address slice into a true [Slice].
//
// See the caveats of [unsafe2.Addr.AssertValid].
func (s Addr[T]) Untyped() Untyped {
	return Untyped{
		Ptr: unsafe2.Addr[byte](s.Ptr),
		Len: s.Len,
		Cap: s.Cap,
	}
}

// String implements [fmt.Stringer].
func (s Addr[T]) String() string {
	return s.Untyped().String()
}

// Untyped is an [Addr] that has forgotten what type it is.
type Untyped struct {
	Ptr      unsafe2.Addr[byte]
	Len, Cap uint32
}

// OffArena creates a new off-arena slice.
//
// When cast to a concrete type, this will clear.
func OffArena[T any](ptr *T, len int) Untyped {
	return Untyped{
		Ptr: ^unsafe2.Addr[byte](unsafe2.AddrOf(ptr)),
		Len: uint32(len),
		Cap: uint32(len),
	}
}

// CastUntyped gives a type to a [Untyped], asserting it as valid in the
// process.
func CastUntyped[To any](s Untyped) Slice[To] {
	return Slice[To]{
		ptr: unsafe2.Addr[To](s.Ptr.ClearSignBit()).AssertValid(),
		len: s.Len,
		cap: s.Cap,
	}
}

// OffArena returns if this is off-arena memory, i.e., as created with
// [OffArena].
func (s Untyped) OffArena() bool {
	return s.Ptr.SignBit()
}

// String implements [fmt.Stringer].
func (s Untyped) String() string {
	return fmt.Sprintf("%v[%d:%d]", s.Ptr, s.Len, s.Cap)
}

func sliceLayout[T any](n int) (size int) {
	layout := layout.Of[T]()
	if layout.Align > arena.Align {
		panic("fastpb: over-aligned object")
	}
	return arena.SuggestSize(layout.Size * n)
}
