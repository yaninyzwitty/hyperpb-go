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
	"iter"
	"math"
	"reflect"
	"runtime"
	"slices"
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/bufbuild/hyperpb/internal/arena"
	"github.com/bufbuild/hyperpb/internal/debug"
	"github.com/bufbuild/hyperpb/internal/scc"
	"github.com/bufbuild/hyperpb/internal/swiss"
	"github.com/bufbuild/hyperpb/internal/tdp"
	"github.com/bufbuild/hyperpb/internal/tdp/vm"
	"github.com/bufbuild/hyperpb/internal/unsafe2"
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
		root:    md,

		types:   make(map[protoreflect.MessageDescriptor]*ir),
		sccInfo: make(map[*scc.Component[*ir]]*sccInfo),

		symbols: make(map[any]int),
		relos:   make(map[int]relo),

		fdCache: make(map[protoreflect.MessageDescriptor][]protoreflect.ExtensionDescriptor),
	}

	return c.compile(md)
}

// compiler converts descriptors into [tdp.Type]s.
type compiler struct {
	Options
	root  protoreflect.MessageDescriptor
	types map[protoreflect.MessageDescriptor]*ir

	buf []byte

	dag     *scc.DAG[*ir]
	sccInfo map[*scc.Component[*ir]]*sccInfo

	// Maps used for linking. Symbols maps arbitrary keys to an offset in buf
	// and relos maps offsets to pointer values that should be filled in with
	// the final pointer value for that symbol.
	symbols map[any]int
	relos   map[int]relo

	fdCache map[protoreflect.MessageDescriptor][]protoreflect.FieldDescriptor
}

func (c *compiler) compile(md protoreflect.MessageDescriptor) *tdp.Type {
	c.recurse(md)
	c.dag = scc.Sort(c.types[md], func(ty *ir) iter.Seq[*ir] {
		return func(yield func(*ir) bool) {
			for _, t := range ty.t {
				md := fieldMessage(t.d)
				if md != nil && !yield(c.types[md]) {
					return
				}
			}
		}
	})

	for cycle := range c.dag.Topological() {
		c.sccInfo[cycle] = newSCCInfo(c, cycle)

		for _, ir := range cycle.Members() {
			ir.doLayout(c)
			ir.doSchedule(c)
			c.codegen(ir)
		}
	}

	c.log("bytes", "%d", len(c.buf))
	if len(c.buf) > math.MaxInt32 {
		panic(fmt.Errorf("hyperpb: type has too many dependencies: %s", md.FullName()))
	}

	auxes := make([]tdp.Aux, len(c.types))

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
	requiredSet := make(map[int32]struct{})
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

		// Find which fields are required or contain required fields.
		for _, fd := range ty.FieldDescriptors {
			if fd.IsExtension() {
				// Extensions cannot be required. Once we see one extension
				// we're all done.
				break
			}

			if fd.Cardinality() == protoreflect.Required {
				requiredSet[int32(fd.Index())] = struct{}{}
			}

			m := fieldMessage(fd)
			if m != nil && c.sccInfo[c.dag.ForNode(c.types[m])].hasRequired {
				requiredSet[^int32(fd.Index())] = struct{}{}
			}
		}
		for i := range requiredSet {
			ty.Required = append(ty.Required, i)
		}
		slices.Sort(ty.Required)
		slices.Reverse(ty.Required)
		clear(requiredSet)

		lib.Types[sym.ty] = ty

		if debug.Enabled {
			*ty.Layout.Get() = c.types[sym.ty].layout
		}
	}

	if debug.Enabled {
		runtime.SetFinalizer(lib.Base, func(t *tdp.Type) {
			c.log("finalizer", "%p:%s", t, t.Descriptor.FullName())
		})
	}

	entry := lib.Types[md]
	c.log("done", "%v", entry)
	return entry
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

// recurse calls analyze recursively.
func (c *compiler) recurse(md protoreflect.MessageDescriptor) {
	if c.types[md] != nil {
		return
	}

	c.log("message", "%s", md.FullName())
	ir := newIR(c, md)
	c.types[md] = ir
	for _, t := range ir.t {
		if m := fieldMessage(t.d); m != nil {
			c.recurse(m)
		}
	}
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

	offset := c.write(pSym,
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

	// Write the fast-lookup lut.
	writeLUT(c, offset, numbers)

	// Append the parser's field number table.
	writeTable(c, tableSymbol{pSym}, numbers)

	offset = c.write(mSym,
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

	// Write the map entry parser.
	const mapValue = 0x2<<3 | tdp.Tag(protowire.BytesType) // Field number 2 with bytes type (so, 0b10010).
	numbers = []swiss.Entry[int32, uint32]{{Key: int32(mapValue), Value: 0}}
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

	// Write the fast-lookup lut.
	writeLUT(c, offset, numbers)

	// Append the parser's field number table.
	writeTable(c, tableSymbol{mSym}, numbers)
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
			panic(fmt.Sprintf("hyperpb: undefined symbol while linking %v: %v", c.root.FullName(), relo.symbol))
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

func writeLUT(c *compiler, offset int, entries []swiss.Entry[int32, uint32]) {
	offset += int(unsafe.Offsetof(tdp.TypeParser{}.TagLUT))
	lut := (*[128]uint8)(c.buf[offset : offset+128])

	for i := range lut {
		lut[i] = 0xff
	}

	for _, e := range entries {
		if e.Key >= 0 && e.Key < 128 && e.Value <= 0xff {
			c.log("lut write", "%#x->%#x", e.Key, e.Value)
			lut[byte(e.Key)] = uint8(e.Value)
		}
	}

	c.log("lut", "%x", lut)
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
			panic(fmt.Sprintf("hyperpb: symbol %#v defined twice: %#x, %#x", symbol, old, offset))
		}
		c.symbols[symbol] = offset
	}

	for _, relo := range relos {
		offset := int(relo.offset) + offset
		if _, ok := c.relos[offset]; ok {
			panic(fmt.Sprintf("hyperpb: two relocations for the same offset %#x", offset))
		}
		c.relos[offset] = relo
	}

	return offset
}

func (c *compiler) log(op, format string, args ...any) {
	debug.Log([]any{"%p", c}, op, format, args...)
}
