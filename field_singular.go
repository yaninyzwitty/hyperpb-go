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
	"math"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/unsafe2"
)

//go:generate go run ./internal/stencil

// singularFields consists of archetypes for singular (i.e., non-optional)
// fields of each field type.
var singularFields = [...]archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		getter:  getScalar[int32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint32}},
	},
	protoreflect.Uint32Kind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		getter:  getScalar[uint32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint32}},
	},
	protoreflect.Sint32Kind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		getter:  getScalar[int32],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseZigZag32}},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		getter:  getScalar[int64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint64}},
	},
	protoreflect.Uint64Kind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		getter:  getScalar[uint64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint64}},
	},
	protoreflect.Sint64Kind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		getter:  getScalar[int64],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseZigZag64}},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		getter:  getScalar[uint32],
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseFixed32}},
	},
	protoreflect.Sfixed32Kind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		getter:  getScalar[int32],
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseFixed32}},
	},
	protoreflect.FloatKind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		getter:  getFloat32,
		parsers: []parseKind{{kind: protowire.Fixed32Type, parser: parseFixed32}},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		getter:  getScalar[uint64],
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseFixed64}},
	},
	protoreflect.Sfixed64Kind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		getter:  getScalar[int64],
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseFixed64}},
	},
	protoreflect.DoubleKind: {
		size:    uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		getter:  getFloat64,
		parsers: []parseKind{{kind: protowire.Fixed64Type, parser: parseFixed64}},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		size: 0, align: 1, bits: 1,
		getter:  getBool,
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseBool}},
	},
	protoreflect.EnumKind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		getter:  getScalar[protoreflect.EnumNumber],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		size:    uint32(zcSize),
		align:   uint32(zcAlign),
		getter:  getString,
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseString}},
	},
	protoreflect.BytesKind: {
		size:    uint32(zcSize),
		align:   uint32(zcAlign),
		getter:  getBytes,
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		// A singular message is laid out as a single *message pointer.
		size:   uint32(unsafe2.PointerSize),
		align:  uint32(unsafe2.PointerAlign),
		getter: getMessage,
		// This message parser is eager. TODO: add a lazy message archetype.
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseMessage}},
	},
	protoreflect.GroupKind: {
		// Not implemented.
	},
}

func getScalar[T scalar](m *message, _ Type, getter getter) protoreflect.Value {
	v := unsafe2.ByteLoad[T](m, getter.offset.data)

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
	v := unsafe2.ByteLoad[uint32](m, getter.offset.data)
	if v == 0 {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(math.Float32frombits(v))
}

func getFloat64(m *message, _ Type, getter getter) protoreflect.Value {
	v := unsafe2.ByteLoad[uint64](m, getter.offset.data)
	if v == 0 {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(math.Float64frombits(v))
}

func getString(m *message, _ Type, getter getter) protoreflect.Value {
	zc := unsafe2.ByteLoad[zc](m, getter.offset.data)
	data := zc.utf8(m.context.src)

	if data == "" {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(data)
}

func getBytes(m *message, _ Type, getter getter) protoreflect.Value {
	zc := unsafe2.ByteLoad[zc](m, getter.offset.data)
	data := zc.bytes(m.context.src)

	if len(data) == 0 {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(data)
}

func getMessage(m *message, ty Type, getter getter) protoreflect.Value {
	ptr := unsafe2.ByteLoad[*message](m, getter.offset.data)
	if ptr == nil {
		return protoreflect.ValueOf(empty{ty})
	}
	return protoreflect.ValueOf(ptr)
}

//go:nosplit
//fastpb:stencil parseVarint32 parseVarint[uint32]
//fastpb:stencil parseVarint64 parseVarint[uint64]
func parseVarint[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	storeField(p1, p2, T(n))

	return p1, p2
}

//go:nosplit
//fastpb:stencil parseZigZag32 parseZigZag[uint32]
//fastpb:stencil parseZigZag64 parseZigZag[uint64]
func parseZigZag[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	storeField(p1, p2, zigzag64[T](n))

	return p1, p2
}

func parseFixed32(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.fixed32(p2)
	storeField(p1, p2, n)

	return p1, p2
}

func parseFixed64(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.fixed64(p2)
	storeField(p1, p2, n)

	return p1, p2
}

func parseString(p1 parser1, p2 parser2) (parser1, parser2) {
	var zc zc
	p1, p2, zc = p1.utf8(p2)
	storeField(p1, p2, zc)

	return p1, p2
}

func parseBytes(p1 parser1, p2 parser2) (parser1, parser2) {
	var zc zc
	p1, p2, zc = p1.bytes(p2)
	storeField(p1, p2, zc)

	return p1, p2
}

func parseBool(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	p2.m().setBit(p2.f().offset.bit, n != 0)

	return p1, p2
}

func parseMessage(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.lengthPrefix(p2)

	// This is an address for the same reason that rep[E].ptr is.
	mp := unsafe2.Cast[unsafe2.Addr[message]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
	m := (*mp).AssertValid() //nolint:gocritic // Explicit deref for emphasis.
	if m == nil {
		p1, p2, m = p1.alloc(p2)
		*mp = unsafe2.AddrOf(m)
	}

	return p1.message(p2, int(n), m)
}
