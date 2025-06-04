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
	"cmp"
	"fmt"
	"iter"
	"math"
	"reflect"
	"runtime"
	"slices"
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/swiss"
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
		relos:   make(map[int]relo),

		layouts: make(map[protoreflect.MessageDescriptor]typeLayout),
		fdCache: make(map[protoreflect.MessageDescriptor][]protoreflect.ExtensionDescriptor),
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
// The zero value of this type results in "default behavior", indicating that
// the profile does not have enough data to form an opinion.
type FieldProfile struct {
	Parse Temperature // How likely this field is to be seen on the wire.
}

// Temperature is a reading from 1.0 to -1.0 that indicates the "hotness" of
// some codepath, such as parsing or access.
//
// The expected probability is P = (n+1)/2. This is such that the zero value
// indicates "no opinion".
type Temperature float64

// PGO adds profile-guided optimization information to a compiler.
//
// Profile is a function that returns profiling information for a given field.
func PGO(prof func(site FieldSite) FieldProfile) CompileOption {
	return func(c *compiler) { c.prof = prof }
}

// Extensions provides an extension resolver for a compiler.
//
// Unlike ordinary Protobuf parsers, fastpb does not perform extension
// resolution on the fly. Instead, any extensions that should be parsed must
// be provided up-front.
func Extensions(resolver func(protoreflect.MessageDescriptor) iter.Seq[protoreflect.ExtensionDescriptor]) CompileOption {
	return func(c *compiler) { c.extns = resolver }
}

// ExtensionsFromRegistry uses a type registry to provide extension information
// about a message type.
func ExtensionsFromRegistry(types *protoregistry.Types) CompileOption {
	return Extensions(func(md protoreflect.MessageDescriptor) iter.Seq[protoreflect.ExtensionDescriptor] {
		return func(yield func(protoreflect.ExtensionDescriptor) bool) {
			types.RangeExtensionsByMessage(md.FullName(), func(ext protoreflect.ExtensionType) bool {
				return yield(ext.TypeDescriptor().Descriptor())
			})
		}
	})
}

// compiler is context for compiling a descriptor into a [Type].
type compiler struct {
	buf        []byte
	totalTypes int

	// Maps used for linking. Symbols maps arbitrary keys to an offset in buf
	// and relos maps offsets to pointer values that should be filled in with
	// the final pointer value for that symbol.
	symbols map[any]int
	relos   map[int]relo

	layouts map[protoreflect.MessageDescriptor]typeLayout

	prof  func(FieldSite) FieldProfile
	extns func(protoreflect.MessageDescriptor) iter.Seq[protoreflect.ExtensionDescriptor]

	fdCache map[protoreflect.MessageDescriptor][]protoreflect.FieldDescriptor
}

type typeSymbol struct {
	ty protoreflect.MessageDescriptor
}

type parserSymbol struct {
	ty       protoreflect.MessageDescriptor
	mapEntry bool
}

type tableSymbol struct{ sym any }

type fieldParserSymbol struct {
	parser any
	index  int
}

func (s typeSymbol) String() string {
	return fmt.Sprintf("typeSymbol:%s", s.ty.Name())
}

func (s parserSymbol) String() string {
	return fmt.Sprintf("parserSymbol:%s:%v", s.ty.Name(), s.mapEntry)
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
	lib := &Library{
		base:  unsafe2.Cast[typeHeader](p),
		types: make(map[protoreflect.MessageDescriptor]Type),
	}
	var i int
	for symbol, offset := range c.symbols {
		sym, ok := symbol.(typeSymbol)
		if !ok {
			continue
		}

		ty := Type{raw: unsafe2.ByteAdd(lib.base, offset)}
		ty.raw.aux = &auxes[i]
		i++

		ty.raw.aux.lib = lib
		ty.raw.aux.desc = sym.ty
		ty.raw.aux.methods.Unmarshal = unmarshalShim
		ty.raw.aux.methods.CheckInitialized = requiredShim
		ty.raw.aux.fds = c.fdCache[sym.ty]

		lib.types[sym.ty] = ty

		if dbg.Enabled {
			*ty.raw.aux.layout.Get() = c.layouts[sym.ty]
		}
	}

	if dbg.Enabled {
		runtime.SetFinalizer(lib.base, func(t *typeHeader) {
			c.log("finalizer", "%p:%s", t, t.aux.desc.FullName())
		})
	}

	return Type{raw: lib.base}
}

// ir is analysis information about a message type for generating a parser
// and a dynamic type for it.
type ir struct {
	d protoreflect.MessageDescriptor

	// Each Protobuf field has three associated pieces of data that can be
	// sorted in different orders. There is the field inside of a [Type],
	// the field's parsers (which there may be more than one of per tField),
	// and the field's struct offsets (which may be shared by t).
	t []tField
	p []pField
	s []sField

	hot, cold int
	layout    typeLayout
}

type tField struct {
	d      protoreflect.FieldDescriptor
	prof   FieldProfile
	arch   *archetype
	offset fieldOffset
}

type pField struct {
	tIdx int // Index in ir.t.
	aIdx int // Index in ir.t[tIdx].arch.parsers.

	hot  bool // If true, this parser should be in the "hot" part of the stream.
	next int  // The next parser to execute, as an index into ir.p.
}

type sField struct {
	tIdx []int // Index in ir.t. May be more than one!

	size, align, bits uint32
	offset            fieldOffset
	hot               bool
}

// profile returns profiling information for fd in the compiler's current
// context.
func (c *compiler) profile(fd protoreflect.FieldDescriptor) FieldProfile {
	if c.prof == nil {
		return FieldProfile{}
	}
	return c.prof(FieldSite{Field: fd})
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

	if c.extns != nil {
		fields = slices.AppendSeq(fields, c.extns(md))
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

		hot := prof.Parse >= 0
		if prof.Parse == 0 && fd.IsExtension() {
			// Extensions default to being cold.
			prof.Parse = -0.5
			hot = false
		}

		arch := selectArchetype(fd, prof)

		if arch.bits > 0 && arch.oneof {
			panic(fmt.Sprintf("oneof archetype for %v requested bits; this is a bug", fd.FullName()))
		}

		tIdx := len(ir.t)
		ir.t = append(ir.t, tField{
			d:    fd,
			prof: prof,
			arch: arch,
		})

		for j := range arch.parsers {
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
		if arch.oneof &&
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
		var temp Temperature
		for _, j := range sf.tIdx {
			arch := ir.t[j].arch
			sf.size = max(sf.size, arch.size)
			sf.align = max(sf.align, arch.align)
			sf.bits = max(sf.bits, arch.bits)

			temp += ir.t[j].prof.Parse
		}

		bits += int(sf.bits)
		temp /= Temperature(len(sf.tIdx))
		sf.hot = temp >= 0

		if ir.t[sf.tIdx[0]].arch.oneof {
			whichWords++
		}
	}

	// Append hidden zero-size fields at the end to ensure that the stride of
	// this type is divisible by 8.
	ir.s = append(ir.s, sField{align: uint32(unsafe2.Int64Align), hot: true})
	ir.s = append(ir.s, sField{align: uint32(unsafe2.Int64Align), hot: false})

	slices.SortStableFunc(ir.s, func(a, b sField) int {
		// Sort hot fields before cold fields. This simplifies the code below.
		switch {
		case a.hot == b.hot:
			return -cmp.Compare(a.align, b.align)
		case a.hot:
			return -1
		default:
			return 1
		}
	})

	// Figure out the number of bit words we need. We use 32-bit words.
	const bitsPerWord = 32
	bitWords := (bits + bitsPerWord - 1) / bitsPerWord // Divide and round up.
	ir.layout.bitWords = bitWords + whichWords

	ir.hot, _ = unsafe2.Layout[message]()
	ir.hot += (bitWords + whichWords) * 4

	ir.cold, _ = unsafe2.Layout[cold]()

	var nextBit uint32
	nextWhichWord := uint32(ir.hot - whichWords*4)
	for i := range ir.s {
		sf := &ir.s[i]
		if sf.align == 0 {
			continue
		}

		// Allocate bit and byte storage for this field.
		size := &ir.hot
		if !sf.hot {
			size = &ir.cold
		}

		_, up := unsafe2.Addr[byte](*size).Misalign(int(sf.align))
		*size += up
		if dbg.Enabled && up > 0 {
			// Note alignment padding required for the previous field.
			if i == 0 && sf.hot {
				ir.layout.bitWords += up / 4
			} else if ir.s[i-1].hot == sf.hot {
				f := ir.layout.fields
				f[len(f)-1].padding = uint32(up)
			}
		}

		sf.offset.data = int32(*size)
		if !sf.hot {
			sf.offset.data = ^sf.offset.data
		}
		*size += int(sf.size)

		if sf.bits > 0 {
			sf.offset.bit = nextBit
			nextBit += sf.bits
		}

		oneof := sf.tIdx != nil && ir.t[sf.tIdx[0]].arch.oneof
		if oneof {
			sf.offset.bit = nextWhichWord
			nextWhichWord += 4
		}

		// Copy the offset information into each field that uses this struct
		// slot.
		for _, j := range sf.tIdx {
			ir.t[j].offset = sf.offset
			if oneof {
				ir.t[j].offset.number = uint32(ir.t[j].d.Number())
			}
		}

		if dbg.Enabled && sf.tIdx != nil {
			index := sf.tIdx[0]
			if ir.t[index].arch.oneof {
				index = ^ir.t[index].d.ContainingOneof().Index()
			}

			ir.layout.fields = append(ir.layout.fields, fieldLayout{
				size:   sf.size,
				align:  sf.align,
				bits:   sf.bits,
				index:  index,
				offset: sf.offset,
			})
		}
	}

	if ir.hot > math.MaxInt32 {
		panic(fmt.Errorf("fastpb: message struct for %v too large (%d bytes, max is %d)", md.FullName(), ir.hot, math.MaxInt32))
	}
	if ir.cold > math.MaxInt32 {
		panic(fmt.Errorf("fastpb: message struct for %v too large (%d bytes, max is %d)", md.FullName(), ir.cold, math.MaxInt32))
	}

	if dbg.Enabled {
		// Print the resulting layout for this struct.
		c.log("layout", "%s, %d/%d\n%v", ir.d.FullName(), ir.hot, ir.cold,
			dbg.Formatter(func(buf fmt.State) {
				start, _ := unsafe2.Layout[message]()
				fmt.Fprintf(buf, "  %#04x(-)[%d:4:0] [%d]uint32\n", start, 4*ir.layout.bitWords, ir.layout.bitWords)
				for _, sf := range ir.s {
					if sf.tIdx == nil {
						continue
					}

					tf := ir.t[sf.tIdx[0]]
					name := tf.d.Name()
					if tf.arch.oneof {
						name = "oneof:" + tf.d.ContainingOneof().Name()
					}

					fmt.Fprintf(buf, "  %#04x", sf.offset.data)
					if sf.bits > 0 {
						fmt.Fprintf(buf, "(%v)", sf.offset.bit)
					} else {
						fmt.Fprint(buf, "(-)")
					}
					fmt.Fprintf(buf, "[%d:%d:%d]", sf.size, sf.align, sf.bits)

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
		if tf.arch.oneof {
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

		p := ir.t[pf.tIdx].arch.parsers[pf.aIdx]
		if p.retry {
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

	if dbg.Enabled {
		// Print the parser CFG.
		c.log("cfg", "%s\n%v", ir.d.FullName(), dbg.Formatter(func(buf fmt.State) {
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
		typeHeader{
			size:     uint32(ir.hot),
			coldSize: uint32(ir.cold),
			count:    uint32(len(ir.t)),
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

	numbers := make([]swiss.Entry[int32, uint32], 0, len(ir.t))
	for i, tf := range ir.t {
		var relos []relo
		if md := fieldMessage(tf.d); md != nil {
			relos = []relo{{
				symbol: typeSymbol{md},
				offset: unsafe.Offsetof(field{}.message),
			}}
		}

		// Append whatever field data we can before doing layout.
		c.write(nil,
			field{
				getter: getter{
					offset: tf.offset,
					thunk:  tf.arch.getter,
				},
			},
			relos...,
		)

		numbers = append(numbers, swiss.KV(int32(tf.d.Number()), uint32(i)))
	}
	// Append the dummy end field.
	c.write(nil, field{})

	// Append the field number table.
	writeTable(c, tableSymbol{tSym}, numbers)

	c.write(pSym,
		typeParser{},
		relo{
			symbol:   tSym,
			offset:   unsafe.Offsetof(typeParser{}.tyOffset),
			relative: true,
		},
		relo{
			symbol: tableSymbol{pSym},
			offset: unsafe.Offsetof(typeParser{}.tags),
		},
		relo{
			symbol: mSym,
			offset: unsafe.Offsetof(typeParser{}.mapEntry),
		},
		relo{
			symbol: fieldParserSymbol{parser: pSym, index: 0},
			offset: unsafe.Offsetof(typeParser{}.entry) +
				unsafe.Offsetof(fieldParser{}.nextOk),
		},
	)

	numbers = numbers[:0]
	// Lay out the parser table.
	for i, pf := range ir.p {
		tf := ir.t[pf.tIdx]
		p := tf.arch.parsers[pf.aIdx]

		var tag fieldTag
		tag.encode(tf.d.Number(), p.kind)

		numbers = append(numbers, swiss.KV(
			int32(protowire.EncodeTag(tf.d.Number(), p.kind)),
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
				offset: unsafe.Offsetof(fieldParser{}.nextOk),
			},
			{
				symbol: fieldParserSymbol{parser: pSym, index: nextErr},
				offset: unsafe.Offsetof(fieldParser{}.nextErr),
			},
		}
		if md := fieldMessage(tf.d); md != nil {
			relos = append(relos, relo{
				symbol: parserSymbol{md, false},
				offset: unsafe.Offsetof(fieldParser{}.message),
			})
		}

		c.write(
			fieldParserSymbol{parser: pSym, index: i},
			fieldParser{
				tag:    tag,
				offset: tf.offset,
				thunk:  unsafe2.NewPC(p.parser),
			},
			relos...,
		)
	}

	// Ensure that there is at least one parser to be the entry-point.
	if len(ir.p) == 0 {
		c.write(
			fieldParserSymbol{parser: pSym, index: 0},
			fieldParser{
				tag: ^fieldTag(0), // This will never be matched.
			},
			relo{
				symbol: fieldParserSymbol{parser: pSym, index: 0},
				offset: unsafe.Offsetof(fieldParser{}.nextOk),
			},
			relo{
				symbol: fieldParserSymbol{parser: pSym, index: 0},
				offset: unsafe.Offsetof(fieldParser{}.nextErr),
			},
		)
	}

	// Append the parser's field number table.
	writeTable(c, tableSymbol{pSym}, numbers)

	// Write the map entry parser.
	c.write(mSym,
		typeParser{
			discardUnknown: true,
		},
		relo{
			symbol:   tSym,
			offset:   unsafe.Offsetof(typeParser{}.tyOffset),
			relative: true,
		},
		relo{
			symbol: tableSymbol{mSym},
			offset: unsafe.Offsetof(typeParser{}.tags),
		},
		relo{
			symbol: fieldParserSymbol{parser: mSym, index: 0},
			offset: unsafe.Offsetof(typeParser{}.entry) +
				unsafe.Offsetof(fieldParser{}.nextOk),
		},
	)
	const mapValue = 0x2<<3 | fieldTag(protowire.BytesType) // Field number 2 with bytes type (so, 0b10010).
	c.write(
		fieldParserSymbol{parser: mSym, index: 0},
		fieldParser{
			tag:   mapValue,
			thunk: unsafe2.NewPC(parserThunk(parseMapEntry)),
		},
		relo{
			symbol: fieldParserSymbol{parser: mSym, index: 0},
			offset: unsafe.Offsetof(fieldParser{}.nextOk),
		},
		relo{
			symbol: fieldParserSymbol{parser: mSym, index: 0},
			offset: unsafe.Offsetof(fieldParser{}.nextErr),
		},
		relo{
			symbol: pSym,
			offset: unsafe.Offsetof(fieldParser{}.message),
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
	dbg.Log([]any{"%p", c}, op, format, args...)
}
