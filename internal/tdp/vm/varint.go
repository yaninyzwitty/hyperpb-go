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

package vm

import (
	"github.com/bufbuild/fastpb/internal/debug"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// parseVarint is the core varint parsing implementation.
//
//go:nosplit
func parseVarint(p1 P1, p2 P2) (P1, P2, uint64) {
	// Inlined from protowire.ConsumeVarint to minimize spills and remove
	// bounds checks.
	var b byte
	var x uint64
	var i int
	p := p1.Ptr()

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if b <= 1 {
		goto exit
	}

	p1.Fail(p2, ErrorOverflow)

exit:
	if debug.Enabled {
		len := int(unsafe2.AddrOf(p) - p1.PtrAddr) // For debug only.
		p1.Log(p2, "varint", "%d:%#x (%d bytes)", x, x, len)
	}

	p1.PtrAddr = unsafe2.AddrOf(p)
	if p1.Len() < 0 {
		p1.Fail(p2, ErrorTruncated)
	}

	return p1, p2, x
}

//go:noinline
func parseVarintNoinline(p1 P1, p2 P2) (P1, P2, uint64) {
	return parseVarint(p1, p2)
}
