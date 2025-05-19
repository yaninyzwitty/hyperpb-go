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

package swiss

import (
	"fmt"
	"math/bits"
	"unsafe"

	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// fxhash is a simple hasher.
type fxhash uint64

func (h fxhash) h1() uint64 { return uint64(h >> 7) }
func (h fxhash) h2() byte   { return ^(byte(h) & 0x7f) }

// zext zero-extends k regardless of its sign.
func zext[T Key](k T) uint64 {
	n := uint64(k)
	n &= 1<<(8*unsafe.Sizeof(k)) - 1
	return n
}

//go:nosplit
func (h fxhash) u64(n uint64) fxhash {
	const (
		rotate = 5
		key    = 0x517cc1b727220a95
	)

	// See https://docs.rs/fxhash.
	var lo, hi uint64
	hi, lo = bits.Mul64(bits.RotateLeft64(uint64(h), rotate)^n, key)
	h = fxhash(lo ^ hi)
	return h
}

//go:nosplit
func (h fxhash) bytes(b []byte) fxhash {
	h = h.u64(uint64(len(b)))
	if len(b) == 0 {
		return h
	}

	p := unsafe2.Cast[uint64](unsafe.SliceData(b))
	q := unsafe2.AddrOf(p).Add(len(b))
	var left int
	for {
		data := *p
		h = h.u64(data)

		left := int(q - unsafe2.AddrOf(p))
		if left < 8 {
			break
		}
		p = unsafe2.Add(p, 1)
	}

	switch {
	case left > 3:
		p := unsafe2.Cast[uint32](p)
		m := left - 4
		last := uint64(*p) | uint64(*unsafe2.ByteAdd(p, m))<<(m*8)
		return h.u64(last)
	case left > 0:
		p := unsafe2.Cast[uint8](p)
		last := uint64(*p) | uint64(*unsafe2.Add(p, left/2)) | uint64(*unsafe2.Add(p, left-1))
		return h.u64(last)
	default:
		return h
	}
}

// String implements [fmt.Stringer].
func (h fxhash) String() string {
	return fmt.Sprintf("%015x:%02x", h.h1(), h.h2())
}
