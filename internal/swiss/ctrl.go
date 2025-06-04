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

const (
	lows  = 0x0101_0101_0101_0101
	highs = lows << 7

	empty    = 0x00
	ctrlSize = int(unsafe.Sizeof(ctrl{}))
)

type prober struct {
	ctrl    *unsafe2.VLA[ctrl]
	i, mask int
	h1      int
}

func newProber(ctrlWords *unsafe2.VLA[ctrl], words int, hash hash) prober {
	return prober{
		ctrl: ctrlWords,
		mask: words - 1,
		h1:   int(hash.h1()) & (words - 1),
	}
}

// next returns the next control word and its index.
func (p prober) next() (prober, int, ctrl) {
	n := p.h1
	ctrl := *p.ctrl.Get(n)

	// We evaluate f(i) = (i^2 + i)/2 mod buckets recursively, noting that for
	// j = i+1,
	//
	//  f(j) = (j^2 + j)/2
	//       = (i^2 + 2i + 1 + i + 1)/2
	//       = (i^2 + i)/2 + (2i+2)/2
	//       = f(i) + i + 1
	//       = f(i) + j
	p.i++
	p.h1 += p.i
	p.h1 &= p.mask

	return p, n, ctrl
}

// ctrl is a control word, the heart of the Swisstable data structure.
type ctrl struct{ x0 uint64 }

// broadcast returns a control word whose bytes are each b.
func broadcast(b byte) ctrl {
	return ctrl{uint64(b) * lows}
}

// nonzero returns whether this control word contains any non-empty slots.
func (c ctrl) nonempty() bool {
	return c.x0 != 0
}

// match returns a control word whose nth byte is zero if and only if
// c[n] != needle[n].
func (c ctrl) matches(needle ctrl) ctrl {
	x0 := c.x0 ^ needle.x0
	return ctrl{
		x0: (x0 - lows) &^ x0 & highs,
	}
}

// next rotates this control word by 8 and returns whether the low byte
// was nonzero.
func (c ctrl) next() (ctrl, bool) {
	ok := c.x0&0xff != 0
	c.x0 = bits.RotateLeft64(c.x0, -8)
	return c, ok
}

// first returns the smallest n such that c[n] == needle[n], or ctrlSize if
// there is no such n.
func (c ctrl) first(needle ctrl) int {
	n0 := bits.TrailingZeros64(c.matches(needle).x0) / 8
	return n0
}

// String implements [fmt.Stringer].
func (c ctrl) String() string {
	return fmt.Sprintf("%016x", c.x0)
}
