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
		getter:  getOneofScalar[int32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofVarint32}},
	},
	protoreflect.Uint32Kind: {
		layout:  layout.Of[uint32](),
		oneof:   true,
		getter:  getOneofScalar[uint32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofVarint32}},
	},
	protoreflect.Sint32Kind: {
		layout:  layout.Of[int32](),
		oneof:   true,
		getter:  getOneofScalar[int32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofZigZag32}},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		layout:  layout.Of[int64](),
		oneof:   true,
		getter:  getOneofScalar[int64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofVarint64}},
	},
	protoreflect.Uint64Kind: {
		layout:  layout.Of[uint64](),
		oneof:   true,
		getter:  getOneofScalar[uint64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofVarint64}},
	},
	protoreflect.Sint64Kind: {
		layout:  layout.Of[int64](),
		oneof:   true,
		getter:  getOneofScalar[int64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofZigZag64}},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		layout:  layout.Of[uint32](),
		oneof:   true,
		getter:  getOneofScalar[uint32],
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOneofFixed32}},
	},
	protoreflect.Sfixed32Kind: {
		layout:  layout.Of[int32](),
		oneof:   true,
		getter:  getOneofScalar[int32],
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOneofFixed32}},
	},
	protoreflect.FloatKind: {
		layout:  layout.Of[float32](),
		oneof:   true,
		getter:  getOneofScalar[float32],
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOneofFixed32}},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		layout:  layout.Of[uint64](),
		oneof:   true,
		getter:  getOneofScalar[uint64],
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOneofFixed64}},
	},
	protoreflect.Sfixed64Kind: {
		layout:  layout.Of[int64](),
		oneof:   true,
		getter:  getOneofScalar[int64],
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOneofFixed64}},
	},
	protoreflect.DoubleKind: {
		layout:  layout.Of[float64](),
		oneof:   true,
		getter:  getOneofScalar[float64],
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOneofFixed64}},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		layout:  layout.Of[byte](),
		oneof:   true,
		getter:  getOneofBool,
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofBool}},
	},
	protoreflect.EnumKind: {
		layout:  layout.Of[protoreflect.EnumNumber](),
		oneof:   true,
		getter:  getOneofScalar[protoreflect.EnumNumber],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOneofVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		layout:  layout.Of[zc.Range](),
		oneof:   true,
		getter:  getOneofString,
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOneofString}},
	},
	proto2StringKind: {
		layout:  layout.Of[zc.Range](),
		oneof:   true,
		getter:  getOneofString,
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOneofBytes}},
	},
	protoreflect.BytesKind: {
		layout:  layout.Of[zc.Range](),
		oneof:   true,
		getter:  getOneofBytes,
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOneofBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		// A singular message is laid out as a single *message pointer.
		layout: layout.Of[*message](),
		oneof:  true,
		getter: getOneofMessage,
		// This message parser is eager. TODO: add a lazy message archetype.
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOneofMessage}},
	},
	protoreflect.GroupKind: {
		// Not implemented.
	},
}

func getOneofScalar[T scalar](m *message, _ Type, getter getter) protoreflect.Value {
	which := unsafe2.ByteLoad[uint32](m, getter.offset.bit)
	if which != getter.offset.number {
		return protoreflect.ValueOf(nil)
	}
	v := *getField[T](m, getter.offset)
	return protoreflect.ValueOf(v)
}

func getOneofBool(m *message, _ Type, getter getter) protoreflect.Value {
	which := unsafe2.ByteLoad[uint32](m, getter.offset.bit)
	if which != getter.offset.number {
		return protoreflect.ValueOf(nil)
	}
	v := *getField[byte](m, getter.offset)
	return protoreflect.ValueOf(v != 0)
}

func getOneofString(m *message, _ Type, getter getter) protoreflect.Value {
	which := unsafe2.ByteLoad[uint32](m, getter.offset.bit)
	if which != getter.offset.number {
		return protoreflect.ValueOf(nil)
	}
	r := *getField[zc.Range](m, getter.offset)
	return protoreflect.ValueOf(r.String(m.context.src))
}

func getOneofBytes(m *message, _ Type, getter getter) protoreflect.Value {
	which := unsafe2.ByteLoad[uint32](m, getter.offset.bit)
	if which != getter.offset.number {
		return protoreflect.ValueOf(nil)
	}
	r := *getField[zc.Range](m, getter.offset)
	return protoreflect.ValueOf(r.Bytes(m.context.src))
}

func getOneofMessage(m *message, ty Type, getter getter) protoreflect.Value {
	which := unsafe2.ByteLoad[uint32](m, getter.offset.bit)
	if which != getter.offset.number {
		return protoreflect.ValueOf(empty{ty})
	}
	ptr := *getField[*message](m, getter.offset)
	return protoreflect.ValueOf(ptr)
}

//go:nosplit
//fastpb:stencil parseOneofVarint32 parseOneofVarint[uint32]
//fastpb:stencil parseOneofVarint64 parseOneofVarint[uint64]
func parseOneofVarint[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	p1, p2, p2.scratch = p1.varint(p2)
	p1, p2 = storeFromScratch[T](p1, p2)
	unsafe2.ByteStore(p2.m(), p2.f().offset.bit, p2.f().offset.number)

	return p1, p2
}

//go:nosplit
//fastpb:stencil parseOneofZigZag32 parseOneofZigZag[uint32]
//fastpb:stencil parseOneofZigZag64 parseOneofZigZag[uint64]
func parseOneofZigZag[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	p1, p2, p2.scratch = p1.varint(p2)
	p2.scratch = uint64(zigzag64[T](p2.scratch))
	p1, p2 = storeFromScratch[T](p1, p2)
	unsafe2.ByteStore(p2.m(), p2.f().offset.bit, p2.f().offset.number)

	return p1, p2
}

//go:nosplit
func parseOneofFixed32(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.fixed32(p2)
	p2.scratch = uint64(n)
	p1, p2 = storeFromScratch[uint32](p1, p2)
	unsafe2.ByteStore(p2.m(), p2.f().offset.bit, p2.f().offset.number)

	return p1, p2
}

//go:nosplit
func parseOneofFixed64(p1 parser1, p2 parser2) (parser1, parser2) {
	p1, p2, p2.scratch = p1.fixed64(p2)
	p1, p2 = storeFromScratch[uint64](p1, p2)
	unsafe2.ByteStore(p2.m(), p2.f().offset.bit, p2.f().offset.number)

	return p1, p2
}

//go:nosplit
func parseOneofString(p1 parser1, p2 parser2) (parser1, parser2) {
	var r zc.Range
	p1, p2, r = p1.utf8(p2)
	p2.scratch = uint64(r)
	p1, p2 = storeFromScratch[uint64](p1, p2)
	unsafe2.ByteStore(p2.m(), p2.f().offset.bit, p2.f().offset.number)

	return p1, p2
}

//go:nosplit
func parseOneofBytes(p1 parser1, p2 parser2) (parser1, parser2) {
	var r zc.Range
	p1, p2, r = p1.bytes(p2)
	p2.scratch = uint64(r)
	p1, p2 = storeFromScratch[uint64](p1, p2)
	unsafe2.ByteStore(p2.m(), p2.f().offset.bit, p2.f().offset.number)

	return p1, p2
}

func parseOneofBool(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	if n != 0 {
		n = 1
	}
	p2.scratch = n
	p1, p2 = storeFromScratch[byte](p1, p2)
	unsafe2.ByteStore(p2.m(), p2.f().offset.bit, p2.f().offset.number)

	return p1, p2
}

func parseOneofMessage(p1 parser1, p2 parser2) (parser1, parser2) {
	unsafe2.ByteStore(p2.m(), p2.f().offset.bit, p2.f().offset.number)
	return parseMessage(p1, p2)
}
