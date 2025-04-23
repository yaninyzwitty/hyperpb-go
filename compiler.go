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
	"reflect"
	"runtime"
	"slices"
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/table"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// CompileOption is a configuration setting for [Compile].
type CompileOption func(*compiler)

// Compile compiles a descriptor into a [Type], for optimized parsing.
//
// Panics if md is too complicated (i.e. it exceeds internal limitations for the compiler).
func Compile(md protoreflect.MessageDescriptor, options ...CompileOption) Type {
	c := &compiler{
		symbols: make(map[any]int),
		relos:   make(map[int]any),

		layouts: make(map[protoreflect.MessageDescriptor]typeLayout),
	}
	for _, opt := range options {
		if opt != nil {
			opt(c)
		}
	}

	return c.compile(md)
}

// FieldSite is "call site" information for a message field. This type is the
// key used to look up information in a profile. See [PGO].
type FieldSite struct {
	// The field in question.
	Field protoreflect.FieldDescriptor
}

// FieldProfile is profiling information returned by a profile passed to
// [PGO].
//
// The zero value of this type results in "default behavior".
type FieldProfile struct {
	// If true, this indicates that this field is rarely seen in its parent
	// message and should take a slow path.
	Cold bool
}

// PGO adds profile-guided optimization information to a compiler.
//
// Profile is a function that returns profiling information for a given field.
func PGO(prof func(site FieldSite) FieldProfile) CompileOption {
	return func(c *compiler) { c.prof = prof }
}

// compiler is context for compiling a descriptor into a [Type].
type compiler struct {
	buf        []byte
	totalTypes int

	// Maps used for linking. Symbols maps arbitrary keys to an offset in buf
	// and relos maps offsets to pointer values that should be filled in with
	// the final pointer value for that symbol.
	symbols map[any]int
	relos   map[int]any

	layouts map[protoreflect.MessageDescriptor]typeLayout

	prof func(FieldSite) FieldProfile
}

type typeSymbol struct {
	ty protoreflect.MessageDescriptor
}

type parserSymbol struct {
	ty protoreflect.MessageDescriptor
}

type tableSymbol struct{ sym any }

type fieldParserSymbol struct {
	parser any
	index  int
}

func (c *compiler) compile(md protoreflect.MessageDescriptor) Type {
	c.message(md)

	if len(c.buf) > math.MaxInt32 {
		panic(fmt.Errorf("tdp: type has too many dependencies: %s", md.FullName()))
	}

	auxes := make([]typeAux, c.totalTypes)

	// Copy buf onto some memory that the GC can trace through md to keep all of
	// the descriptors alive.
	p := arena.AllocTraceable(len(c.buf), unsafe.Pointer(unsafe.SliceData(auxes)))
	copy(unsafe2.Slice(p, len(c.buf)), c.buf)

	// Resolve all relocations.
	c.link(p)

	// Resolve all message type references. This needs to be done as a separate
	// step due to potential cycles.
	base := unsafe2.Cast[typeHeader](p)
	var i int
	for symbol, offset := range c.symbols {
		sym, ok := symbol.(typeSymbol)
		if !ok {
			continue
		}

		ty := Type{raw: unsafe2.ByteAdd(base, offset)}
		ty.raw.aux = &auxes[i]
		i++

		ty.raw.aux.desc = sym.ty
		ty.raw.aux.methods.Unmarshal = unmarshalShim
		ty.raw.aux.methods.CheckInitialized = requiredShim

		if dbg.Enabled {
			*ty.raw.aux.layout.Get() = c.layouts[sym.ty]
		}
	}

	if dbg.Enabled {
		runtime.SetFinalizer(base, func(t *typeHeader) {
			dbg.Log(nil, "finalizer", "%p:%s", t, t.aux.desc.FullName())
		})
	}

	return Type{raw: base}
}

func (c *compiler) message(md protoreflect.MessageDescriptor) {
	tSym := typeSymbol{md}
	pSym := parserSymbol{md}

	if _, ok := c.symbols[tSym]; ok {
		return
	}

	fields := md.Fields()
	tOffset := c.write(tSym,
		typeHeader{
			count: uint32(fields.Len()),
		},
		relo{
			symbol: pSym,
			offset: unsafe.Offsetof(typeHeader{}.parser),
		},
		relo{
			symbol: tableSymbol{tSym},
			offset: unsafe.Offsetof(typeHeader{}.numbers),
		},
	)
	c.totalTypes++

	type entry struct {
		tIdx, pIdx int
		arch       *archetype
		fd         protoreflect.FieldDescriptor
	}

	// Classify all of the fields into entries.
	entries := make([]entry, 0, fields.Len())
	var totalBits int
	for i := range fields.Len() {
		fd := fields.Get(i)
		arch := selectArchetype(fd, func(fd protoreflect.FieldDescriptor) FieldProfile {
			if c.prof == nil {
				return FieldProfile{}
			}
			return c.prof(FieldSite{Field: fd})
		})
		if arch != nil {
			// TODO: oneof members should be excluded here and processed separately,
			// since each oneof will correspond to only one entry.
			entries = append(entries, entry{
				tIdx: i,
				arch: arch,
				fd:   fd,
			})
			totalBits += int(arch.bits)
		}

		var relos []relo
		if md := fd.Message(); md != nil {
			relos = []relo{{
				symbol: typeSymbol{md},
				offset: unsafe.Offsetof(field{}.message),
			}}
		}

		// Append whatever field data we can before doing layout.
		c.write(nil, field{}, relos...)
	}
	// Append the dummy end field.
	c.write(nil, field{})

	// Append the field number table.
	numbers := make([]table.Entry[uint32], fields.Len())
	for i := range numbers {
		numbers[i].Key = int32(fields.Get(i).Number())
		numbers[i].Value = uint32(i)
	}
	c.writeFunc(
		tableSymbol{tSym},
		func(b []byte) (int, []byte) {
			b, t := table.New(b, numbers...)
			return unsafe2.Sub(t.Data, unsafe.SliceData(b)), b
		},
	)

	// Append the "unspecialized" parser for this type.
	pOffset := c.write(pSym,
		typeParser{},
		relo{
			symbol: tSym,
			offset: unsafe.Offsetof(typeParser{}.ty),
		},
		relo{
			symbol: tableSymbol{pSym},
			offset: unsafe.Offsetof(typeParser{}.tags),
		},
		relo{
			symbol: fieldParserSymbol{
				parser: pSym,
				index:  0,
			},
			offset: unsafe.Offsetof(typeParser{}.entry) + unsafe.Offsetof(fieldParser{}.nextOk),
		},
	)

	numbers = numbers[:0]
	// Lay out the parser table.
	var j int
	for i := range entries {
		e := &entries[i]
		e.pIdx = j

		for k, p := range e.arch.parsers {
			var tag fieldTag
			tag.encode(e.fd.Number(), p.kind)

			numbers = append(numbers, table.Entry[uint32]{
				Key:   int32(protowire.EncodeTag(e.fd.Number(), p.kind)),
				Value: uint32(j),
			})

			nextOk := e.pIdx + len(e.arch.parsers)
			nextErr := j + 1
			if i == len(entries)-1 && k == len(e.arch.parsers)-1 {
				nextOk, nextErr = 0, 0
			}

			if p.retry {
				nextOk = j
			}

			relos := []relo{
				{
					symbol: fieldParserSymbol{
						parser: pSym,
						index:  nextOk,
					},
					offset: unsafe.Offsetof(fieldParser{}.nextOk),
				},
				{
					symbol: fieldParserSymbol{
						parser: pSym,
						index:  nextErr,
					},
					offset: unsafe.Offsetof(fieldParser{}.nextErr),
				},
			}
			if md := e.fd.Message(); md != nil {
				relos = append(relos, relo{
					symbol: parserSymbol{md},
					offset: unsafe.Offsetof(fieldParser{}.message),
				})
			}

			c.write(
				fieldParserSymbol{
					parser: pSym,
					index:  j,
				},
				fieldParser{
					tag:   tag,
					thunk: unsafe2.NewPC(p.parser),
				},
				relos...,
			)

			j++
		}
	}

	// Append the parser's field number table.
	c.writeFunc(
		tableSymbol{pSym},
		func(b []byte) (int, []byte) {
			b, t := table.New(b, numbers...)
			return unsafe2.Sub(t.Data, unsafe.SliceData(b)), b
		},
	)

	// Sort the fields by decreasing alignment. This means we do not need to
	// adjust fields to add padding as we lay them out.
	slices.SortStableFunc(entries, func(a, b entry) int {
		return int(b.arch.align) - int(a.arch.align)
	})

	var layout typeLayout

	// Figure out the number of bit words we need. We use words equal to the
	// int64 alignment.
	bitsPerWord := unsafe2.Int64Align * 8
	bitWords := totalBits / bitsPerWord
	if totalBits%bitsPerWord > 0 {
		bitWords++
	}
	bitBytes := bitWords * unsafe2.Int64Align
	layout.bitWords = bitWords

	size, _ := unsafe2.Layout[message]()
	size += bitBytes

	// Lay out the struct for this type.
	ty := Type{raw: unsafe2.Cast[typeHeader](unsafe2.Add(unsafe.SliceData(c.buf), tOffset))}
	parser := unsafe2.Cast[typeParser](unsafe2.Add(unsafe.SliceData(c.buf), pOffset))

	var nextBit uint32
	nextOffset := uint32(size)
	for _, e := range entries {
		f := ty.byIndex(e.tIdx)

		// Allocate bit and byte storage for this field.
		//
		// TODO: oneofs will require special handling here.
		var offset fieldOffset
		if e.arch.size > 0 {
			_, up := unsafe2.Addr[byte](nextOffset).Misalign(int(e.arch.align))
			nextOffset += uint32(up)
			if dbg.Enabled && up > 0 {
				layout.fields[len(layout.fields)-1].padding = up
			}

			offset.data = nextOffset
			nextOffset += e.arch.size
		}
		if e.arch.bits > 0 {
			offset.bit = nextBit
			nextBit += e.arch.bits
		}

		f.getter.offset = offset
		f.getter.thunk = e.arch.getter

		for i := range e.arch.parsers {
			parser.fields().Get(e.pIdx + i).offset = offset
		}

		if dbg.Enabled {
			layout.fields = append(layout.fields, fieldLayout{
				arch:   e.arch,
				index:  e.tIdx,
				offset: offset,
			})
		}
	}

	if dbg.Enabled {
		c.layouts[md] = layout
	}

	ty.raw.size = nextOffset
	for i := range fields.Len() {
		field := fields.Get(i)
		if m := field.Message(); m != nil {
			c.message(m)
		}
	}
}

// relo is a relocation that is resolved in [compiler.link].
type relo struct {
	symbol any
	offset uintptr
}

func (c *compiler) link(base *byte) {
	for target, symbol := range c.relos {
		offset, ok := c.symbols[symbol]
		if !ok {
			panic(fmt.Sprintf("fastpb: undefined symbol: %v", symbol))
		}

		target := unsafe2.Cast[*byte](unsafe2.Add(base, target))
		symbol := unsafe2.Add(base, offset)

		dbg.Log([]any{"%p", c}, "relo", "%#v %p->%p", symbol, target, symbol)
		*target = symbol
	}
}

// write writes a value to the inner buffer and returns its offset.
//
// If symbol is not nil, the offset is recorded as a symbol.
func (c *compiler) write(symbol, v any, relos ...relo) int {
	return c.writeFunc(symbol, func(b []byte) (int, []byte) {
		align := reflect.TypeOf(v).Align()
		_, up := unsafe2.Addr[byte](len(c.buf)).Misalign(align)
		b = append(b, make([]byte, up)...)

		return len(b), append(b, unsafe2.AnyBytes(v)...)
	}, relos...)
}

// writeFunc is like write, but it uses the given function to append data.
func (c *compiler) writeFunc(symbol any, f func([]byte) (int, []byte), relos ...relo) int {
	var offset int
	offset, c.buf = f(c.buf)

	if symbol != nil {
		if old, ok := c.symbols[symbol]; ok {
			panic(fmt.Sprintf("fastpb: symbol %#v defined twice: %#x, %#x", symbol, old, offset))
		}
		c.symbols[symbol] = offset
	}

	for _, relo := range relos {
		offset := int(relo.offset) + offset
		if _, ok := c.relos[offset]; ok {
			panic(fmt.Sprintf("fastpb: two relocations for the same offset %#x", offset))
		}
		c.relos[offset] = relo.symbol
	}

	return offset
}
