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
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/unsafe2"
)

//go:generate go run ./internal/stencil

var optionalFields = [...]archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    1,
		getter:  getOptionalScalar[int32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalVarint32}},
	},
	protoreflect.Uint32Kind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    1,
		getter:  getOptionalScalar[uint32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalVarint32}},
	},
	protoreflect.Sint32Kind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    1,
		getter:  getOptionalScalar[int32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalZigZag32}},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    1,
		getter:  getOptionalScalar[int64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalVarint64}},
	},
	protoreflect.Uint64Kind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    1,
		getter:  getOptionalScalar[uint64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalVarint64}},
	},
	protoreflect.Sint64Kind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    1,
		getter:  getOptionalScalar[int64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalZigZag64}},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    1,
		getter:  getOptionalScalar[uint32],
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOptionalFixed32}},
	},
	protoreflect.Sfixed32Kind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    1,
		getter:  getOptionalScalar[int32],
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOptionalFixed32}},
	},
	protoreflect.FloatKind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    1,
		getter:  getOptionalScalar[float32],
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseOptionalFixed32}},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    1,
		getter:  getOptionalScalar[uint64],
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOptionalFixed64}},
	},
	protoreflect.Sfixed64Kind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    1,
		getter:  getOptionalScalar[int64],
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOptionalFixed64}},
	},
	protoreflect.DoubleKind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    1,
		getter:  getOptionalScalar[float64],
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseOptionalFixed64}},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		size: 0, align: 1, bits: 2,
		getter:  getOptionalBool,
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalBool}},
	},
	protoreflect.EnumKind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    1,
		getter:  getOptionalScalar[protoreflect.EnumNumber],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseOptionalVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		size:    uint32(zcSize),
		align:   uint32(zcAlign),
		bits:    1,
		getter:  getOptionalString,
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOptionalString}},
	},
	protoreflect.BytesKind: {
		size:    uint32(zcSize),
		align:   uint32(zcAlign),
		bits:    1,
		getter:  getOptionalBytes,
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOptionalBytes}},
	},

	// Same layout as for singular.
	protoreflect.MessageKind: singularFields[protoreflect.MessageKind],
	protoreflect.GroupKind:   singularFields[protoreflect.GroupKind],
}

func getOptionalScalar[T scalar](m *message, _ Type, getter getter) protoreflect.Value {
	if !m.getBit(getter.offset.bit) {
		return protoreflect.ValueOf(nil)
	}
	v := *getField[T](m, getter.offset)
	return protoreflect.ValueOf(v)
}

func getOptionalBool(m *message, _ Type, getter getter) protoreflect.Value {
	if !m.getBit(getter.offset.bit) {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(m.getBit(getter.offset.bit + 1))
}

func getOptionalString(m *message, _ Type, getter getter) protoreflect.Value {
	if !m.getBit(getter.offset.bit) {
		return protoreflect.ValueOf(nil)
	}
	zc := *getField[zc](m, getter.offset)
	return protoreflect.ValueOf(zc.utf8(m.context.src))
}

func getOptionalBytes(m *message, _ Type, getter getter) protoreflect.Value {
	if !m.getBit(getter.offset.bit) {
		return protoreflect.ValueOf(nil)
	}
	zc := *getField[zc](m, getter.offset)
	return protoreflect.ValueOf(zc.bytes(m.context.src))
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
	var zc zc
	p1, p2, zc = p1.utf8(p2)
	p2.scratch = uint64(zc)
	p1, p2 = storeFromScratch[uint64](p1, p2)
	p1, p2 = p1.setBit(p2)

	return p1, p2
}

//go:nosplit
func parseOptionalBytes(p1 parser1, p2 parser2) (parser1, parser2) {
	var zc zc
	p1, p2, zc = p1.bytes(p2)
	p2.scratch = uint64(zc)
	p1, p2 = storeFromScratch[uint64](p1, p2)
	p1, p2 = p1.setBit(p2)

	return p1, p2
}

func parseOptionalBool(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	p1, p2 = p1.setBit(p2)
	p2.m().setBit(p2.f().offset.bit+1, n != 0)

	return p1, p2
}
