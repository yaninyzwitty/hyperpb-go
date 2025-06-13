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

package compiler

import (
	"cmp"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"slices"
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/debug"
	"github.com/bufbuild/fastpb/internal/stats"
	"github.com/bufbuild/fastpb/internal/swiss"
	"github.com/bufbuild/fastpb/internal/tdp"
	"github.com/bufbuild/fastpb/internal/tdp/dynamic"
	"github.com/bufbuild/fastpb/internal/tdp/vm"
	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
)

// CompileOption is a configuration setting for [Compile].
type Options struct {
	Profile    Profile
	Extensions ExtensionResolver

	// Backend connects a [compiler] with backend configuration defined in another
	// package.
	//
	// This type mostly exists to break a circular dependency.
	Backend interface {
		// SelectArchetype classifies a field into a particular archetype.
		//
		// prof is a profile that inquires for the profile of a field within the context
		// of parsing fd. It takes a FieldDescriptor rather than a FieldSite because
		// the caller is responsible for constructing the FieldSite.
		//
		// Returns nil if the field is not supported yet.
		SelectArchetype(protoreflect.FieldDescriptor, FieldProfile) *Archetype

		// PopulateMethods gives the backend an opportunity to populate the
		// fast-path methods of the generated type.
		PopulateMethods(*protoiface.Methods)
	}
}

// Compile compiles a descriptor into a [Type], for optimized parsing.
//
// Panics if md is too complicated (i.e. it exceeds internal limitations for the compiler).
func Compile(md protoreflect.MessageDescriptor, options Options) *tdp.Type {
	c := &compiler{
		Options: options,

		symbols: make(map[any]int),
		relos:   make(map[int]relo),

		layouts: make(map[protoreflect.MessageDescriptor]tdp.TypeLayout),
		fdCache: make(map[protoreflect.MessageDescriptor][]protoreflect.ExtensionDescriptor),
	}

	return c.compile(md)
}

// compiler converts descriptors into [tdp.Type]s.
type compiler struct {
	Options

	buf        []byte
	totalTypes int

	// Maps used for linking. Symbols maps arbitrary keys to an offset in buf
	// and relos maps offsets to pointer values that should be filled in with
	// the final pointer value for that symbol.
	symbols map[any]int
	relos   map[int]relo

	layouts map[protoreflect.MessageDescriptor]tdp.TypeLayout
	fdCache map[protoreflect.MessageDescriptor][]protoreflect.FieldDescriptor
}

func (c *compiler) compile(md protoreflect.MessageDescriptor) *tdp.Type {
	c.message(md)

	if len(c.buf) > math.MaxInt32 {
		panic(fmt.Errorf("tdp: type has too many dependencies: %s", md.FullName()))
	}

	auxes := make([]tdp.Aux, c.totalTypes)

	// Copy buf onto some memory that the GC can trace through md to keep all of
	// the descriptors alive.
	p := arena.AllocTraceable(len(c.buf), unsafe.Pointer(unsafe.SliceData(auxes)))
	copy(unsafe2.Slice(p, len(c.buf)), c.buf)

	// Resolve all relocations.
	c.link(p)

	// Resolve all message type references. This needs to be done as a separate
	// step due to potential cycles.
	lib := &tdp.Library{
		Base:  unsafe2.Cast[tdp.Type](p),
		Types: make(map[protoreflect.MessageDescriptor]*tdp.Type),
	}
	var i int
	for symbol, offset := range c.symbols {
		sym, ok := symbol.(typeSymbol)
		if !ok {
			continue
		}

		ty := lib.AtOffset(uint32(offset))
		ty.Aux = &auxes[i]
		i++

		ty.Library = lib
		ty.Descriptor = sym.ty
		ty.FieldDescriptors = c.fdCache[sym.ty]

		c.Backend.PopulateMethods(&ty.Methods)

		lib.Types[sym.ty] = ty

		if debug.Enabled {
			*ty.Layout.Get() = c.layouts[sym.ty]
		}
	}

	if debug.Enabled {
		runtime.SetFinalizer(lib.Base, func(t *tdp.Type) {
			c.log("finalizer", "%p:%s", t, t.Descriptor.FullName())
		})
	}

	return lib.Base
}

// profile returns profiling information for fd in the compiler's current
// context.
func (c *compiler) profile(fd protoreflect.FieldDescriptor) FieldProfile {
	site := FieldSite{Field: fd}
	if c.Profile == nil {
		return site.DefaultProfile()
	}
	return c.Profile.Field(site)
}

func (c *compiler) fields(md protoreflect.MessageDescriptor) []protoreflect.FieldDescriptor {
	fields, ok := c.fdCache[md]
	if ok {
		return fields
	}

	fds := md.Fields()
	fields = make([]protoreflect.FieldDescriptor, fds.Len())
	for i := range fields {
		fields[i] = fds.Get(i)
	}

	if c.Extensions != nil {
		fields = append(fields, c.Extensions.FindExtensionsByMessage(md.FullName())...)
		// Ensure determinism by sorting the extension fields by number. The
		// implementation of GlobalTypes.RangeExtensions uses a map and so likes
		// to have a per-process random order.
		//
		// We don't actually lose anything from not sorting, but it makes tests
		// deterministic.
		slices.SortFunc(fields[fds.Len():], func(a, b protoreflect.FieldDescriptor) int {
			return cmp.Compare(a.Number(), b.Number())
		})
	}

	c.fdCache[md] = fields
	return fields
}

// analyze generates an intermediate representation for a given message,
// performing the necessary layout and scheduling analysis for its parser(s).
func (c *compiler) analyze(md protoreflect.MessageDescriptor) *ir {
	ir := &ir{d: md}

	// Classify all of the fields into archetypes.
	for _, fd := range c.fields(md) {
		prof := c.profile(fd)

		hot := prof.DecodeProbability >= 0.5
		arch := c.Backend.SelectArchetype(fd, prof)

		if arch.Bits > 0 && arch.Oneof {
			panic(fmt.Sprintf("oneof archetype for %v requested bits; this is a bug", fd.FullName()))
		}

		tIdx := len(ir.t)
		ir.t = append(ir.t, tField{
			d:    fd,
			prof: prof,
			arch: arch,
		})

		for j := range arch.Parsers {
			ir.p = append(ir.p, pField{
				tIdx: tIdx,
				aIdx: j,
				hot:  j == 0 && hot,
			})
		}

		// Protoc will always place oneof members contiguously in the fields
		// array of a message. This means that if this is not the first member
		// of a oneof, the most recent value in ir.s will be the current oneof's
		// struct slot.
		if arch.Oneof &&
			fd.ContainingOneof().Fields().Get(0).Index() != fd.Index() {
			last := &ir.s[len(ir.s)-1]
			last.tIdx = append(last.tIdx, tIdx)
		} else {
			ir.s = append(ir.s, sField{
				tIdx: []int{tIdx},
			})
		}
	}

	// Next, lay out the struct by sorting the struct members by alignment.
	var bits, whichWords int
	for i := range ir.s {
		sf := &ir.s[i]
		var temp stats.Mean
		for _, j := range sf.tIdx {
			arch := ir.t[j].arch
			sf.layout = sf.layout.Max(arch.Layout)
			sf.bits = max(sf.bits, arch.Bits)

			temp.Record(ir.t[j].prof.DecodeProbability)
		}

		bits += int(sf.bits)
		sf.hot = temp.Get() >= 0

		if ir.t[sf.tIdx[0]].arch.Oneof {
			whichWords++
		}
	}

	// Append hidden zero-size fields at the end to ensure that the stride of
	// this type is divisible by 8.
	ir.s = append(ir.s, sField{layout: layout.Of[[0]uint64](), hot: true})
	ir.s = append(ir.s, sField{layout: layout.Of[[0]uint64](), hot: false})

	slices.SortStableFunc(ir.s, func(a, b sField) int {
		// Sort hot fields before cold fields. This simplifies the code below.
		switch {
		case a.hot == b.hot:
			return -cmp.Compare(a.layout.Align, b.layout.Align)
		case a.hot:
			return -1
		default:
			return 1
		}
	})

	// Figure out the number of bit words we need. We use 32-bit words.
	const bitsPerWord = 32
	bitWords := (bits + bitsPerWord - 1) / bitsPerWord // Divide and round up.
	ir.layout.BitWords = bitWords + whichWords

	ir.hot = layout.Size[dynamic.Message]()
	ir.hot += (bitWords + whichWords) * 4

	ir.cold = layout.Size[dynamic.Cold]()

	var nextBit uint32
	nextWhichWord := uint32(ir.hot - whichWords*4)
	for i := range ir.s {
		sf := &ir.s[i]
		if sf.layout.Align == 0 {
			continue
		}

		// Allocate bit and byte storage for this field.
		size := &ir.hot
		if !sf.hot {
			size = &ir.cold
		}

		_, up := unsafe2.Addr[byte](*size).Misalign(sf.layout.Align)
		*size += up
		if debug.Enabled && up > 0 {
			// Note alignment padding required for the previous field.
			if i == 0 && sf.hot {
				ir.layout.BitWords += up / 4
			} else if ir.s[i-1].hot == sf.hot {
				f := ir.layout.Fields
				f[len(f)-1].Padding = uint32(up)
			}
		}

		sf.offset.Data = int32(*size)
		if !sf.hot {
			sf.offset.Data = ^sf.offset.Data
		}
		*size += sf.layout.Size

		if sf.bits > 0 {
			sf.offset.Bit = nextBit
			nextBit += sf.bits
		}

		oneof := sf.tIdx != nil && ir.t[sf.tIdx[0]].arch.Oneof
		if oneof {
			sf.offset.Bit = nextWhichWord
			nextWhichWord += 4
		}

		// Copy the offset information into each field that uses this struct
		// slot.
		for _, j := range sf.tIdx {
			ir.t[j].offset = sf.offset
			if oneof {
				ir.t[j].offset.Number = uint32(ir.t[j].d.Number())
			}
		}

		if debug.Enabled && sf.tIdx != nil {
			index := sf.tIdx[0]
			if ir.t[index].arch.Oneof {
				index = ^ir.t[index].d.ContainingOneof().Index()
			}

			ir.layout.Fields = append(ir.layout.Fields, tdp.FieldLayout{
				Size:   uint32(sf.layout.Size),
				Align:  uint32(sf.layout.Align),
				Bits:   sf.bits,
				Index:  index,
				Offset: sf.offset,
			})
		}
	}

	if ir.hot > math.MaxInt32 {
		panic(fmt.Errorf("fastpb: message struct for %v too large (%d bytes, max is %d)", md.FullName(), ir.hot, math.MaxInt32))
	}
	if ir.cold > math.MaxInt32 {
		panic(fmt.Errorf("fastpb: message struct for %v too large (%d bytes, max is %d)", md.FullName(), ir.cold, math.MaxInt32))
	}

	if debug.Enabled {
		// Print the resulting layout for this struct.
		c.log("layout", "%s, %d/%d\n%v", ir.d.FullName(), ir.hot, ir.cold,
			debug.Formatter(func(buf fmt.State) {
				start := layout.Size[dynamic.Message]()
				fmt.Fprintf(buf, "  %#04x(-)[%d:4:0] [%d]uint32\n", start, 4*ir.layout.BitWords, ir.layout.BitWords)
				for _, sf := range ir.s {
					if sf.tIdx == nil {
						continue
					}

					tf := ir.t[sf.tIdx[0]]
					name := tf.d.Name()
					if tf.arch.Oneof {
						name = "oneof:" + tf.d.ContainingOneof().Name()
					}

					fmt.Fprintf(buf, "  %#04x", sf.offset.Data)
					if sf.bits > 0 {
						fmt.Fprintf(buf, "(%v)", sf.offset.Bit)
					} else {
						fmt.Fprint(buf, "(-)")
					}
					fmt.Fprintf(buf, "[%d:%d:%d]", sf.layout.Size, sf.layout.Align, sf.bits)

					fmt.Fprintf(buf, " %s: ", name)
					switch tf.d.Cardinality() {
					case protoreflect.Optional:
						if tf.d.HasOptionalKeyword() {
							fmt.Fprint(buf, "optional ")
						}
					case protoreflect.Repeated:
						fmt.Fprint(buf, "repeated ")
					case protoreflect.Required:
						fmt.Fprint(buf, "required ")
					}
					if m := tf.d.Message(); m != nil {
						fmt.Fprintf(buf, "%v (%v) ", m.FullName(), tf.d.Kind())
					} else if e := tf.d.Enum(); e != nil {
						fmt.Fprintf(buf, "%v (%v) ", e.FullName(), tf.d.Kind())
					} else {
						fmt.Fprintf(buf, "%v ", tf.d.Kind())
					}
					fmt.Fprintln(buf)
				}
			}))
	}

	// Now, sort the parsers into the hot and cold sides. Stable sort is
	// particularly important here!
	slices.SortStableFunc(ir.p, func(a, b pField) int {
		var aCold, bCold int
		if !a.hot {
			aCold = 1
		}
		if !b.hot {
			bCold = 1
		}
		return cmp.Compare(aCold, bCold)
	})

	// Now, lay out control flow between parsers. Each parser points to the
	// first one after it that refers to a different field or oneof, except
	// for cold parsers, which always point to a hot parser.
	//
	// For this purpose, we build a table of the index of the first hot parser
	// for each field/oneof. Oneof indices are entered as their complements.
	table := make(map[int]int, len(ir.t))
	idx := func(tIdx int) int {
		tf := ir.t[tIdx]
		if tf.arch.Oneof {
			return ^tf.d.ContainingOneof().Index()
		}
		return tf.d.Index()
	}

	for i, pf := range ir.p {
		if !pf.hot {
			continue
		}

		j := idx(pf.tIdx)
		if _, ok := table[j]; !ok {
			table[j] = i
		}
	}

	for i := range ir.p {
		pf := &ir.p[i]

		p := ir.t[pf.tIdx].arch.Parsers[pf.aIdx]
		if p.Retry {
			pf.next = i
			continue
		}

		orig := idx(pf.tIdx)
	loop:
		for tIdx := pf.tIdx; tIdx < len(ir.t); tIdx++ {
			i := idx(tIdx)
			j, ok := table[i]
			if !ok {
				continue
			}

			// j is the index of *some* hot parser. This may be for the same
			// field/oneof as the current index, so we need to keep incrementing
			// it until it either:
			//
			// 1. Points to a cold parser, and hence it should just wrap around
			//    to the first parser in the stream.
			//
			// 2. We hit a parser for a different field/oneof.
			for ; ; j++ {
				if j == len(ir.p) {
					break loop // Wraparound.
				}
				next := ir.p[j]
				if !next.hot {
					break loop // We reached the cold section.
				}

				if idx(next.tIdx) != orig {
					pf.next = j
					break loop
				}
			}
		}
	}

	if debug.Enabled {
		// Print the parser CFG.
		c.log("cfg", "%s\n%v", ir.d.FullName(), debug.Formatter(func(buf fmt.State) {
			for i, pf := range ir.p {
				tf := ir.t[pf.tIdx]
				fmt.Fprintf(buf, "  #%d: %v#%d -> #%d\n", i, tf.d.Name(), pf.aIdx, pf.next)
			}
		}))
	}

	return ir
}

// codegen code-generates the analyzed contents of an intermediate
// representation.
func (c *compiler) codegen(ir *ir) {
	tSym := typeSymbol{ir.d}
	pSym := parserSymbol{ir.d, false}
	mSym := parserSymbol{ir.d, true}

	c.write(tSym,
		tdp.Type{
			Size:     uint32(ir.hot),
			ColdSize: uint32(ir.cold),
			Count:    uint32(len(ir.t)),
		},
		relo{
			symbol: pSym,
			offset: unsafe.Offsetof(tdp.Type{}.Parser),
		},
		relo{
			symbol: tableSymbol{tSym},
			offset: unsafe.Offsetof(tdp.Type{}.Numbers),
		},
	)

	numbers := make([]swiss.Entry[int32, uint32], 0, len(ir.t))
	for i, tf := range ir.t {
		var relos []relo
		if md := fieldMessage(tf.d); md != nil {
			relos = []relo{{
				symbol: typeSymbol{md},
				offset: unsafe.Offsetof(tdp.Field{}.Message),
			}}
		}

		// Append whatever field data we can before doing layout.
		c.write(nil,
			tdp.Field{
				Accessor: tdp.Accessor{
					Offset: tf.offset,
					Getter: tf.arch.Getter.adapt(),
				},
			},
			relos...,
		)

		numbers = append(numbers, swiss.KV(int32(tf.d.Number()), uint32(i)))
	}
	// Append the dummy end field.
	c.write(nil, tdp.Field{})

	// Append the field number table.
	writeTable(c, tableSymbol{tSym}, numbers)

	c.write(pSym,
		tdp.TypeParser{},
		relo{
			symbol:   tSym,
			offset:   unsafe.Offsetof(tdp.TypeParser{}.TypeOffset),
			relative: true,
		},
		relo{
			symbol: tableSymbol{pSym},
			offset: unsafe.Offsetof(tdp.TypeParser{}.Tags),
		},
		relo{
			symbol: mSym,
			offset: unsafe.Offsetof(tdp.TypeParser{}.MapEntry),
		},
		relo{
			symbol: fieldParserSymbol{parser: pSym, index: 0},
			offset: unsafe.Offsetof(tdp.TypeParser{}.Entrypoint) +
				unsafe.Offsetof(tdp.FieldParser{}.NextOk),
		},
	)

	numbers = numbers[:0]
	// Lay out the parser table.
	for i, pf := range ir.p {
		tf := ir.t[pf.tIdx]
		p := tf.arch.Parsers[pf.aIdx]

		tag := tdp.EncodeTag(tf.d.Number(), p.Kind)

		numbers = append(numbers, swiss.KV(
			int32(protowire.EncodeTag(tf.d.Number(), p.Kind)),
			uint32(i),
		))

		nextOk := pf.next
		nextErr := i + 1
		if nextErr == len(ir.p) {
			nextErr = 0
		}

		relos := []relo{
			{
				symbol: fieldParserSymbol{parser: pSym, index: nextOk},
				offset: unsafe.Offsetof(tdp.FieldParser{}.NextOk),
			},
			{
				symbol: fieldParserSymbol{parser: pSym, index: nextErr},
				offset: unsafe.Offsetof(tdp.FieldParser{}.NextErr),
			},
		}
		if md := fieldMessage(tf.d); md != nil {
			relos = append(relos, relo{
				symbol: parserSymbol{md, false},
				offset: unsafe.Offsetof(tdp.FieldParser{}.Message),
			})
		}

		c.write(
			fieldParserSymbol{parser: pSym, index: i},
			tdp.FieldParser{
				Tag:    tag,
				Offset: tf.offset,
				Parse:  uintptr(unsafe2.NewPC(p.Thunk)),
			},
			relos...,
		)
	}

	// Ensure that there is at least one parser to be the entry-point.
	if len(ir.p) == 0 {
		c.write(
			fieldParserSymbol{parser: pSym, index: 0},
			tdp.FieldParser{
				Tag: ^tdp.Tag(0), // This will never be matched.
			},
			relo{
				symbol: fieldParserSymbol{parser: pSym, index: 0},
				offset: unsafe.Offsetof(tdp.FieldParser{}.NextOk),
			},
			relo{
				symbol: fieldParserSymbol{parser: pSym, index: 0},
				offset: unsafe.Offsetof(tdp.FieldParser{}.NextErr),
			},
		)
	}

	// Append the parser's field number table.
	writeTable(c, tableSymbol{pSym}, numbers)

	// Write the map entry parser.
	c.write(mSym,
		tdp.TypeParser{
			DiscardUnknown: true,
		},
		relo{
			symbol:   tSym,
			offset:   unsafe.Offsetof(tdp.TypeParser{}.TypeOffset),
			relative: true,
		},
		relo{
			symbol: tableSymbol{mSym},
			offset: unsafe.Offsetof(tdp.TypeParser{}.Tags),
		},
		relo{
			symbol: fieldParserSymbol{parser: mSym, index: 0},
			offset: unsafe.Offsetof(tdp.TypeParser{}.Entrypoint) +
				unsafe.Offsetof(tdp.FieldParser{}.NextOk),
		},
	)
	const mapValue = 0x2<<3 | tdp.Tag(protowire.BytesType) // Field number 2 with bytes type (so, 0b10010).
	c.write(
		fieldParserSymbol{parser: mSym, index: 0},
		tdp.FieldParser{
			Tag:   mapValue,
			Parse: uintptr(unsafe2.NewPC(vm.Thunk(vm.P1.ParseMapEntry))),
		},
		relo{
			symbol: fieldParserSymbol{parser: mSym, index: 0},
			offset: unsafe.Offsetof(tdp.FieldParser{}.NextOk),
		},
		relo{
			symbol: fieldParserSymbol{parser: mSym, index: 0},
			offset: unsafe.Offsetof(tdp.FieldParser{}.NextErr),
		},
		relo{
			symbol: pSym,
			offset: unsafe.Offsetof(tdp.FieldParser{}.Message),
		},
	)
	writeTable(c, tableSymbol{mSym}, []swiss.Entry[int32, uint32]{{Key: int32(mapValue), Value: 0}})
}

func (c *compiler) message(md protoreflect.MessageDescriptor) {
	if _, ok := c.symbols[typeSymbol{md}]; ok {
		return
	}
	c.totalTypes++

	c.log("message", "%s", md.FullName())
	ir := c.analyze(md)
	c.codegen(ir)
	c.layouts[ir.d] = ir.layout

	fields := md.Fields()
	for i := range fields.Len() {
		if m := fieldMessage(fields.Get(i)); m != nil {
			c.message(m)
		}
	}
}

func fieldMessage(fd protoreflect.FieldDescriptor) protoreflect.MessageDescriptor {
	if fd.IsMap() {
		return fd.MapValue().Message()
	}
	return fd.Message()
}

// relo is a relocation that is resolved in [compiler.link].
type relo struct {
	symbol   any
	offset   uintptr
	relative bool // If true, the written value is relative to the base address.
}

func (c *compiler) link(base *byte) {
	for target, relo := range c.relos {
		offset, ok := c.symbols[relo.symbol]
		if !ok {
			panic(fmt.Sprintf("fastpb: undefined symbol: %v", relo.symbol))
		}

		if relo.relative {
			c.log("relo", "%#v %#x->%#x", relo.symbol, target, uint32(offset))
			unsafe2.ByteStore(base, target, uint32(offset))
		} else {
			value := unsafe2.Add(base, offset)
			c.log("relo", "%#v %#x->%#x", relo.symbol, target, value)
			unsafe2.ByteStore(base, target, value)
		}
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

func writeTable[V comparable](c *compiler, symbol any, entries []swiss.Entry[int32, V]) int {
	return c.writeFunc(symbol, func(b []byte) (int, []byte) {
		b, t := swiss.New(b, nil, entries...)
		return unsafe2.Sub(unsafe2.Cast[byte](t), unsafe.SliceData(b)), b
	})
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
		c.relos[offset] = relo
	}

	return offset
}

func (c *compiler) log(op, format string, args ...any) {
	debug.Log([]any{"%p", c}, op, format, args...)
}
