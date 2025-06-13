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
	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
	"github.com/bufbuild/fastpb/internal/zc"
)

// Optionals are implemented as one bit for presence in the hasbits array, and
// storage for the singular equivalent (see field_singular.go).
//
// Optional bool is two bits; one hasbit and one value. Optional message is
// equivalent to singular message.

var optionalFields = map[protoreflect.Kind]*archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		layout:  layout.Of[int32](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[int32]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalVarint32}},
	},
	protoreflect.Uint32Kind: {
		layout:  layout.Of[uint32](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[uint32]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalVarint32}},
	},
	protoreflect.Sint32Kind: {
		layout:  layout.Of[int32](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[int32]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalZigZag32}},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		layout:  layout.Of[int64](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[int64]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalVarint64}},
	},
	protoreflect.Uint64Kind: {
		layout:  layout.Of[uint64](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[uint64]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalVarint64}},
	},
	protoreflect.Sint64Kind: {
		layout:  layout.Of[int64](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[int64]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalZigZag64}},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		layout:  layout.Of[uint32](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[uint32]),
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOptionalFixed32}},
	},
	protoreflect.Sfixed32Kind: {
		layout:  layout.Of[int32](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[int32]),
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOptionalFixed32}},
	},
	protoreflect.FloatKind: {
		layout:  layout.Of[float32](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[float32]),
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOptionalFixed32}},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		layout:  layout.Of[uint64](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[uint64]),
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOptionalFixed64}},
	},
	protoreflect.Sfixed64Kind: {
		layout:  layout.Of[int64](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[int64]),
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOptionalFixed64}},
	},
	protoreflect.DoubleKind: {
		layout:  layout.Of[float64](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[float64]),
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOptionalFixed64}},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		layout:  layout.Of[struct{}](),
		bits:    2,
		getter:  adaptGetter(getOptionalBool),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalBool}},
	},
	protoreflect.EnumKind: {
		layout:  layout.Of[protoreflect.EnumNumber](),
		bits:    1,
		getter:  adaptGetter(getOptionalScalar[protoreflect.EnumNumber]),
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		layout:  layout.Of[zc.Range](),
		bits:    1,
		getter:  adaptGetter(getOptionalString),
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOptionalString}},
	},
	proto2StringKind: {
		layout:  layout.Of[zc.Range](),
		bits:    1,
		getter:  adaptGetter(getOptionalString),
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOptionalBytes}},
	},
	protoreflect.BytesKind: {
		layout:  layout.Of[zc.Range](),
		bits:    1,
		getter:  adaptGetter(getOptionalBytes),
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOptionalBytes}},
	},

	// Same layout as for singular.
	protoreflect.MessageKind: singularFields[protoreflect.MessageKind],
	protoreflect.GroupKind:   singularFields[protoreflect.GroupKind],
}

func getOptionalScalar[T scalar](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	if !m.GetBit(getter.Offset.Bit) {
		return protoreflect.ValueOf(nil)
	}
	v := *dynamic.GetField[T](m, getter.Offset)
	return protoreflect.ValueOf(v)
}

func getOptionalBool(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	if !m.GetBit(getter.Offset.Bit) {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(m.GetBit(getter.Offset.Bit + 1))
}

func getOptionalString(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	if !m.GetBit(getter.Offset.Bit) {
		return protoreflect.ValueOf(nil)
	}
	r := *dynamic.GetField[zc.Range](m, getter.Offset)
	return protoreflect.ValueOf(r.String(m.Shared.Src))
}

func getOptionalBytes(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	if !m.GetBit(getter.Offset.Bit) {
		return protoreflect.ValueOf(nil)
	}
	r := *dynamic.GetField[zc.Range](m, getter.Offset)
	return protoreflect.ValueOf(r.Bytes(m.Shared.Src))
}

//go:nosplit
//fastpb:stencil parseOptionalVarint32 parseOptionalVarint[uint32]
//fastpb:stencil parseOptionalVarint64 parseOptionalVarint[uint64]
func parseOptionalVarint[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	p1, p2, p2.scratch = p1.varint(p2)
	p1, p2 = storeFromScratch[T](p1, p2)
	p1, p2 = p1.setBit(p2)

	return p1, p2
}

//go:nosplit
//fastpb:stencil parseOptionalZigZag32 parseOptionalZigZag[uint32]
//fastpb:stencil parseOptionalZigZag64 parseOptionalZigZag[uint64]
func parseOptionalZigZag[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	p1, p2, p2.scratch = p1.varint(p2)
	p2.scratch = uint64(zigzag64[T](p2.scratch))
	p1, p2 = storeFromScratch[T](p1, p2)
	p1, p2 = p1.setBit(p2)

	return p1, p2
}

//go:nosplit
func parseOptionalFixed32(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.fixed32(p2)
	p2.scratch = uint64(n)
	p1, p2 = storeFromScratch[uint32](p1, p2)
	p1, p2 = p1.setBit(p2)

	return p1, p2
}

//go:nosplit
func parseOptionalFixed64(p1 parser1, p2 parser2) (parser1, parser2) {
	p1, p2, p2.scratch = p1.fixed64(p2)
	p1, p2 = storeFromScratch[uint64](p1, p2)
	p1, p2 = p1.setBit(p2)

	return p1, p2
}

//go:nosplit
func parseOptionalString(p1 parser1, p2 parser2) (parser1, parser2) {
	var r zc.Range
	p1, p2, r = p1.utf8(p2)
	p2.scratch = uint64(r)
	p1, p2 = storeFromScratch[uint64](p1, p2)
	p1, p2 = p1.setBit(p2)

	return p1, p2
}

//go:nosplit
func parseOptionalBytes(p1 parser1, p2 parser2) (parser1, parser2) {
	var r zc.Range
	p1, p2, r = p1.bytes(p2)
	p2.scratch = uint64(r)
	p1, p2 = storeFromScratch[uint64](p1, p2)
	p1, p2 = p1.setBit(p2)

	return p1, p2
}

func parseOptionalBool(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	p1, p2 = p1.setBit(p2)
	p2.m().SetBit(p2.f().Offset.Bit+1, n != 0)

	return p1, p2
}
