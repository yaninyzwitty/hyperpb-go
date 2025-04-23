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
	"math/bits"
	"reflect"
	"unsafe"

	"github.com/bufbuild/fastpb/internal/unsafe2"
)

func suggestSizeLog(bytes int) uint {
	// Snap to the next power of two.
	return max(6, uint(bits.Len(uint(bytes)-1)))
}

// suggestSize suggests an allocation size by rounding up to a power of 2.
func suggestSize(bytes int) int {
	// Snap to the next power of two.
	n := 1 << suggestSizeLog(bytes)
	if bytes == 0 {
		return n
	}
	return n
}

func (a *Arena) allocChunk(size int) (*byte, int) {
	log := suggestSizeLog(size)
	n := 1 << log
	if int(log) < len(a.blocks) {
		if a.blocks[log] == nil {
			a.blocks[log] = AllocTraceable(n, unsafe.Pointer(a))
		}
		return a.blocks[log], n
	}

	p := AllocTraceable(n, unsafe.Pointer(a))
	a.blocks = append(a.blocks, make([]*byte, int(log+1)-len(a.blocks))...)
	a.blocks[log] = p

	return p, n
}

// AllocTraceable allocates size bytes of garbage-collected memory and returns
// a pointer to them.
//
// This function will also store ptr in the same allocation in such a way that
// as long as any pointer into the allocated memory is live, ptr will be marked
// as live by the garbage collector.
func AllocTraceable(size int, ptr unsafe.Pointer) *byte {
	// This needs to be done with reflection, because we need a weirdly-shaped
	// allocation: a bunch of bytes followed by a pointer.
	//
	// To avoid the overhead of hammering reflection, we cache the shape for
	// each power of two size. For non-powers of two, we hammer reflection
	// every time, because that path is not used by the arena implementation.
	var shape reflect.Type

	_, up := unsafe2.Addr[byte](size).Misalign(unsafe2.PointerAlign)
	size += up

	if isPow2(size) {
		shape = shapes[bits.TrailingZeros(uint(size))]
	} else {
		shape = chunkShape(size)
	}

	p := (*byte)(reflect.New(shape).UnsafePointer())
	unsafe2.ByteStore(p, size, ptr)

	// Skip over the arena pointer and return the data pointer.
	return p
}

// Pre-allocate a shape for every power of 2.
var shapes [bits.UintSize - 1]reflect.Type

func init() {
	for i := range shapes {
		shapes[i] = chunkShape(1 << i)
	}
}

func chunkShape(size int) reflect.Type {
	return reflect.StructOf([]reflect.StructField{
		{Name: "Data", Type: reflect.ArrayOf(size, reflect.TypeFor[byte]())},
		{Name: "Arena", Type: reflect.TypeFor[*Arena]()},
	})
}

func isPow2(n int) bool {
	return n&(n-1) == 0
}
