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

// Package unsafe provides a more convenient interface for performing unsafe
// operations than Go's built-in package unsafe.
package unsafe2

import (
	"fmt"
	"reflect"
	"sync"
	"unsafe"
)

const (
	PointerSize  = int(unsafe.Sizeof(unsafe.Pointer(nil)))
	PointerAlign = int(unsafe.Sizeof(unsafe.Pointer(nil)))

	Int32Size  = int(unsafe.Sizeof(int32(0)))
	Int32Align = int(unsafe.Sizeof(int32(0)))

	Int64Size  = int(unsafe.Sizeof(int64(0)))
	Int64Align = int(unsafe.Sizeof(int64(0)))
)

// Int is any integer type.
type Int interface {
	int | int8 | int16 | int32 | int64 |
		uint | uint8 | uint16 | uint32 | uint64 |
		uintptr
}

// Layout returns the size and alignment of a given type.
func Layout[T any]() (size, align int) {
	var z T
	return int(unsafe.Sizeof(z)), int(unsafe.Alignof(z))
}

// BitCast performs an unsafe bitcast from one type to another.
func BitCast[To, From any](v From) To {
	return *(*To)(unsafe.Pointer(&v))
}

// Cast casts one pointer type to another.
func Cast[To, From any](p *From) *To {
	return (*To)(unsafe.Pointer(p))
}

// Add adds the given offset to p, scaled by the size of T.
func Add[P ~*E, E any, I Int](p P, n I) P {
	size, _ := Layout[E]()
	return P(unsafe.Add(unsafe.Pointer(p), uintptr(size)*uintptr(n)))
}

// Sub computes the difference between two pointers, scaled by the size of T.
func Sub[P ~*E, E any](p1, p2 P) int {
	size, _ := Layout[E]()
	return int(uintptr(unsafe.Pointer(p1))-uintptr(unsafe.Pointer(p2))) / size
}

// Load loads a value of the given type at the given index.
func Load[P ~*E, E any, I Int](p P, n I) E {
	return *Add(p, n)
}

// LoadSlice loads a slice without performing a bounds check.
func LoadSlice[S ~[]E, E any, I Int](s S, n I) E {
	return Load(unsafe.SliceData(s), n)
}

// Store stores a value at the given index.
func Store[P ~*E, E any, I Int](p P, n I, v E) {
	*Add(p, n) = v
}

// StoreNoWB performs a store without generating any write barriers.
func StoreNoWB[E any](p **E, q *E) {
	*Cast[Addr[E]](p) = AddrOf(q)
}

// ByteAdd adds the given offset to p, without scaling.
func ByteAdd[P ~*E, E any, I Int](p P, n I) P {
	return P(unsafe.Add(unsafe.Pointer(p), uintptr(n)))
}

// ByteLoad loads a value of the given type at the given byte offset.
func ByteLoad[T any, P ~*E, E any, I Int](p P, n I) T {
	return *Cast[T](ByteAdd(p, n))
}

// ByteLoad stores a value of the given type at the given byte offset.
func ByteStore[T any, P ~*E, E any, I Int](p P, n I, v T) {
	*Cast[T](ByteAdd(p, n)) = v
}

// Ping reminds the processor that *p should be loaded into the data cache.
func Ping[P ~*E, E any](p P) {
	_ = ByteLoad[byte](NoEscape(p), 0)
}

// Misalign returns the misalignment for an address: i.e., the byte offset to
// make this pointer aligned to the previous, or next, align-aligned word.
//
// align must be a power of two. If p is aligned, returns 0, 0.
func Misalign[P ~*E, E any](p P, align int) (prev, next int) {
	return AddrOf(p).Misalign(align)
}

// Slice is like [unsafe.Slice], but isn't as branchy.
func Slice[P ~*E, E any, I Int](p P, len I) []E {
	return Slice2(p, len, len)
}

// Slice2 is like [unsafe.Slice], but allows specifying length and capacity
// separately.
func Slice2[P ~*E, E any, I Int](p P, len, cap I) []E {
	return unsafe.Slice(p, cap)[:len]
}

// Bytes converts a pointer into a slice of its contents.
func Bytes[P ~*E, E any](p P) []byte {
	size, _ := Layout[E]()
	return Slice(Cast[byte](p), size)
}

// String is like [unsafe.String], but isn't as branchy.
func String[P ~*E, E any, I Int](p P, len I) string {
	size, _ := Layout[E]()
	slice := struct {
		ptr P
		len int
	}{p, int(len) * size}

	return BitCast[string](slice)
}

// Copy copies n elements from one pointer to the other.
func Copy[P ~*E, E any, I Int](dst, src P, n I) {
	copy(Slice(dst, n), Slice(src, n))
}

// Clear zeros n elements at p.
func Clear[P ~*E, E any, I Int](p P, n I) {
	clear(Slice(p, n))
}

var (
	alwaysFalse bool
	sink        unsafe.Pointer //nolint:unused
)

// Escape escapes a pointer to the heap.
func Escape[P ~*E, E any](p P) P {
	if alwaysFalse {
		sink = unsafe.Pointer(p)
	}
	return p
}

// NoEscape hides a pointer from escape analysis, preventing it from
// escaping to the heap.
func NoEscape[P ~*E, E any](p P) P {
	//nolint:staticcheck // False positive: complains that p^0 does nothing.
	return P((AddrOf(p) ^ 0).AssertValid())
}

// SliceToString converts a slice into a string, multiplying the slice length
// as appropriate.
func SliceToString[S ~[]T, T any](s S) string {
	size, _ := Layout[T]()
	str := struct {
		ptr *T
		len int
	}{unsafe.SliceData(s), len(s) * size}
	return BitCast[string](str)
}

// AnyData extracts the pointer value from an any.
func AnyData(v any) *byte {
	type iface struct {
		_, data *byte
	}
	return Cast[iface](&v).data
}

// AnyBytes extracts a slice pointing to the variable-length data of an any.
func AnyBytes(v any) []byte {
	if v == nil {
		return nil
	}

	return unsafe.Slice(
		AnyData(v),
		int(reflect.TypeOf(v).Size()),
	)
}

// Addr is a typed raw address.
type Addr[T any] uintptr

// AddrOf gets the address of a pointer.
func AddrOf[P ~*E, E any](p P) Addr[E] {
	return Addr[E](unsafe.Pointer(p))
}

// AssertValid asserts that this address is a valid pointer.
//
//go:nosplit
func (a Addr[T]) AssertValid() *T {
	return (*T)(unsafe.Pointer(a)) // Don't worry about it.
}

// Add adds the given offset to this address.
func (a Addr[T]) Add(n int) Addr[T] {
	size, _ := Layout[T]()
	return a + Addr[T](n*size)
}

// Add adds the given offset to this address.
func (a Addr[T]) Sub(b Addr[T]) int {
	size, _ := Layout[T]()
	return int(a-b) / size
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

// Format implements [fmt.Formatter].
func (a Addr[T]) Format(state fmt.State, verb rune) {
	if verb == 'v' {
		fmt.Fprintf(state, "%#x", uintptr(a))
		return
	}

	fmt.Fprintf(state, fmt.FormatString(state, verb), uintptr(a))
}

// VLA is a mechanism for accessing a variable-length array that follows
// some struct.
type VLA[T any] [0]T

// Beyond obtains the VLA past the end of p.
func Beyond[T, Header any](p *Header) *VLA[T] {
	return &Cast[struct {
		_   Header
		VLA VLA[T]
	}](p).VLA
}

// Get returns a pointer to the nth element of this array.
func (a *VLA[T]) Get(n int) *T {
	return Add(Cast[T](a), n)
}

// Slice converts this VLA into a slice of the given length.
func (a *VLA[T]) Slice(n int) []T {
	return unsafe.Slice(a.Get(0), n)
}

// NoCopy is a type that go vet will complain about having been moved.
//
// It does so by implementing [sync.Locker].
type NoCopy [0]sync.Mutex

// Func is a raw function pointer, which can be used to store captureless
// funcs.
//
// Suppose a func() is in rax. Go implements calling it by emitting the
// following code:
//
//	mov  rdx, rax
//	mov  rcx, [rdx]
//	call rcx
//
// For a captureless func, this load will be of a constant containing the PC
// of the function to call. This can result in cache misses. This type works
// around that by keeping the PC local, so the resulting load avoids this
// problem.
type PC[F any] uintptr

// NewPC wraps a func. This performs no checking that the func does not
// capture any variables.
func NewPC[F any](f F) PC[F] {
	// Recall that a func()'s layout is *runtime.funcval, and PC[F] is emulating
	// runtime.funcval.
	return *BitCast[*PC[F]](f)
}

// Get returns the func this PC wraps.
func (pc *PC[F]) Get() F {
	return BitCast[F](pc)
}
