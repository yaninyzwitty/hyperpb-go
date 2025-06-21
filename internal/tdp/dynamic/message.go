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

// Package dynamic contains the implementation of hyperpb's dynamic message types.
package dynamic

import (
	"fmt"
	"math/bits"
	"strings"

	"github.com/bufbuild/hyperpb/internal/arena/slice"
	"github.com/bufbuild/hyperpb/internal/debug"
	"github.com/bufbuild/hyperpb/internal/tdp"
	"github.com/bufbuild/hyperpb/internal/unsafe2"
	"github.com/bufbuild/hyperpb/internal/unsafe2/layout"
	"github.com/bufbuild/hyperpb/internal/zc"
)

// Cold is portions of a message that are located in context.Cold.
type Cold struct {
	Unknown slice.Slice[zc.Range] // Unknown field chunks.
}

// Message is a dynamic message value.
//
// A *Message lives on some arena, and all of its submessages do too. Because
// arenas are designed such that if a pointer to any of its allocated data is
// reachable, the whole arena is reachable, simply holding a *Message into
// the arena will keep everything else alive.
//
// This means that *Message values not being directly operated on by the
// application do not need to be marked by the GC, because their memory already
// gets marked whenever the GC sweeps a *root. As such, all of the fields of
// a Message are laid out in memory that follows it.
type Message struct {
	_ unsafe2.NoCopy

	Shared     *Shared
	TypeOffset uint32

	// Index into context.cold; negative means no cold pointer.
	// Exported for open-coding in the parser.
	ColdIndex int32

	// Fields follow this. The bitset words are allocated immediately after
	// the end of the message, so they are easy to offset to.
	//
	// The field data follows that, and offsets into the field data are already
	// baked to include both the message header and the bitset words.
}

// GetField returns the field data for a given message.
//
// Returns nil if the field is cold and there is no cold region allocated.
func GetField[T any](m *Message, offset tdp.Offset) *T {
	if offset.Data < 0 {
		cold := m.Cold()
		if cold == nil {
			return nil
		}
		return unsafe2.Cast[T](unsafe2.ByteAdd(cold, ^offset.Data))
	}
	return unsafe2.Cast[T](unsafe2.ByteAdd(m, offset.Data))
}

// GetBit gets the value of the nth bit from this message's bitset.
func (m *Message) GetBit(n uint32) bool {
	words := unsafe2.Cast[uint32](unsafe2.Add(m, 1))
	word := unsafe2.Load(words, int(n)/32)
	mask := uint32(1) << (n % 32)
	return word&mask != 0
}

// StBit sets the value of the nth bit from this message's bitset.
func (m *Message) SetBit(n uint32, flag bool) {
	words := unsafe2.Cast[uint32](unsafe2.Add(m, 1))
	word := unsafe2.Add(words, int(n)/32)
	mask := uint32(1) << (n % 32)

	if flag {
		*word |= mask
	} else {
		*word &^= mask
	}
}

// Type returns this message's type.
func (m *Message) Type() *tdp.Type {
	return m.Shared.lib.AtOffset(m.TypeOffset)
}

// cold returns a pointer to the cold region, or nil if it hasn't been allocated.
func (m *Message) Cold() *Cold {
	if m.ColdIndex < 0 {
		return nil
	}
	return unsafe2.LoadSlice(m.Shared.Cold, m.ColdIndex)
}

// MutableCold returns a pointer to the cold region, allocating one if needed.
func (m *Message) MutableCold() *Cold {
	if m.ColdIndex < 0 {
		size := int(m.Type().ColdSize)
		cold := unsafe2.Cast[Cold](m.Shared.arena.Alloc(size))

		m.ColdIndex = int32(len(m.Shared.Cold))
		m.Shared.Cold = append(m.Shared.Cold, cold)
		return cold
	}
	return unsafe2.LoadSlice(m.Shared.Cold, m.ColdIndex)
}

// Dump dumps the internal state of a message.
func (m *Message) Dump() string {
	buf := new(strings.Builder)
	cold := m.Cold()

	fmt.Fprintf(buf, "type: %p:%v, %p/%#x\n",
		m.Type().Descriptor, m.Type().Descriptor.FullName(), m.Type(), m.TypeOffset)
	fmt.Fprintf(buf, "hot:  %p, %d/%#[2]x\n", m, m.Type().Size)
	fmt.Fprintf(buf, "cold: %p, %d/%#[2]x\n", cold, m.Type().ColdSize)
	fmt.Fprintf(buf, "ctx:  %p\n", m.Shared)

	if !debug.Enabled {
		fmt.Fprintln(buf, "bits: ???")
		fmt.Fprintln(buf, "fields: ???")
		return buf.String()
	}

	tLayout := m.Type().Layout.Get()

	// Print out the bit words.
	if tLayout.BitWords > 0 {
		fmt.Fprint(buf, "bits:")
		words := unsafe2.Beyond[byte](m).Slice(tLayout.BitWords * 4)
		for i, word := range words {
			if i > 0 && i%4 == 0 {
				fmt.Fprintf(buf, "\n     ")
			}
			fmt.Fprintf(buf, " %08b", bits.Reverse8(word))
		}
		fmt.Fprintln(buf)
	}

	// Print out each field.
	if len(tLayout.Fields) > 0 {
		fmt.Fprintln(buf, "fields:")
		oneofs := m.Type().Descriptor.Oneofs()

		var maxBits uint32
		for _, field := range tLayout.Fields {
			maxBits = max(field.Bits, maxBits)
		}

		for _, field := range tLayout.Fields {
			start := buf.Len()
			data := GetField[byte](m, field.Offset)

			switch {
			case field.Size == 0:
				fmt.Fprint(buf, "  0x----/0x")

				nybbles := (bits.Len(uint(unsafe2.AddrOf(data))) + 3) / 4
				for range nybbles {
					fmt.Fprint(buf, "-")
				}
			case field.Offset.Data < 0:
				fmt.Fprintf(buf, " ^%#04x/%p", ^field.Offset.Data, data)
			default:
				fmt.Fprintf(buf, "  %#04x/%p", field.Offset.Data, data)
			}

			if field.Index >= 0 {
				fd := m.Type().FieldDescriptors[field.Index]
				if fd.IsExtension() {
					fmt.Fprintf(buf, " [%s]/%d:", fd.Name(), fd.Number())
				} else {
					fmt.Fprintf(buf, " %s/%d:", fd.Name(), fd.Number())
				}
			} else {
				od := oneofs.Get(^field.Index)
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
				for i := range field.Bits {
					if m.GetBit(i + field.Offset.Bit) {
						fmt.Fprint(buf, "1")
					} else {
						fmt.Fprint(buf, "0")
					}
				}
				for range maxBits - field.Bits {
					fmt.Fprint(buf, "-")
				}
			}

			// Print each byte of data, grouped into words of four.
			if data == nil {
				fmt.Fprintln(buf, "---")
				continue
			}

			for i := range field.Size + field.Padding {
				if i == field.Size {
					fmt.Fprint(buf, " | ")
				} else if i%4 == 0 {
					fmt.Fprint(buf, " ")
				}
				fmt.Fprintf(buf, "%02x", unsafe2.ByteLoad[byte](data, i))
			}

			if int(field.Size) == layout.Size[zc.Range]() {
				zc := unsafe2.ByteLoad[zc.Range](data, 0)
				start, end := zc.Start(), zc.End()
				if start <= m.Shared.Len && end <= m.Shared.Len && start < end {
					fmt.Fprintf(buf, " %q", zc.Bytes(m.Shared.Src))
				}
			}

			fmt.Fprintln(buf)
		}
	}

	if cold != nil && cold.Unknown.Len() > 0 {
		fmt.Fprint(buf, "unknown:")
		for _, unknown := range cold.Unknown.Raw() {
			fmt.Fprintf(buf, "  %v `%x`\n", unknown, unknown.Bytes(m.Shared.Src))
		}
		fmt.Fprintln(buf)
	}

	return buf.String()
}
