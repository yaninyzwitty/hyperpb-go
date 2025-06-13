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

package fastpb

import (
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/tdp"
	"github.com/bufbuild/fastpb/internal/tdp/dynamic"
	"github.com/bufbuild/fastpb/internal/tdp/vm"
	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
	"github.com/bufbuild/fastpb/internal/zc"
)

// Oneofs are implemented as an actual union. The tag (or "which word") lives
// after the hasbits array in the message; each oneof's field offset has the
// offset for its which word baked into it. Only oneofs with at least two
// variants are turned into a tagged union; otherwise they use an optional
// field archetype.
//
// A oneof variant is active when its which word contains the oneof's number.
// Note that we never give out a pointer to a oneof's union storage, which
// avoids potential temporal memory safety issues with concurrent mutation.
//
// Aliasing a region between integer and pointer memory is safe because we only
// ever place arena pointers into that memory, which do not need to be scanned
// by the garbage collector.
//
// Bool-valued oneof members are stored as a uint8, not a bool or an element of
// a bitset or anything like that.

var oneofFields = map[protoreflect.Kind]*archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		layout:  layout.Of[int32](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[int32]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofVarint32}},
	},
	protoreflect.Uint32Kind: {
		layout:  layout.Of[uint32](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[uint32]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofVarint32}},
	},
	protoreflect.Sint32Kind: {
		layout:  layout.Of[int32](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[int32]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofZigZag32}},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		layout:  layout.Of[int64](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[int64]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofVarint64}},
	},
	protoreflect.Uint64Kind: {
		layout:  layout.Of[uint64](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[uint64]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofVarint64}},
	},
	protoreflect.Sint64Kind: {
		layout:  layout.Of[int64](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[int64]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofZigZag64}},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		layout:  layout.Of[uint32](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[uint32]),
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOneofFixed32}},
	},
	protoreflect.Sfixed32Kind: {
		layout:  layout.Of[int32](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[int32]),
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOneofFixed32}},
	},
	protoreflect.FloatKind: {
		layout:  layout.Of[float32](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[float32]),
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOneofFixed32}},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		layout:  layout.Of[uint64](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[uint64]),
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOneofFixed64}},
	},
	protoreflect.Sfixed64Kind: {
		layout:  layout.Of[int64](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[int64]),
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOneofFixed64}},
	},
	protoreflect.DoubleKind: {
		layout:  layout.Of[float64](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[float64]),
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOneofFixed64}},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		layout:  layout.Of[byte](),
		oneof:   true,
		getter:  adaptGetter(getOneofBool),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofBool}},
	},
	protoreflect.EnumKind: {
		layout:  layout.Of[protoreflect.EnumNumber](),
		oneof:   true,
		getter:  adaptGetter(getOneofScalar[protoreflect.EnumNumber]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		layout:  layout.Of[zc.Range](),
		oneof:   true,
		getter:  adaptGetter(getOneofString),
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOneofString}},
	},
	proto2StringKind: {
		layout:  layout.Of[zc.Range](),
		oneof:   true,
		getter:  adaptGetter(getOneofString),
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOneofBytes}},
	},
	protoreflect.BytesKind: {
		layout:  layout.Of[zc.Range](),
		oneof:   true,
		getter:  adaptGetter(getOneofBytes),
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOneofBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		// A singular message is laid out as a single *message pointer.
		layout: layout.Of[*dynamic.Message](),
		oneof:  true,
		getter: adaptGetter(getOneofMessage),
		// This message parser is eager. TODO: add a lazy message archetype.
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOneofMessage}},
	},
	protoreflect.GroupKind: {
		// Not implemented.
	},
}

func getOneofScalar[T tdp.Scalar](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	which := unsafe2.ByteLoad[uint32](m, getter.Offset.Bit)
	if which != getter.Offset.Number {
		return protoreflect.ValueOf(nil)
	}
	v := *dynamic.GetField[T](m, getter.Offset)
	return protoreflect.ValueOf(v)
}

func getOneofBool(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	which := unsafe2.ByteLoad[uint32](m, getter.Offset.Bit)
	if which != getter.Offset.Number {
		return protoreflect.ValueOf(nil)
	}
	v := *dynamic.GetField[byte](m, getter.Offset)
	return protoreflect.ValueOf(v != 0)
}

func getOneofString(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	which := unsafe2.ByteLoad[uint32](m, getter.Offset.Bit)
	if which != getter.Offset.Number {
		return protoreflect.ValueOf(nil)
	}
	r := *dynamic.GetField[zc.Range](m, getter.Offset)
	return protoreflect.ValueOf(r.String(m.Shared.Src))
}

func getOneofBytes(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	which := unsafe2.ByteLoad[uint32](m, getter.Offset.Bit)
	if which != getter.Offset.Number {
		return protoreflect.ValueOf(nil)
	}
	r := *dynamic.GetField[zc.Range](m, getter.Offset)
	return protoreflect.ValueOf(r.Bytes(m.Shared.Src))
}

func getOneofMessage(m *dynamic.Message, ty *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	which := unsafe2.ByteLoad[uint32](m, getter.Offset.Bit)
	if which != getter.Offset.Number {
		return protoreflect.ValueOf(empty{newType(ty)})
	}
	ptr := *dynamic.GetField[*Message](m, getter.Offset)
	return protoreflect.ValueOf(ptr)
}

//go:nosplit
//fastpb:stencil parseOneofVarint32 parseOneofVarint[uint32]
//fastpb:stencil parseOneofVarint64 parseOneofVarint[uint64]
func parseOneofVarint[T tdp.Int](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	p1, p2, p2.Scratch = p1.Varint(p2)
	p1, p2 = vm.StoreFromScratch[T](p1, p2)
	unsafe2.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)

	return p1, p2
}

//go:nosplit
//fastpb:stencil parseOneofZigZag32 parseOneofZigZag[uint32]
//fastpb:stencil parseOneofZigZag64 parseOneofZigZag[uint64]
func parseOneofZigZag[T tdp.Int](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	p1, p2, p2.Scratch = p1.Varint(p2)
	p2.Scratch = uint64(zigzag64[T](p2.Scratch))
	p1, p2 = vm.StoreFromScratch[T](p1, p2)
	unsafe2.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)

	return p1, p2
}

//go:nosplit
func parseOneofFixed32(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n uint32
	p1, p2, n = p1.Fixed32(p2)
	p2.Scratch = uint64(n)
	p1, p2 = vm.StoreFromScratch[uint32](p1, p2)
	unsafe2.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)

	return p1, p2
}

//go:nosplit
func parseOneofFixed64(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	p1, p2, p2.Scratch = p1.Fixed64(p2)
	p1, p2 = vm.StoreFromScratch[uint64](p1, p2)
	unsafe2.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)

	return p1, p2
}

//go:nosplit
func parseOneofString(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var r zc.Range
	p1, p2, r = p1.UTF8(p2)
	p2.Scratch = uint64(r)
	p1, p2 = vm.StoreFromScratch[uint64](p1, p2)
	unsafe2.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)

	return p1, p2
}

//go:nosplit
func parseOneofBytes(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var r zc.Range
	p1, p2, r = p1.Bytes(p2)
	p2.Scratch = uint64(r)
	p1, p2 = vm.StoreFromScratch[uint64](p1, p2)
	unsafe2.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)

	return p1, p2
}

func parseOneofBool(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	if n != 0 {
		n = 1
	}
	p2.Scratch = n
	p1, p2 = vm.StoreFromScratch[byte](p1, p2)
	unsafe2.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)

	return p1, p2
}

func parseOneofMessage(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	unsafe2.ByteStore(p2.Message(), p2.Field().Offset.Bit, p2.Field().Offset.Number)
	return parseMessage(p1, p2)
}
