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

	"github.com/bufbuild/fastpb/internal/tdp"
	"github.com/bufbuild/fastpb/internal/tdp/dynamic"
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
		getter:  adaptGetter(getScalar[int32]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint32}},
	},
	protoreflect.Uint32Kind: {
		layout:  layout.Of[uint32](),
		getter:  adaptGetter(getScalar[uint32]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint32}},
	},
	protoreflect.Sint32Kind: {
		layout:  layout.Of[int32](),
		getter:  adaptGetter(getScalar[int32]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseZigZag32}},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		layout:  layout.Of[int64](),
		getter:  adaptGetter(getScalar[int64]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint64}},
	},
	protoreflect.Uint64Kind: {
		layout:  layout.Of[uint64](),
		getter:  adaptGetter(getScalar[uint64]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint64}},
	},
	protoreflect.Sint64Kind: {
		layout:  layout.Of[int64](),
		getter:  adaptGetter(getScalar[int64]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseZigZag64}},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		layout:  layout.Of[uint32](),
		getter:  adaptGetter(getScalar[uint32]),
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseFixed32}},
	},
	protoreflect.Sfixed32Kind: {
		layout:  layout.Of[int32](),
		getter:  adaptGetter(getScalar[int32]),
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseFixed32}},
	},
	protoreflect.FloatKind: {
		layout:  layout.Of[float32](),
		getter:  adaptGetter(getFloat32),
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseFixed32}},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		layout:  layout.Of[uint64](),
		getter:  adaptGetter(getScalar[uint64]),
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseFixed64}},
	},
	protoreflect.Sfixed64Kind: {
		layout:  layout.Of[int64](),
		getter:  adaptGetter(getScalar[int64]),
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseFixed64}},
	},
	protoreflect.DoubleKind: {
		layout:  layout.Of[float64](),
		getter:  adaptGetter(getFloat64),
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseFixed64}},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		layout:  layout.Of[[0]byte](),
		bits:    1,
		getter:  adaptGetter(getBool),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseBool}},
	},
	protoreflect.EnumKind: {
		layout:  layout.Of[protoreflect.EnumNumber](),
		getter:  adaptGetter(getScalar[protoreflect.EnumNumber]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		layout:  layout.Of[zc.Range](),
		getter:  adaptGetter(getString),
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseString}},
	},
	proto2StringKind: {
		layout:  layout.Of[zc.Range](),
		getter:  adaptGetter(getString),
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseBytes}},
	},
	protoreflect.BytesKind: {
		layout:  layout.Of[zc.Range](),
		getter:  adaptGetter(getBytes),
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		// A singular message is laid out as a single *message pointer.
		layout: layout.Of[*dynamic.Message](),
		getter: adaptGetter(getMessage),
		// This message parser is eager. TODO: add a lazy message archetype.
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseMessage}},
	},
	protoreflect.GroupKind: {
		// Not implemented.
	},
}

func getScalar[T scalar](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[T](m, getter.Offset)
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

func getBool(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	b := m.GetBit(getter.Offset.Bit)
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
func getFloat32(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[uint32](m, getter.Offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := *p
	if v == 0 {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(math.Float32frombits(v))
}

func getFloat64(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[uint64](m, getter.Offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := *p
	if v == 0 {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(math.Float64frombits(v))
}

func getString(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[zc.Range](m, getter.Offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	r := *p
	data := r.String(m.Shared.Src)

	if data == "" {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(data)
}

func getBytes(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[zc.Range](m, getter.Offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	r := *p
	data := r.Bytes(m.Shared.Src)

	if len(data) == 0 {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(data)
}

func getMessage(m *dynamic.Message, ty *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[*Message](m, getter.Offset)
	if p == nil {
		return protoreflect.ValueOf(nil)
	}

	sub := *p
	if sub == nil {
		return protoreflect.ValueOf(empty{newType(ty)})
	}
	return protoreflect.ValueOf(sub)
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
	p2.m().SetBit(p2.f().Offset.Bit, n != 0)

	return p1, p2
}

func parseMessage(p1 parser1, p2 parser2) (parser1, parser2) {
	var n int
	p1, p2, n = p1.lengthPrefix(p2)
	p2.scratch = uint64(n)

	var mp **dynamic.Message
	p1, p2, mp = getMutableField[*dynamic.Message](p1, p2)
	m := *mp
	if m == nil {
		p1, p2, m = p1.alloc(p2)
		unsafe2.StoreNoWB(mp, m)
	}

	return p1.message(p2, int(p2.scratch), m)
}
