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
	"math"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
	"github.com/bufbuild/fastpb/internal/zc"
)

// Singular fields are implemented as a single field of the appropriate type.
// For integers (and floats, which we never materialize as floats in the parser)
// this is either uint32 or uint64. For bool, this is uint8.
//
// Strings and bytes are stored as a [zc].
//
// Messages, which are optional even when singular, are stored as a *message
// pointer. The pointer is nil when the field is not set.

// singularFields consists of archetypes for singular (i.e., non-optional)
// fields of each field type.
var singularFields = map[protoreflect.Kind]*archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		layout:  layout.Of[int32](),
		getter:  getScalar[int32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint32}},
	},
	protoreflect.Uint32Kind: {
		layout:  layout.Of[uint32](),
		getter:  getScalar[uint32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint32}},
	},
	protoreflect.Sint32Kind: {
		layout:  layout.Of[int32](),
		getter:  getScalar[int32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseZigZag32}},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		layout:  layout.Of[int64](),
		getter:  getScalar[int64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint64}},
	},
	protoreflect.Uint64Kind: {
		layout:  layout.Of[uint64](),
		getter:  getScalar[uint64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint64}},
	},
	protoreflect.Sint64Kind: {
		layout:  layout.Of[int64](),
		getter:  getScalar[int64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseZigZag64}},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		layout:  layout.Of[uint32](),
		getter:  getScalar[uint32],
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseFixed32}},
	},
	protoreflect.Sfixed32Kind: {
		layout:  layout.Of[int32](),
		getter:  getScalar[int32],
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseFixed32}},
	},
	protoreflect.FloatKind: {
		layout:  layout.Of[float32](),
		getter:  getFloat32,
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseFixed32}},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		layout:  layout.Of[uint64](),
		getter:  getScalar[uint64],
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseFixed64}},
	},
	protoreflect.Sfixed64Kind: {
		layout:  layout.Of[int64](),
		getter:  getScalar[int64],
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseFixed64}},
	},
	protoreflect.DoubleKind: {
		layout:  layout.Of[float64](),
		getter:  getFloat64,
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseFixed64}},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		layout:  layout.Of[[0]byte](),
		bits:    1,
		getter:  getBool,
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseBool}},
	},
	protoreflect.EnumKind: {
		layout:  layout.Of[protoreflect.EnumNumber](),
		getter:  getScalar[protoreflect.EnumNumber],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		layout:  layout.Of[zc.Range](),
		getter:  getString,
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseString}},
	},
	proto2StringKind: {
		layout:  layout.Of[zc.Range](),
		getter:  getString,
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseBytes}},
	},
	protoreflect.BytesKind: {
		layout:  layout.Of[zc.Range](),
		getter:  getBytes,
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		// A singular message is laid out as a single *message pointer.
		layout: layout.Of[*message](),
		getter: getMessage,
		// This message parser is eager. TODO: add a lazy message archetype.
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseMessage}},
	},
	protoreflect.GroupKind: {
		// Not implemented.
	},
}

func getScalar[T scalar](m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[T](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := *p
	var zero T
	if v == zero {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v)
}

func getBool(m *message, _ Type, getter getter) protoreflect.Value {
	b := m.getBit(getter.offset.bit)
	if !b {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(true)
}

// We can't use the stencil above due to negative zero: 0.0 == -0.0 according
// to float equality, but proto3 implicit presence requires that we report
// a -0.0 as present, but 0.0 as not present.
//
// This also avoids a potential equality comparison with a signaling NaN, which
// can cause all sorts of mayhem.
func getFloat32(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[uint32](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := *p
	if v == 0 {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(math.Float32frombits(v))
}

func getFloat64(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[uint64](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := *p
	if v == 0 {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(math.Float64frombits(v))
}

func getString(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[zc.Range](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	r := *p
	data := r.String(m.context.src)

	if data == "" {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(data)
}

func getBytes(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[zc.Range](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	r := *p
	data := r.Bytes(m.context.src)

	if len(data) == 0 {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(data)
}

func getMessage(m *message, ty Type, getter getter) protoreflect.Value {
	p := getField[*message](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	m = *p
	if m == nil {
		return protoreflect.ValueOf(empty{ty})
	}
	return protoreflect.ValueOf(m)
}

//go:nosplit
//fastpb:stencil parseVarint32 parseVarint[uint32]
//fastpb:stencil parseVarint64 parseVarint[uint64]
func parseVarint[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	p1, p2, p2.scratch = p1.varint(p2)
	p1, p2 = storeFromScratch[T](p1, p2)

	return p1, p2
}

//go:nosplit
//fastpb:stencil parseZigZag32 parseZigZag[uint32]
//fastpb:stencil parseZigZag64 parseZigZag[uint64]
func parseZigZag[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	p1, p2, p2.scratch = p1.varint(p2)
	p2.scratch = uint64(zigzag64[T](p2.scratch))
	p1, p2 = storeFromScratch[T](p1, p2)

	return p1, p2
}

func parseFixed32(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.fixed32(p2)
	p2.scratch = uint64(n)
	p1, p2 = storeFromScratch[uint32](p1, p2)

	return p1, p2
}

func parseFixed64(p1 parser1, p2 parser2) (parser1, parser2) {
	p1, p2, p2.scratch = p1.fixed64(p2)
	p1, p2 = storeFromScratch[uint64](p1, p2)

	return p1, p2
}

func parseString(p1 parser1, p2 parser2) (parser1, parser2) {
	var r zc.Range
	p1, p2, r = p1.utf8(p2)
	p2.scratch = uint64(r)
	p1, p2 = storeFromScratch[uint64](p1, p2)

	return p1, p2
}

func parseBytes(p1 parser1, p2 parser2) (parser1, parser2) {
	var r zc.Range
	p1, p2, r = p1.bytes(p2)
	p2.scratch = uint64(r)
	p1, p2 = storeFromScratch[uint64](p1, p2)

	return p1, p2
}

func parseBool(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	p2.m().setBit(p2.f().offset.bit, n != 0)

	return p1, p2
}

func parseMessage(p1 parser1, p2 parser2) (parser1, parser2) {
	var n int
	p1, p2, n = p1.lengthPrefix(p2)
	p2.scratch = uint64(n)

	var mp **message
	p1, p2, mp = getMutableField[*message](p1, p2)
	m := *mp
	if m == nil {
		p1, p2, m = p1.alloc(p2)
		unsafe2.StoreNoWB(mp, m)
	}

	return p1.message(p2, int(p2.scratch), m)
}
