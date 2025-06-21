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

// Package zc provides helpers for working with zero-copy ranges.
package zc

import (
	"fmt"
	"math"

	"github.com/bufbuild/hyperpb/internal/debug"
	"github.com/bufbuild/hyperpb/internal/unsafe2"
)

// Range is a representation of a []byte as a slice relative to some larger byte
// array, such as the source of a parsed message.
//
// This is a packed representation of a value with the layout
//
//	struct {
//	  offset, len uint32
//	}
//
// The zero value faithfully represents an empty slice.
type Range uint64

// New creates a new Range over the given source buffer with the given start
// and length.
func New(src *byte, start *byte, len int) Range {
	offset := unsafe2.Sub(start, src)
	return NewRaw(offset, len)
}

// NewRaw is like newZC, but it only takes the offset and length.
func NewRaw(offset, len int) Range {
	debug.Assert(offset <= math.MaxUint32 && len <= math.MaxUint32,
		"offset too large for zc: [%d:%d]", offset, len)
	return Range(offset) | Range(len)<<32
}

// Start returns the start offset of this slice within its source.
func (r Range) Start() int { return int(uint32(r)) }

// End returns the end offset of this slice within its source.
func (r Range) End() int { return r.Start() + r.Len() }

// Len returns the length of this Range.
func (r Range) Len() int { return int(r >> 32) }

// Bytes converts this Range into a byte slice, given its source.
func (r Range) Bytes(src *byte) []byte {
	if r.Len() == 0 {
		return nil
	}
	return unsafe2.Slice(unsafe2.Add(src, r.Start()), r.Len())
}

// String converts this Range into a string, given its source.
func (r Range) String(src *byte) string {
	if r.Len() == 0 {
		return ""
	}
	return unsafe2.String(unsafe2.Add(src, r.Start()), r.Len())
}

// Format implements [fmt.Formatter].
func (r Range) Format(s fmt.State, verb rune) {
	debug.Fprintf("[%d:%d]", r.Start(), r.End()).Format(s, verb)
}
