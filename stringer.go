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
	"math/bits"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Stringer implementations for various internal types. These are only relevant
// for debugging and are thus placed off to the side here.

func (f *field) Format(s fmt.State, verb rune) {
	dbg.Dict("", "message", f.message, "getter", &f.getter).Format(s, verb)
}

func (g *getter) Format(s fmt.State, verb rune) {
	dbg.Dict("", "offset", g.offset, "getter", dbg.Func(g.thunk)).Format(s, verb)
}

func (p *fieldParser) Format(s fmt.State, verb rune) {
	dbg.Dict(
		dbg.Fprintf("%p", p),
		"tag", p.tag,
		"message", func() any {
			if p.message == nil {
				return nil
			}
			return p.message.ty
		}(),
		"offset", p.offset,
		"next", func() any {
			if p.nextOk == p.nextErr {
				return dbg.Fprintf("%p", p.nextOk)
			}
			return dbg.Fprintf("%p/%p", p.nextOk, p.nextErr)
		}(),
		"thunk", dbg.Func(p.thunk),
	).Format(s, verb)
}

func (f fieldOffset) Format(s fmt.State, verb rune) {
	dbg.Fprintf("%d:%d", f.bit, f.data).Format(s, verb)
}

func (ft fieldTag) Format(s fmt.State, verb rune) {
	v := ft.decode()
	n, t := protowire.DecodeTag(v)
	dbg.Fprintf("%#x:%d:%d", uint64(ft), n, t).Format(s, verb)
}

func (zc zc) Format(s fmt.State, verb rune) {
	dbg.Fprintf("[%d:%d]", zc.offset, zc.offset+zc.len).Format(s, verb)
}

func (r rep[E]) Format(s fmt.State, verb rune) {
	if r.isZC() {
		fmt.Fprintf(s, "%v", r.rawZC())
	}

	fmt.Fprintf(s, "%v:%v", r.ptr, r.arena())
}

func (t *typeHeader) Format(s fmt.State, verb rune) {
	dbg.Dict(
		dbg.Fprintf("%p", t),
		"name", t.aux.desc.FullName(),
		"size", t.size,
		"count", t.count,
		"parser", dbg.Fprintf("%p", t.parser),
	).Format(s, verb)
}

func (p *typeParser) Format(s fmt.State, verb rune) {
	dbg.Dict(
		dbg.Fprintf("%p", p),
		"ty", p.ty,
		"tags", p.tags,
	).Format(s, verb)
}

var _ = (*message).dump // Mark this function as used.

// dump dumps the internal state of a message.
func (m *message) dump() string {
	buf := new(strings.Builder)
	fmt.Fprintf(buf, "type: %p:%v\n", m.ty.Descriptor(), m.ty.Descriptor().FullName())
	fmt.Fprintf(buf, "header: %p:%p:%p\n", m, m.context, m.ty.raw)

	if !dbg.Enabled {
		fmt.Fprintln(buf, "bits: ???")
		fmt.Fprintln(buf, "fields: ???")
		return buf.String()
	}

	layout := m.ty.raw.aux.layout.Get()

	// Print out the bit words.
	if layout.bitWords > 0 {
		fmt.Fprint(buf, "bits:")
		words := unsafe2.Beyond[uint64](m).Slice(layout.bitWords)
		for _, word := range words {
			fmt.Fprintf(buf, " %064b", bits.Reverse64(word))
		}
		fmt.Fprintln(buf)
	}

	// Print out each field.
	if len(layout.fields) > 0 {
		fmt.Fprintln(buf, "fields:")
		fields := m.ty.Descriptor().Fields()
		for _, field := range layout.fields {
			start := buf.Len()
			fd := fields.Get(field.index)
			fmt.Fprintf(buf, "  %#04x %s/%d:", field.offset.data, fd.Name(), fd.Number())

			for buf.Len()-start < 24 {
				buf.WriteByte(' ')
			}

			if field.arch.bits > 0 {
				fmt.Fprint(buf, " (")
				for i := range field.arch.bits {
					if m.getBit(i + field.offset.bit) {
						fmt.Fprint(buf, "1")
					} else {
						fmt.Fprint(buf, "0")
					}
				}
				fmt.Fprint(buf, ")")
			}

			for buf.Len()-start < 30 {
				buf.WriteByte(' ')
			}

			// Print each byte of data, grouped into words of four.
			for i := range field.arch.size + uint32(field.padding) {
				if i == field.arch.size {
					fmt.Fprint(buf, " | ")
				} else if i%4 == 0 {
					fmt.Fprint(buf, " ")
				}
				fmt.Fprintf(buf, "%02x", unsafe2.ByteLoad[byte](m, field.offset.data+i))
			}

			if field.arch.size == uint32(zcSize) {
				zc := unsafe2.ByteLoad[zc](m, field.offset.data)
				start := int(zc.offset)
				end := start + int(zc.len)
				if m.context.len >= start && m.context.len >= end {
					fmt.Fprintf(buf, " %q", zc.bytes(m.context.src))
				}
			}

			fmt.Fprintln(buf)
		}
	}

	return buf.String()
}
