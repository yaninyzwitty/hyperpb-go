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

// map<bool, V> is implemented as a pair of V-typed optional fields. The first
// one is the entry for false, and the second is the entry for true.

var boolMapFields = map[protoreflect.Kind]*archetype{
	protoreflect.Int32Kind: {
		size:    2 * uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    2,
		getter:  getMap2xI[int32],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapV32}},
	},
	protoreflect.Int64Kind: {
		size:    2 * uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    2,
		getter:  getMap2xI[int64],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapV64}},
	},
	protoreflect.Uint32Kind: {
		size:    2 * uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    2,
		getter:  getMap2xI[uint32],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapV32}},
	},
	protoreflect.Uint64Kind: {
		size:    2 * uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    2,
		getter:  getMap2xI[uint64],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapV64}},
	},
	protoreflect.Sint32Kind: {
		size:    2 * uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    2,
		getter:  getMap2xI[int32],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapZ32}},
	},
	protoreflect.Sint64Kind: {
		size:    2 * uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    2,
		getter:  getMap2xI[int64],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapZ64}},
	},

	protoreflect.Fixed32Kind: {
		size:    2 * uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    2,
		getter:  getMap2xI[uint32],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF32}},
	},
	protoreflect.Fixed64Kind: {
		size:    2 * uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    2,
		getter:  getMap2xI[uint64],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF64}},
	},
	protoreflect.Sfixed32Kind: {
		size:    2 * uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    2,
		getter:  getMap2xI[int32],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF32}},
	},
	protoreflect.Sfixed64Kind: {
		size:    2 * uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    2,
		getter:  getMap2xI[int64],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF64}},
	},
	protoreflect.FloatKind: {
		size:    2 * uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    2,
		getter:  getMap2xI[float32],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF32}},
	},
	protoreflect.DoubleKind: {
		size:    2 * uint32(unsafe2.Int64Size),
		align:   uint32(unsafe2.Int64Align),
		bits:    2,
		getter:  getMap2xI[float64],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF64}},
	},

	protoreflect.BoolKind: {
		align:   1,
		bits:    4,
		getter:  getBoolBoolMap,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolBoolMap}},
	},
	protoreflect.EnumKind: {
		size:    2 * uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		bits:    2,
		getter:  getMap2xI[protoreflect.EnumNumber],
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapV32}},
	},

	protoreflect.StringKind: {
		size:    2 * uint32(zcSize),
		align:   uint32(zcAlign),
		bits:    2,
		getter:  getMap2xS,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapS}},
	},
	protoreflect.BytesKind: {
		size:    2 * uint32(zcSize),
		align:   uint32(zcAlign),
		bits:    2,
		getter:  getMap2xB,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapB}},
	},

	protoreflect.MessageKind: {
		// Not implemented.
	},
	protoreflect.GroupKind: {
		// Not implemented.
	},
}

// getMap2xI is a [getterThunk] for map<bool, V> for V an integer.
func getMap2xI[V any](m *message, _ Type, getter getter) protoreflect.Value {
	return protoreflect.ValueOf(map2xI[V]{m: m, f: getter.offset})
}

// map2xI is a [protoreflect.Map] for map<bool, V> for V an integer.
type map2xI[V any] struct {
	unimplementedMap
	m *message
	f fieldOffset
}

func (m map2xI[V]) Len() int {
	var n int
	if m.m.getBit(m.f.bit) {
		n++
	}
	if m.m.getBit(m.f.bit + 1) {
		n++
	}
	return n
}
func (m map2xI[V]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m map2xI[V]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	var idx uint32
	if k {
		idx = 1
	}

	p := getField[V](m.m, m.f)
	if !m.m.getBit(m.f.bit+idx) || p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := unsafe2.Add(p, idx)
	return protoreflect.ValueOf(*v)
}

func (m map2xI[V]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	v := getField[V](m.m, m.f)
	if v == nil {
		return
	}
	if m.m.getBit(m.f.bit) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(false)),
			protoreflect.ValueOf(*v)) {
		return
	}

	v = unsafe2.Add(v, 1)
	if m.m.getBit(m.f.bit+1) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(true)),
			protoreflect.ValueOf(*v)) {
		return
	}
}

// getMap2xS is a [getterThunk] for map<bool, string>.
func getMap2xS(m *message, _ Type, getter getter) protoreflect.Value {
	return protoreflect.ValueOf(map2xS{m: m, f: getter.offset})
}

// map2xS is a [protoreflect.Map] for map<bool, string>.
type map2xS struct {
	unimplementedMap
	m *message
	f fieldOffset
}

func (m map2xS) Len() int {
	var n int
	if m.m.getBit(m.f.bit) {
		n++
	}
	if m.m.getBit(m.f.bit + 1) {
		n++
	}
	return n
}
func (m map2xS) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m map2xS) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	var idx uint32
	if k {
		idx = 1
	}

	p := getField[zc](m.m, m.f)
	if !m.m.getBit(m.f.bit+idx) || p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := unsafe2.Add(p, idx)
	return protoreflect.ValueOf(v.utf8(m.m.context.src))
}

func (m map2xS) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	v := getField[zc](m.m, m.f)
	if v == nil {
		return
	}
	if m.m.getBit(m.f.bit) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(false)),
			protoreflect.ValueOf(v.utf8(m.m.context.src))) {
		return
	}

	v = unsafe2.Add(v, 1)
	if m.m.getBit(m.f.bit+1) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(true)),
			protoreflect.ValueOf(v.utf8(m.m.context.src))) {
		return
	}
}

// getMap2xB is a [getterThunk] for map<bool, bytes>.
func getMap2xB(m *message, _ Type, getter getter) protoreflect.Value {
	return protoreflect.ValueOf(map2xB{m: m, f: getter.offset})
}

// map2xB is a [protoreflect.Map] for map<bool, bytes>.
type map2xB struct {
	unimplementedMap
	m *message
	f fieldOffset
}

func (m map2xB) Len() int {
	var n int
	if m.m.getBit(m.f.bit) {
		n++
	}
	if m.m.getBit(m.f.bit + 1) {
		n++
	}
	return n
}
func (m map2xB) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m map2xB) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	var idx uint32
	if k {
		idx = 1
	}

	p := getField[zc](m.m, m.f)
	if !m.m.getBit(m.f.bit+idx) || p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := unsafe2.Add(p, idx)
	return protoreflect.ValueOf(v.bytes(m.m.context.src))
}

func (m map2xB) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	v := getField[zc](m.m, m.f)
	if v == nil {
		return
	}
	if m.m.getBit(m.f.bit) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(false)),
			protoreflect.ValueOf(v.bytes(m.m.context.src))) {
		return
	}

	v = unsafe2.Add(v, 1)
	if m.m.getBit(m.f.bit+1) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(true)),
			protoreflect.ValueOf(v.bytes(m.m.context.src))) {
		return
	}
}

// getBoolBoolMap is a [getterThunk] for map<bool, bool>.
func getBoolBoolMap(m *message, _ Type, getter getter) protoreflect.Value {
	return protoreflect.ValueOf(boolBoolMap{m: m, f: getter.offset})
}

// boolBoolMap is a [protoreflect.Map] for map<bool, bool>.
type boolBoolMap struct {
	unimplementedMap
	m *message
	f fieldOffset
}

func (m boolBoolMap) Len() int {
	var n int
	if m.m.getBit(m.f.bit) {
		n++
	}
	if m.m.getBit(m.f.bit + 1) {
		n++
	}
	return n
}
func (m boolBoolMap) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m boolBoolMap) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	var idx uint32
	if k {
		idx = 1
	}

	if !m.m.getBit(m.f.bit + idx) {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(m.m.getBit(m.f.bit + idx + 2))
}

func (m boolBoolMap) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	if m.m.getBit(m.f.bit) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(false)),
			protoreflect.ValueOfBool(m.m.getBit(m.f.bit+2))) {
		return
	}

	if m.m.getBit(m.f.bit+1) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(true)),
			protoreflect.ValueOf(m.m.getBit(m.f.bit+3))) {
		return
	}
}

//go:generate go run ./internal/stencil

//fastpb:stencil parseBoolScalarMapV32 parseBoolScalarMap[varintItem[uint32], uint32]
//fastpb:stencil parseBoolScalarMapV64 parseBoolScalarMap[varintItem[uint64], uint64]
//fastpb:stencil parseBoolScalarMapZ32 parseBoolScalarMap[zigzagItem[uint32], uint32]
//fastpb:stencil parseBoolScalarMapZ64 parseBoolScalarMap[zigzagItem[uint64], uint64]
//fastpb:stencil parseBoolScalarMapF32 parseBoolScalarMap[fixed32Item, uint32]
//fastpb:stencil parseBoolScalarMapF64 parseBoolScalarMap[fixed64Item, uint64]
//fastpb:stencil parseBoolScalarMapS   parseBoolScalarMap[stringItem, uint64]
//fastpb:stencil parseBoolScalarMapB   parseBoolScalarMap[bytesItem, uint64]

// parseBoolScalarMap parses a map<bool, V> for some non-bool scalar V.
func parseBoolScalarMap[
	VI mapItem[V], V integer,
](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.lengthPrefix(p2)

	p2.scratch = uint64(p1.e_)
	p1.e_ = p1.b_.Add(int(n))

	var p *V
	p1, p2, p = getMutableField[V](p1, p2)

	var k bool
	var vi VI
	var v V

	kTag := protowire.EncodeTag(1, protowire.VarintType)
	vTag := protowire.EncodeTag(2, vi.kind())

	// Basically every map ever encodes its fields in order and does not
	// have duplicate fields, so this is a hot fast path.
	if p1.len() == 0 {
		goto insert
	}
	p1.log(p2, "first byte", "%#02x", *p1.b())
	if *p1.b() == byte(kTag) {
		p1.b_++
		var n uint64
		p1, p2, n = p1.varint(p2)
		k = n != 0
		if p1.len() == 0 {
			goto insert
		}
		p1.log(p2, "second byte", "%#02x", *p1.b())
		if *p1.b() == byte(vTag) {
			p1.b_++
			p1, p2, v = vi.parse(p1, p2)
			p1.log(p2, "map done?",
				"%v:%v, %v/%x: %v/%x",
				p1.b_, p1.e_,
				k, unsafe2.Bytes(&k),
				v, unsafe2.Bytes(&v))
			if p1.b_ == p1.e_ {
				goto insert
			}
		}
	}

	// Slow fallback. This code should almost never be executed so we can
	// afford to call varint() each time we parse a tag.
	for p1.b_ < p1.e_ {
		var tag uint64
		p1, p2, tag = p1.varint(p2)
		switch tag {
		case kTag:
			var n uint64
			p1, p2, n = p1.varint(p2)
			k = n != 0
		case vTag:
			p1, p2, v = vi.parse(p1, p2)
		default:
			n, t := protowire.DecodeTag(tag)
			m := protowire.ConsumeFieldValue(n, t, p1.buf())
			if m < 0 {
				p1.fail(p2, -errCode(m))
			}
			p1.b_ = p1.b_.Add(m)
		}
	}

insert:
	var idx uint32
	if k {
		idx = 1
	}

	*unsafe2.Add(p, idx) = v
	p2.m().setBit(p2.f().offset.bit+idx, true)

	p1.e_ = unsafe2.Addr[byte](p2.scratch)
	return p1, p2
}

// parseBoolBoolMap parses a map<bool, bool>.
func parseBoolBoolMap(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.lengthPrefix(p2)

	p2.scratch = uint64(p1.e_)
	p1.e_ = p1.b_.Add(int(n))

	var k, v bool

	kTag := protowire.EncodeTag(1, protowire.VarintType)
	vTag := protowire.EncodeTag(2, protowire.VarintType)

	// Basically every map ever encodes its fields in order and does not
	// have duplicate fields, so this is a hot fast path.
	p1.log(p2, "first byte", "%#02x", *p1.b())
	if *p1.b() == byte(kTag) {
		p1.b_++
		var n uint64
		p1, p2, n = p1.varint(p2)
		k = n != 0
		p1.log(p2, "second byte", "%#02x", *p1.b())
		if *p1.b() == byte(vTag) {
			p1.b_++
			var n uint64
			p1, p2, n = p1.varint(p2)
			v = n != 0
			p1.log(p2, "map done?",
				"%v:%v, %v/%x: %v/%x",
				p1.b_, p1.e_,
				k, unsafe2.Bytes(&k),
				v, unsafe2.Bytes(&v))
			if p1.b_ == p1.e_ {
				goto insert
			}
		}
	}

	// Slow fallback. This code should almost never be executed so we can
	// afford to call varint() each time we parse a tag.
	for p1.b_ < p1.e_ {
		var tag uint64
		p1, p2, tag = p1.varint(p2)
		switch tag {
		case kTag:
			var n uint64
			p1, p2, n = p1.varint(p2)
			k = n != 0
		case vTag:
			var n uint64
			p1, p2, n = p1.varint(p2)
			v = n != 0
		default:
			n, t := protowire.DecodeTag(tag)
			m := protowire.ConsumeFieldValue(n, t, p1.buf())
			p1.b_ = p1.b_.Add(m)
		}
	}

insert:
	var idx uint32
	if k {
		idx = 1
	}

	p2.m().setBit(p2.f().offset.bit+idx, true)
	p2.m().setBit(p2.f().offset.bit+idx+2, v)

	p1.e_ = unsafe2.Addr[byte](p2.scratch)
	return p1, p2
}
