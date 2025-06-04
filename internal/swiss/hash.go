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

// hash is a simple hasher.
//
// It an fxhash derivative, which is a relatively high-quality hash that is
// completely branchless for integers. We use the variant used in the Rust
// compiler, which can be found here: <https://github.com/rust-lang/rustc-hash>.
type hash uint64

func (h hash) h1() uint64 { return uint64(h >> 7) }
func (h hash) h2() byte   { return ^(byte(h) & 0x7f) }

// zext zero-extends k regardless of its sign.
func zext[T Key](k T) uint64 {
	n := uint64(k)
	n &= 1<<(8*unsafe.Sizeof(k)) - 1
	return n
}

// u64 writes a single uint64 to this hash's state.
//
//go:nosplit
func (h hash) u64(n uint64) hash {
	const (
		rotate = 26
		key    = 0xf1357aea2e62a9c5
	)

	// Older versions of this used ^ instead of +. Addition seems to produce
	// a higher-quality hash, resulting in better latency overall.
	x := mix((uint64(h) + n), key)
	return hash(bits.RotateLeft64(x, rotate))
}

// bytes writes an arbitrary byte array to this hash's state.
//
//go:nosplit
func (h hash) bytes(in []byte) hash {
	const (
		// Digits of pi in hex.
		c0 uint64 = 0x243f6a8885a308d3
		c1 uint64 = 0x13198a2e03707344
		c2 uint64 = 0xa4093822299f31d0
	)

	x0, x1 := c0, c1
	p := unsafe.SliceData(in)
	n := uint64(len(in))

	// The branches below implement binary search on the size of the input
	// buffer to select the best hash for that size.
	if n <= 16 {
		if n >= 4 {
			if n >= 8 {
				x0 ^= unsafe2.ByteLoad[uint64](p, 0)
				x1 ^= unsafe2.ByteLoad[uint64](p, n-8)
			} else {
				x0 ^= uint64(unsafe2.ByteLoad[uint32](p, 0))
				x1 ^= uint64(unsafe2.ByteLoad[uint32](p, n-4))
			}
		} else if n > 0 {
			x0 ^= uint64(unsafe2.ByteLoad[uint8](p, 0))
			x1 ^= uint64(unsafe2.ByteLoad[uint8](p, n-1))
			x1 ^= uint64(unsafe2.ByteLoad[uint8](p, n/2)) << 8
		}
	} else {
		end := unsafe2.AddrOf(p).Add(int(n) - 16)
		for unsafe2.AddrOf(p) < end {
			// TODO: Go does not know how to unroll this loop (Go is bad
			// at unroll-and-jam), so in the future we can probably get better
			// perf for large buffers by unroll-and-jamming this loop ourselves.
			y0 := unsafe2.ByteLoad[uint64](p, 0)
			y1 := unsafe2.ByteLoad[uint64](p, 8)
			x0, x1 = x1, mix(x0^y0, c2^y1)
			p = unsafe2.Add(p, 16)
		}

		x0 ^= unsafe2.ByteLoad[uint64](end.AssertValid(), 0)
		x1 ^= unsafe2.ByteLoad[uint64](end.AssertValid(), 8)
	}

	return h.u64(mix(x0, x1) ^ n)
}

// String implements [fmt.Stringer].
func (h hash) String() string {
	return fmt.Sprintf("%015x:%02x", h.h1(), h.h2())
}

// mix mixes together the bits of a and b.
func mix(a, b uint64) uint64 {
	a, b = bits.Mul64(a, b)
	return a ^ b
}
