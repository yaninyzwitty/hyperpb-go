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

package fastpb

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// zc (short for zero-copy) is a representation of a []byte as a slice into
// the source array for a parsed message.
//
// This is a packed representation of a value with the layout
//
//	struct {
//	  offset, len uint32
//	}
//
// The zero value faithfully represents an empty slice.
type zc uint64

const (
	zcSize  = unsafe.Sizeof(zc(0))
	zcAlign = unsafe.Alignof(zc(0))
)

// newZC creates a new zc over the given source buffer with the given start
// and length.
func newZC(src *byte, start *byte, len int) zc {
	offset := unsafe2.Sub(start, src)
	return newRawZC(offset, len)
}

// newRawZC is like newZC, but it only takes the offset and length.
func newRawZC(offset, len int) zc {
	dbg.Assert(offset <= math.MaxUint32 && len <= math.MaxUint32,
		"offset too large for zc: [%d:%d]", offset, len)
	return zc(offset) | zc(len)<<32
}

// start returns the start offset of this slice in the message source.
func (zc zc) start() int { return int(uint32(zc)) }

// start returns the end offset of this slice in the message source.
func (zc zc) end() int { return zc.start() + zc.len() }

// len returns the length of this slice in the message source.
func (zc zc) len() int { return int(zc >> 32) }

// bytes converts this zc into a byte slice, given the message source.
func (zc zc) bytes(src *byte) []byte {
	if zc.len() == 0 {
		return nil
	}
	return unsafe2.Slice(unsafe2.Add(src, zc.start()), zc.len())
}

// utf8 converts this zc into a string, given the message source.
func (zc zc) utf8(src *byte) string {
	if zc.len() == 0 {
		return ""
	}
	return unsafe2.String(unsafe2.Add(src, zc.start()), zc.len())
}

// Format implements [fmt.Formatter].
func (zc zc) Format(s fmt.State, verb rune) {
	dbg.Fprintf("[%d:%d]", zc.start(), zc.end()).Format(s, verb)
}
