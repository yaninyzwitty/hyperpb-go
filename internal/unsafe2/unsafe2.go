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
	"sync"
	"unsafe"

	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
)

// NoCopy is a type that go vet will complain about having been moved.
//
// It does so by implementing [sync.Locker].
type NoCopy [0]sync.Mutex

// Int is any integer type.
type Int interface {
	int | int8 | int16 | int32 | int64 |
		uint | uint8 | uint16 | uint32 | uint64 |
		uintptr
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
	size := layout.Size[E]()
	return P(unsafe.Add(unsafe.Pointer(p), uintptr(size)*uintptr(n)))
}

// Sub computes the difference between two pointers, scaled by the size of T.
func Sub[P ~*E, E any](p1, p2 P) int {
	size := layout.Size[E]()
	return int(uintptr(unsafe.Pointer(p1))-uintptr(unsafe.Pointer(p2))) / size
}

// Load loads a value of the given type at the given index.
func Load[P ~*E, E any, I Int](p P, n I) E {
	return *Add(p, n)
}

// Store stores a value at the given index.
func Store[P ~*E, E any, I Int](p P, n I, v E) {
	*Add(p, n) = v
}

// StoreNoWB performs a store without generating any write barriers.
func StoreNoWB[P ~*E, E any](p *P, q P) {
	*Cast[uintptr](p) = uintptr(unsafe.Pointer(q))
}

// StoreNoWBUntyped performs a store without generating any write barriers.
func StoreNoWBUntyped[P ~unsafe.Pointer](p *P, q P) {
	*Cast[uintptr](p) = uintptr(q)
}

// ByteAdd adds the given offset to p, without scaling.
func ByteAdd[P ~*E, E any, I Int](p P, n I) P {
	return P(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(n)))
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
