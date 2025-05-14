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
			return p.message.tyOffset
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
	dbg.Fprintf("%d:%d:%#04x", f.bit, f.number, f.data).Format(s, verb)
}

func (ft fieldTag) Format(s fmt.State, verb rune) {
	v := ft.decode()
	n, t := protowire.DecodeTag(v)
	dbg.Fprintf("%#x:%d:%d", uint64(ft), n, t).Format(s, verb)
}

func (zc zc) Format(s fmt.State, verb rune) {
	dbg.Fprintf("[%d:%d]", zc.offset, zc.offset+zc.len).Format(s, verb)
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
		"ty", p.tyOffset,
		"tags", p.tags,
	).Format(s, verb)
}

var _ = (*message).dump // Mark this function as used.

// dump dumps the internal state of a message.
func (m *message) dump() string {
	buf := new(strings.Builder)
	cold := m.cold()

	fmt.Fprintf(buf, "type: %p:%v, %p/%#x\n", m.ty().Descriptor(), m.ty().Descriptor().FullName(), m.ty().raw, m.tyOffset)
	fmt.Fprintf(buf, "hot:  %p, %d/%#[2]x\n", m, m.ty().raw.size)
	fmt.Fprintf(buf, "cold: %p, %d/%#[2]x\n", cold, m.ty().raw.coldSize)
	fmt.Fprintf(buf, "ctx:  %p\n", m.context)

	if !dbg.Enabled {
		fmt.Fprintln(buf, "bits: ???")
		fmt.Fprintln(buf, "fields: ???")
		return buf.String()
	}

	layout := m.ty().raw.aux.layout.Get()

	// Print out the bit words.
	if layout.bitWords > 0 {
		fmt.Fprint(buf, "bits:")
		words := unsafe2.Beyond[byte](m).Slice(layout.bitWords * 4)
		for i, word := range words {
			if i > 0 && i%4 == 0 {
				fmt.Fprintf(buf, "\n     ")
			}
			fmt.Fprintf(buf, " %08b", bits.Reverse8(word))
		}
		fmt.Fprintln(buf)
	}

	// Print out each field.
	if len(layout.fields) > 0 {
		fmt.Fprintln(buf, "fields:")
		oneofs := m.ty().Descriptor().Oneofs()

		var maxBits uint32
		for _, field := range layout.fields {
			maxBits = max(field.bits, maxBits)
		}

		for _, field := range layout.fields {
			start := buf.Len()
			data := getField[byte](m, field.offset)

			switch {
			case field.size == 0:
				fmt.Fprint(buf, "  0x----/0x")

				nybbles := (bits.Len(uint(unsafe2.AddrOf(data))) + 3) / 4
				for range nybbles {
					fmt.Fprint(buf, "-")
				}
			case field.offset.data < 0:
				fmt.Fprintf(buf, " ^%#04x/%p", ^field.offset.data, data)
			default:
				fmt.Fprintf(buf, "  %#04x/%p", field.offset.data, data)
			}

			if field.index >= 0 {
				fd := m.ty().raw.aux.fds[field.index]
				if fd.IsExtension() {
					fmt.Fprintf(buf, " [%s]/%d:", fd.Name(), fd.Number())
				} else {
					fmt.Fprintf(buf, " %s/%d:", fd.Name(), fd.Number())
				}
			} else {
				od := oneofs.Get(^field.index)
				fmt.Fprintf(buf, " %s/", od.Name())
				for i := range od.Fields().Len() {
					if i > 0 {
						buf.WriteByte(',')
					}
					fmt.Fprint(buf, od.Fields().Get(i).Number())
				}
				buf.WriteByte(':')
			}

			for buf.Len()-start < 32 {
				buf.WriteByte(' ')
			}

			if maxBits > 0 {
				fmt.Fprint(buf, " ")
				for i := range field.bits {
					if m.getBit(i + field.offset.bit) {
						fmt.Fprint(buf, "1")
					} else {
						fmt.Fprint(buf, "0")
					}
				}
				for range maxBits - field.bits {
					fmt.Fprint(buf, "-")
				}
			}

			// Print each byte of data, grouped into words of four.
			if data == nil {
				fmt.Fprintln(buf, "---")
				continue
			}

			for i := range field.size + field.padding {
				if i == field.size {
					fmt.Fprint(buf, " | ")
				} else if i%4 == 0 {
					fmt.Fprint(buf, " ")
				}
				fmt.Fprintf(buf, "%02x", unsafe2.ByteLoad[byte](data, i))
			}

			if field.size == uint32(zcSize) {
				zc := unsafe2.ByteLoad[zc](data, 0)
				start := int(zc.offset)
				end := start + int(zc.len)
				if start <= m.context.len && end <= m.context.len && start < end {
					fmt.Fprintf(buf, " %q", zc.bytes(m.context.src))
				}
			}

			fmt.Fprintln(buf)
		}
	}

	if cold != nil && cold.unknown.Len() > 0 {
		fmt.Fprint(buf, "unknown:")
		for _, unknown := range cold.unknown.Raw() {
			fmt.Fprintf(buf, "  %v `%x`\n", unknown, unknown.bytes(m.context.src))
		}
		fmt.Fprintln(buf)
	}

	return buf.String()
}
