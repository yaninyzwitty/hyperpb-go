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

package unsafe2

import (
	"fmt"
	"unsafe"

	"github.com/bufbuild/hyperpb/internal/unsafe2/layout"
)

// intptr is an integer type with the same layout as a uintptr but signed.
//
// On every platform we support, int and uintptr have the same layout.
type intptr int

// Addr is a typed raw address.
//
// The underlying type is an int64 in order to work around a Go codegen bug.
// The bug is essentially that we want to do an arithmetic shift on the value,
// which requires casting what would normally be a uintptr to int64. For some
// reason, when in a generic context, this confuses Go's inliner *just
// enough* to cause things to fail to inline, resulting in a generic function
// call on the critical path.
type Addr[T any] intptr

// AddrOf gets the address of a pointer.
func AddrOf[P ~*E, E any](p P) Addr[E] {
	return Addr[E](uintptr(unsafe.Pointer(p)))
}

// AssertValid asserts that this address is a valid pointer.
//
//go:nosplit
func (a Addr[T]) AssertValid() *T {
	return (*T)(unsafe.Pointer(uintptr(a))) // Don't worry about it.
}

// Add adds the given offset to this address.
func (a Addr[T]) Add(n int) Addr[T] {
	return a + Addr[T](n*layout.Size[T]())
}

// ByteAdd adds the given unscaled offset to this address.
func (a Addr[T]) ByteAdd(n int) Addr[T] {
	return a + Addr[T](n)
}

// Add adds the given offset to this address.
func (a Addr[T]) Sub(b Addr[T]) int {
	return int(a-b) / layout.Size[T]()
}

// Misalign returns the misalignment for an address: i.e., the byte offset to
// make this pointer aligned to the previous, or next, align-aligned word.
//
// align must be a power of two. If p is aligned, returns 0, 0.
func (a Addr[T]) Misalign(align int) (prev, next int) {
	addr := int(a)
	prev = addr & (align - 1)           // p % align
	next = (align - addr) & (align - 1) // (align - p) % align
	return prev, next
}

// SignBit returns whether this address has its sign bit set.
//
// Pointers with the high bits set are never used by Go, so we can use this bit
// to store extra information.
func (a Addr[T]) SignBit() bool {
	return a>>(layout.Bits[Addr[T]]()-1) != 0
}

// SignBitMask returns either all zeros or all ones, according to the sign bit
// of a.
func (a Addr[T]) SignBitMask() Addr[T] {
	return a >> (layout.Bits[Addr[T]]() - 1)
}

// ClearSignBit clears the sign bit of this address, flipping all of the other
// bits in the process.
func (a Addr[T]) ClearSignBit() Addr[T] {
	return a ^ a.SignBitMask()
}

// Format implements [fmt.Formatter].
func (a Addr[T]) Format(state fmt.State, verb rune) {
	if verb == 'v' {
		fmt.Fprintf(state, "%#x", uintptr(a))
		return
	}

	fmt.Fprintf(state, fmt.FormatString(state, verb), uintptr(a))
}
