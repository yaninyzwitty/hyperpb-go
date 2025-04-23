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

const (
	SliceSize  = unsafe2.PointerSize + unsafe2.Int32Size*2
	SliceAlign = max(unsafe2.PointerAlign, unsafe2.Int32Align)
)

// SliceFromParts assembles a slice from its raw components.
func SliceFromParts[T any](ptr *T, len, cap uint32) Slice[T] {
	return Slice[T]{ptr, len, cap}
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

// Cap returns this slice's capacity.
func (s Slice[_]) Cap() int {
	return int(s.cap)
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
	a.Log("grow", "%p[%d:%d], %d", s.ptr, s.len, s.cap, n)

	if s.ptr == nil {
		cap := sliceLayout[T](n)
		s.ptr = unsafe2.Cast[T](a.Alloc(cap))
		s.cap = uint32(cap)
		return s
	}

	oldSize := sliceLayout[T](s.Cap())
	newSize := sliceLayout[T](s.Cap() + n)

	size, _ := unsafe2.Layout[T]()
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
