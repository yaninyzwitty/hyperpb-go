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
	"math/bits"
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

//go:generate go run ./internal/stencil

// scalar is a Protobuf scalar type.
type scalar interface {
	int32 | int64 |
		uint32 | uint64 |
		float32 | float64 |
		protoreflect.EnumNumber
}

// integer is any of the integer types that this package has to handle
// generically.
type integer interface {
	int8 | uint8 | int32 | int64 | uint32 | uint64
}

func zigzag64[T integer](raw uint64) T {
	return zigzag(T(raw))
}

func zigzag[T integer](raw T) T {
	n := uint64(raw)
	n &= (1 << (unsafe.Sizeof(raw) * 8)) - 1

	return T(protowire.DecodeZigZag(n))
}

// field is an optimized descriptor for a field.
type field struct {
	_ unsafe2.NoCopy
	// The message type for this field, if there is one.
	message Type
	getter  getter
}

// getter is all the information necessary for accessing a field of a [message].
type getter struct {
	offset fieldOffset

	// The thunk for extracting the field.
	thunk func(*message, Type, getter) protoreflect.Value
}

// fieldParser is a parser for a single field.
type fieldParser struct {
	_ unsafe2.NoCopy
	// The expected tag value for the field. This is encoded as a single uint64
	// so it is passed in registers.
	//
	// The high byte is the length of the tag. The tag is encoded without any
	// sign bits set, since the matching code strips off the sign bits for
	// performing the comparison.
	tag fieldTag

	// Byte offset to the typeParser this fieldParser uses, if any.
	message *typeParser

	// Field offset information for the field this parser parses. Duplicated
	// from [getter].
	offset fieldOffset

	// The parser to jump to after this one, depending on whether the parse
	// succeeds or fails.
	nextOk, nextErr *fieldParser

	// The thunk to call for this field. The bool return must always return
	// true.
	thunk unsafe2.PC[func(parser1, parser2) (parser1, parser2)]
}

// fieldOffset is field offset information for a generated message type's field.
type fieldOffset struct {
	// First bit index for bits allocated to this field.
	bit uint32
	// Byte offset within the containing message to the data for this field.
	data uint32
}

// fieldTag is a specially-formatted tag for the parser.
//
// The tag is formatted in the way it would be when encoded in a Protobuf
// message, but with the high bit of each byte cleared.
//
//nolint:recvcheck
type fieldTag uint64

// encode encodes this field tag from the given number and type.
func (ft *fieldTag) encode(n protowire.Number, t protowire.Type) {
	protowire.AppendTag(unsafe2.Bytes(ft)[:0], n, t)
	*ft &^= signBits
}

// decode decodes this field tag into a number and a type.
func (ft fieldTag) decode() uint64 {
	var tag uint64
	mask := uint64(0x7f)
	i := 0

	tag |= (uint64(ft) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(ft) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(ft) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(ft) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(ft) & mask) >> i
	mask <<= 8
	i++

	_, _ = i, mask
	return tag
}

// valid returns whether or not this is the sentinel invalid field in a [Type]'s
// field table.
func (f *field) valid() bool {
	return f.getter.thunk != nil
}

// get gets the value of this field out of a message of appropriate type.
// Returns nil if the field is unset.
//
// This performs no type-checking! Callers are responsible for ensuring that m
// is of the correct type.
func (f *field) get(m *message) protoreflect.Value {
	if !f.valid() {
		return protoreflect.ValueOf(nil)
	}
	return f.getter.thunk(m, f.message, f.getter)
}

// archetype represents a class of fields that have the same layout within a
// *message. This includes parsing and access information.
//
// Archetypes are used to organize field allocation and parsing strategies for
// use in the construction of a [fastpb.Type].
type archetype struct {
	// The layout for the field's storage in the message.
	size, align uint32
	// Bits to allocate for this field.
	bits uint32

	// The getter thunk for this field.
	//
	// This func MUST be a reference to a function or a global closure, so that
	// it is not a GC-managed pointer.
	getter func(*message, Type, getter) protoreflect.Value

	// Parsers available for different forms of this field.
	parsers []parseKind
}

type parseKind struct {
	kind protowire.Type

	// If set, the parser will always retry this field instead of going to the
	// next one if it parses successfully. Used for repeated fields.
	retry bool

	// The bool return must always be true.
	//
	// This func MUST be a reference to a function or a global closure, so that
	// it is not a GC-managed pointer.
	parser func(parser1, parser2) (parser1, parser2)
}

// selectArchetype classifies a field into a particular archetype.
//
// prof is a profile that inquires for the profile of a field within the context
// of parsing fd. It takes a FieldDescriptor rather than a FieldSite because
// the caller is responsible for constructing the FieldSite.
//
// Returns nil if the field is not supported yet.
func selectArchetype(
	fd protoreflect.FieldDescriptor,
	prof func(protoreflect.FieldDescriptor) FieldProfile,
) (a *archetype) {
	switch {
	case fd.Cardinality() == protoreflect.Repeated:
		a = &repeatedFields[fd.Kind()]

	case fd.HasPresence():
		// TODO: Oneofs are currently are implemented as a struct rather than a union.
		a = &optionalFields[fd.Kind()]
	default:
		a = &singularFields[fd.Kind()]
	}

	if a.getter == nil {
		a = nil
	}

	return a
}

// zc (short for zero-copy) is a representation of a []byte as a slice into
// the source array for a parsed message.
type zc struct {
	offset, len uint32
}

const (
	zcSize  = unsafe.Sizeof(zc{})
	zcAlign = unsafe.Alignof(zc{})
)

// bytes converts this zc into a byte slice, given the message source.
func (zc zc) bytes(src *byte) []byte {
	return unsafe2.Slice(unsafe2.Add(src, zc.offset), zc.len)
}

// utf8 converts this zc into a string, given the message source.
func (zc zc) utf8(src *byte) string {
	return unsafe2.String(unsafe2.Add(src, zc.offset), zc.len)
}

// rep is the raw form of a rep field.
//
// It is a union between zc and arena.Slice[E].
type rep[E any] struct {
	// This is an address to prevent the generation of write barriers when
	// writing to a rep[E] inside of a message.
	//
	// messages are always on the heap and we assume the GC will never move
	// them or the arena-allocated pointer this field holds.
	//
	// Not doing this means that any update to a repeated field will touch
	// the write barrier, a global atomic used to coordinate GC operations. We
	// do not need to participate in the write barrier because this address is
	// already traceable from elsewhere (the root message being operated on)
	// and will never move and thus need to be updated by the GC.
	//
	// Write barriers are generated whenever we load or store a pointer value,
	// such as when loading or storing a **T or *[]T. Touching the write barrier
	// is very slow: even in the fast path it causes a cache miss.
	//
	// Moreover, all *rep[E]s are arena pointers, which the GC cannot trace
	// into, so this pointer already doesn't exist as far as the GC is
	// concerned.
	ptr unsafe2.Addr[E]

	// This part does double duty as a zc.
	len, cap uint32
}

const (
	repSize  = unsafe.Sizeof(rep[int32]{})
	repAlign = unsafe.Alignof(rep[int32]{})
)

func repCast[T, U any](r rep[U]) rep[T] {
	return rep[T]{unsafe2.Addr[T](r.ptr), r.len, r.cap}
}

// isZC returns whether this rep is in zero-copy mode.
func (r rep[E]) isZC() bool {
	return r.ptr == 0
}

func (r rep[E]) rawZC() zc {
	return zc{offset: r.len, len: r.cap}
}

// zc interprets this rep as a zero-copy slice take from src.
func (r rep[E]) zc(src *byte) []E {
	size, _ := unsafe2.Layout[E]()
	// This is a borrow from src, and len and cap are a zc. Note that both
	// are denominated in bytes in this mode.
	return unsafe2.Slice(unsafe2.Cast[E](unsafe2.Add(src, r.len)), int(r.cap)/size)
}

// setZC sets this rep to the given ZC value.
func (r *rep[E]) setZC(v zc) {
	r.ptr = 0
	r.len = v.offset
	r.cap = v.len
}

// arena interprets this rep as an arena slice.
func (r rep[E]) arena() arena.Slice[E] {
	return arena.SliceFromParts(r.ptr.AssertValid(), r.len, r.cap)
}

// setArena sets this rep to the given arena slice.
func (r *rep[E]) setArena(v arena.Slice[E]) {
	r.ptr = unsafe2.AddrOf(v.Ptr())
	r.len = uint32(v.Len())
	r.cap = uint32(v.Cap())
}

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
		getter: func(m *message, _ Type, getter getter) protoreflect.Value {
			b := m.getBit(getter.offset.bit)
			if !b {
				return protoreflect.ValueOf(nil)
			}
			return protoreflect.ValueOf(true)
		},
		parsers: []parseKind{{
			kind: protowire.VarintType,
			parser: func(p1 parser1, p2 parser2) (parser1, parser2) {
				var n uint64
				p1, p2, n = p1.varint(p2)
				p2.m().setBit(p2.f().offset.bit, n != 0)

				return p1, p2
			},
		}},
	},
	protoreflect.EnumKind: {
		size:    uint32(unsafe2.Int32Size),
		align:   uint32(unsafe2.Int32Align),
		getter:  getScalar[protoreflect.EnumNumber],
		parsers: []parseKind{{kind: protowire.VarintType, parser: parseVarint32}},
	},

	// String types.
	protoreflect.StringKind: {
		size:  uint32(zcSize),
		align: uint32(zcAlign),
		getter: func(m *message, _ Type, getter getter) protoreflect.Value {
			zc := unsafe2.ByteLoad[zc](m, getter.offset.data)
			data := zc.utf8(m.context.src)

			if data == "" {
				return protoreflect.ValueOf(nil)
			}

			return protoreflect.ValueOf(data)
		},
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseString}},
	},
	protoreflect.BytesKind: {
		size:  uint32(zcSize),
		align: uint32(zcAlign),
		getter: func(m *message, _ Type, getter getter) protoreflect.Value {
			zc := unsafe2.ByteLoad[zc](m, getter.offset.data)
			data := zc.bytes(m.context.src)

			if len(data) == 0 {
				return protoreflect.ValueOf(nil)
			}

			return protoreflect.ValueOf(data)
		},
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		// A singular message is laid out as a single *message pointer.
		size:  uint32(unsafe2.PointerSize),
		align: uint32(unsafe2.PointerAlign),
		getter: func(m *message, ty Type, getter getter) protoreflect.Value {
			ptr := unsafe2.ByteLoad[*message](m, getter.offset.data)
			if ptr == nil {
				return protoreflect.ValueOf(empty{ty})
			}
			return protoreflect.ValueOf(ptr)
		},

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

func storeField[T any](p1 parser1, p2 parser2, v T) {
	unsafe2.ByteStore(p2.m(), p2.f().offset.data, v)
}

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
		getter: func(m *message, _ Type, getter getter) protoreflect.Value {
			if !m.getBit(getter.offset.bit) {
				return protoreflect.ValueOf(nil)
			}
			return protoreflect.ValueOf(m.getBit(getter.offset.bit + 1))
		},
		parsers: []parseKind{{
			kind: protowire.VarintType,
			parser: func(p1 parser1, p2 parser2) (parser1, parser2) {
				var n uint64
				p1, p2, n = p1.varint(p2)
				p2.m().setBit(p2.f().offset.bit, true)
				p2.m().setBit(p2.f().offset.bit+1, n != 0)

				return p1, p2
			},
		}},
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
		size:  uint32(zcSize),
		align: uint32(zcAlign),
		bits:  1,
		getter: func(m *message, _ Type, getter getter) protoreflect.Value {
			if !m.getBit(getter.offset.bit) {
				return protoreflect.ValueOf(nil)
			}
			zc := unsafe2.ByteLoad[zc](m, getter.offset.data)
			return protoreflect.ValueOf(zc.utf8(m.context.src))
		},
		parsers: []parseKind{{kind: protowire.BytesType, parser: parseOptionalString}},
	},
	protoreflect.BytesKind: {
		size:  uint32(zcSize),
		align: uint32(zcAlign),
		bits:  1,
		getter: func(m *message, _ Type, getter getter) protoreflect.Value {
			if !m.getBit(getter.offset.bit) {
				return protoreflect.ValueOf(nil)
			}
			zc := unsafe2.ByteLoad[zc](m, getter.offset.data)
			return protoreflect.ValueOf(zc.bytes(m.context.src))
		},
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
	v := unsafe2.ByteLoad[T](m, getter.offset.data)
	return protoreflect.ValueOf(v)
}

//go:nosplit
//fastpb:stencil parseOptionalVarint32 parseOptionalVarint[uint32]
//fastpb:stencil parseOptionalVarint64 parseOptionalVarint[uint64]
func parseOptionalVarint[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	storeField(p1, p2, T(n))
	p2.m().setBit(p2.f().offset.bit, true)

	return p1, p2
}

//go:nosplit
//fastpb:stencil parseOptionalZigZag32 parseOptionalZigZag[uint32]
//fastpb:stencil parseOptionalZigZag64 parseOptionalZigZag[uint64]
func parseOptionalZigZag[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	storeField(p1, p2, zigzag64[T](n))
	p2.m().setBit(p2.f().offset.bit, true)

	return p1, p2
}

//go:nosplit
func parseOptionalFixed32(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.fixed32(p2)
	storeField(p1, p2, n)
	p2.m().setBit(p2.f().offset.bit, true)

	return p1, p2
}

//go:nosplit
func parseOptionalFixed64(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.fixed64(p2)
	storeField(p1, p2, n)
	p2.m().setBit(p2.f().offset.bit, true)

	return p1, p2
}

//go:nosplit
func parseOptionalString(p1 parser1, p2 parser2) (parser1, parser2) {
	var zc zc
	p1, p2, zc = p1.utf8(p2)
	storeField(p1, p2, zc)
	p2.m().setBit(p2.f().offset.bit, true)

	return p1, p2
}

//go:nosplit
func parseOptionalBytes(p1 parser1, p2 parser2) (parser1, parser2) {
	var zc zc
	p1, p2, zc = p1.bytes(p2)
	storeField(p1, p2, zc)
	p2.m().setBit(p2.f().offset.bit, true)

	return p1, p2
}

var repeatedFields = [...]archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalarMaybeBytes[int32],
		parsers: []parseKind{
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint32},
			{kind: protowire.BytesType, parser: parsePackedVarint32},
		},
	},
	protoreflect.Uint32Kind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalarMaybeBytes[uint32],
		parsers: []parseKind{
			{kind: protowire.VarintType, parser: parseRepeatedVarint32},
			{kind: protowire.BytesType, parser: parsePackedVarint32},
		},
	},
	protoreflect.Sint32Kind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedZigZag[int32],
		parsers: []parseKind{
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint32},
			{kind: protowire.BytesType, parser: parsePackedVarint32},
		},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalarMaybeBytes[int64],
		parsers: []parseKind{
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint64},
			{kind: protowire.BytesType, parser: parsePackedVarint64},
		},
	},
	protoreflect.Uint64Kind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalarMaybeBytes[uint64],
		parsers: []parseKind{
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint64},
			{kind: protowire.BytesType, parser: parsePackedVarint64},
		},
	},
	protoreflect.Sint64Kind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedZigZag[int64],
		parsers: []parseKind{
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint64},
			{kind: protowire.BytesType, parser: parsePackedVarint64},
		},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalar[uint32],
		parsers: []parseKind{
			{kind: protowire.Fixed32Type, retry: true, parser: parseRepeatedFixed32},
			{kind: protowire.BytesType, parser: parsePackedFixed32},
		},
	},
	protoreflect.Sfixed32Kind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalar[int32],
		parsers: []parseKind{
			{kind: protowire.Fixed32Type, retry: true, parser: parseRepeatedFixed32},
			{kind: protowire.BytesType, parser: parsePackedFixed32},
		},
	},
	protoreflect.FloatKind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalar[float32],
		parsers: []parseKind{
			{kind: protowire.Fixed32Type, retry: true, parser: parseRepeatedFixed32},
			{kind: protowire.BytesType, parser: parsePackedFixed32},
		},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalar[uint64],
		parsers: []parseKind{
			{kind: protowire.Fixed64Type, retry: true, parser: parseRepeatedFixed64},
			{kind: protowire.BytesType, parser: parsePackedFixed64},
		},
	},
	protoreflect.Sfixed64Kind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalar[int64],
		parsers: []parseKind{
			{kind: protowire.Fixed64Type, retry: true, parser: parseRepeatedFixed64},
			{kind: protowire.BytesType, parser: parsePackedFixed64},
		},
	},
	protoreflect.DoubleKind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalar[float64],
		parsers: []parseKind{
			{kind: protowire.Fixed64Type, retry: true, parser: parseRepeatedFixed64},
			{kind: protowire.BytesType, parser: parsePackedFixed64},
		},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedBool,
		parsers: []parseKind{
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint8},
			{kind: protowire.BytesType, parser: parsePackedVarint8},
		},
	},
	protoreflect.EnumKind: {
		size:   uint32(repSize),
		align:  uint32(repAlign),
		getter: getRepeatedScalar[protoreflect.EnumNumber],
		parsers: []parseKind{
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint32},
			{kind: protowire.BytesType, parser: parsePackedVarint32},
		},
	},

	// String types.
	protoreflect.StringKind: {
		size:    uint32(repSize),
		align:   uint32(repAlign),
		getter:  getRepeatedString,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedUTF8}},
	},
	protoreflect.BytesKind: {
		size:    uint32(repSize),
		align:   uint32(repAlign),
		getter:  getRepeatedBytes,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		size:    uint32(repSize),
		align:   uint32(repAlign),
		bits:    1, // This bit determines whether the field is inlined or pointer mode.
		getter:  getRepeatedMessage,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedMessage}},
	},
	protoreflect.GroupKind: {},
}

func getRepeatedScalar[T scalar](m *message, _ Type, getter getter) protoreflect.Value {
	v := unsafe2.ByteLoad[rep[T]](m, getter.offset.data)
	var raw []T
	switch {
	case !v.isZC():
		raw = v.arena().Raw()
	case v.cap > 0:
		raw = v.zc(m.context.src)
	default:
		return protoreflect.ValueOf(emptyList{})
	}

	return protoreflect.ValueOf(scalarList[T]{raw: raw})
}

func getRepeatedScalarMaybeBytes[T integer](m *message, _ Type, getter getter) protoreflect.Value {
	v := unsafe2.ByteLoad[rep[T]](m, getter.offset.data)
	var raw []T
	switch {
	case !v.isZC():
		raw = v.arena().Raw()
	case v.cap > 0:
		raw := repCast[byte](v).zc(m.context.src)
		return protoreflect.ValueOf(byteScalarList[T]{raw: raw})
	default:
		return protoreflect.ValueOf(emptyList{})
	}

	return protoreflect.ValueOf(scalarList[T]{raw: raw})
}

func getRepeatedZigZag[T integer](m *message, _ Type, getter getter) protoreflect.Value {
	v := unsafe2.ByteLoad[rep[T]](m, getter.offset.data)
	var raw []T
	switch {
	case !v.isZC():
		raw = v.arena().Raw()
	case v.cap > 0:
		raw := repCast[byte](v).zc(m.context.src)
		return protoreflect.ValueOf(byteZigZagList[T]{raw: raw})
	default:
		return protoreflect.ValueOf(emptyList{})
	}

	return protoreflect.ValueOf(zigzagList[T]{raw: raw})
}

func getRepeatedBool(m *message, _ Type, getter getter) protoreflect.Value {
	v := unsafe2.ByteLoad[rep[uint8]](m, getter.offset.data)
	var raw []uint8
	switch {
	case !v.isZC():
		raw = v.arena().Raw()
	case v.cap > 0:
		raw = v.zc(m.context.src)
	default:
		return protoreflect.ValueOf(emptyList{})
	}

	return protoreflect.ValueOf(boolList{raw: raw})
}

func getRepeatedBytes(m *message, _ Type, getter getter) protoreflect.Value {
	raw := unsafe2.ByteLoad[arena.Slice[zc]](m, getter.offset.data).Raw()
	if raw == nil {
		return protoreflect.ValueOf(emptyList{})
	}

	return protoreflect.ValueOf(bytesList{raw: raw, shared: m.context})
}

func getRepeatedString(m *message, _ Type, getter getter) protoreflect.Value {
	raw := unsafe2.ByteLoad[arena.Slice[zc]](m, getter.offset.data).Raw()
	if raw == nil {
		return protoreflect.ValueOf(emptyList{})
	}

	return protoreflect.ValueOf(stringList{raw: raw, shared: m.context})
}

func getRepeatedMessage(m *message, _ Type, getter getter) protoreflect.Value {
	if m.getBit(getter.offset.bit) {
		raw := unsafe2.ByteLoad[arena.Slice[*message]](m, getter.offset.data)
		return protoreflect.ValueOf(messageList{raw: raw.Raw()})
	}

	raw := unsafe2.ByteLoad[arena.Slice[byte]](m, getter.offset.data)
	if raw.Ptr() == nil {
		return protoreflect.ValueOf(emptyList{})
	}

	// Get the type from the first element of the list.
	first := unsafe2.Cast[message](raw.Ptr())

	return protoreflect.ValueOf(inlineMessageList{
		ty:    first.ty,
		raw:   first,
		dummy: make([]struct{}, raw.Len()),
	})
}

//go:nosplit
//fastpb:stencil appendVarint8 appendVarint[uint8] spillArena -> spillArena8
//fastpb:stencil appendVarint32 appendVarint[uint32] spillArena -> spillArena32
//fastpb:stencil appendVarint64 appendVarint[uint64] spillArena -> spillArena64
func appendVarint[T integer](p1 parser1, p2 parser2, v T) (parser1, parser2) {
	rep := unsafe2.Cast[rep[T]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))

	// Check if we're already an arena, or an empty repeated field which looks like
	// an empty arena slice.
	if rep.isZC() && rep.cap > 0 {
		// Already holds a borrow. Need to spill to arena.
		// This is the worst-case scenario.
		zc := repCast[byte](*rep).zc(p1.c().src)
		slice := arena.NewSlice[T](p1.arena(), len(zc)+1)
		for i, b := range zc {
			unsafe2.Store(slice.Ptr(), i, T(b))
		}
		unsafe2.Store(slice.Ptr(), slice.Len(), v)
		rep.setArena(slice)
		return p1, p2
	}

	if rep.len < rep.cap {
		unsafe2.Store(rep.ptr.AssertValid(), rep.len, v)
		rep.len++
		return p1, p2
	}

	rep.setArena(rep.arena().AppendOne(p1.arena(), v))
	return p1, p2
}

//go:nosplit
//fastpb:stencil appendFixed8 appendFixed[uint8] spillArena -> spillArena8
//fastpb:stencil appendFixed32 appendFixed[uint32] spillArena -> spillArena32
//fastpb:stencil appendFixed64 appendFixed[uint64] spillArena -> spillArena64
func appendFixed[T any](p1 parser1, p2 parser2, v T) (parser1, parser2) {
	rep := unsafe2.Cast[rep[T]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))

	// Check if we're already an arena, or an empty repeated field which looks like
	// an empty arena slice.
	if rep.isZC() && rep.cap > 0 {
		// Already holds a borrow. Need to spill to arena.
		// This is the worst-case scenario.
		p1, p2, rep = spillArena(p1, p2, rep)
	}

	if rep.len < rep.cap {
		unsafe2.Store(rep.ptr.AssertValid(), rep.len, v)
		rep.len++
		return p1, p2
	}

	rep.setArena(rep.arena().AppendOne(p1.arena(), v))
	return p1, p2
}

//go:nosplit
//fastpb:stencil spillArena8 spillArena[uint8]
//fastpb:stencil spillArena32 spillArena[uint32]
//fastpb:stencil spillArena64 spillArena[uint64]
func spillArena[E any](p1 parser1, p2 parser2, rep *rep[E]) (parser1, parser2, *rep[E]) {
	slice := rep.zc(p1.c().src)
	rep.setArena(arena.SliceOf(p1.arena(), slice...))
	return p1, p2, rep
}

//go:nosplit
//fastpb:stencil parseRepeatedVarint8 parseRepeatedVarint[uint8] appendVarint -> appendVarint8
//fastpb:stencil parseRepeatedVarint32 parseRepeatedVarint[uint32] appendVarint -> appendVarint32
//fastpb:stencil parseRepeatedVarint64 parseRepeatedVarint[uint64] appendVarint -> appendVarint64
func parseRepeatedVarint[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	p1, p2 = appendVarint(p1, p2, T(n))

	return p1, p2
}

//go:nosplit
//fastpb:stencil parsePackedVarint8 parsePackedVarint[uint8]
//fastpb:stencil parsePackedVarint32 parsePackedVarint[uint32]
//fastpb:stencil parsePackedVarint64 parsePackedVarint[uint64]
func parsePackedVarint[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.lengthPrefix(p2)
	if n == 0 {
		return p1, p2
	}

	p2.scratch = uint64(p1.e_)
	p1.e_ = p1.b_.Add(int(n))

	// Count the number of varints in this packed field. We do this by counting
	// bytes without the sign bit set, in groups of 8.
	var count uint32
	for p := p1.b_; p < p1.e_; p += 8 {
		n := min(8, p1.e_-p)
		bytes := *unsafe2.Cast[uint64](p.AssertValid())
		bytes |= signBits << (n * 8)
		count += uint32(bits.OnesCount64(signBits &^ bytes))
	}

	rep := unsafe2.Cast[rep[T]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
	if rep.isZC() {
		// Check if we're already on-arena, or an empty repeated field which looks
		// like an empty arena slice.
		switch {
		case rep.cap > 0:
			// Already holds a borrow. Need to spill to arena.
			// This is the worst-case scenario.
			zc := repCast[byte](*rep).zc(p1.c().src)
			slice := arena.NewSlice[T](p1.arena(), len(zc)+int(count))
			for i, b := range zc {
				unsafe2.Store(slice.Ptr(), i, T(b))
			}
			rep.setArena(slice)

		case count == n:
			offset := p1.b_.Sub(unsafe2.AddrOf(p1.c().src))

			rep.setZC(zc{uint32(offset), n})
			p1.b_ = p1.e_
			p1.e_ = unsafe2.Addr[byte](p2.scratch)
			return p1, p2

		default:
			rep.setArena(rep.arena().Grow(p1.arena(), int(count)))
		}
	} else if spare := rep.cap - rep.len; spare < count {
		rep.setArena(rep.arena().Grow(p1.arena(), int(count-spare)))
	}

	// Manual inlining of AppendOne. Previously, we called AppendOne, but
	// Go would not inline it, which resulted in a lot of spilling in a hot
	// loop!
	p := rep.ptr.Add(int(rep.len))
	// There are three variants of this loop: one for the cases where every
	// varint is small (one byte; common). One for the cases where most varints
	// are small (so the special-case branches are likely to be well-predicted)
	// and when many varints are large so the aforementioned branches would
	// not be predicted well.
	switch {
	case count == uint32(p1.len()):
		for {
			*p.AssertValid() = T(*p1.b())
			p1.b_++
			p = p.Add(1)

			if p1.b_ != p1.e_ {
				continue
			}

			break
		}
	case count >= uint32(p1.len())/2:
		for {
			var x uint64
			if v := *p1.b(); int8(v) >= 0 {
				x = uint64(v)
				p1.b_++
			} else if c := p1.b_.Add(1); c != p1.e_ && int8(*c.AssertValid()) >= 0 {
				x = uint64(*p1.b()&0x7f) | uint64(*c.AssertValid())<<7
				p1.b_ += 2
			} else {
				p1, p2, x = p1.varint(p2)
			}

			*p.AssertValid() = T(x)
			p = p.Add(1)
			if p1.b_ != p1.e_ {
				continue
			}

			break
		}
	default:
		for {
			var x uint64
			p1, p2, x = p1.varint(p2)

			*p.AssertValid() = T(x)
			p = p.Add(1)
			if p1.b_ != p1.e_ {
				continue
			}

			break
		}
	}

	rep.len = uint32(p.Sub(rep.ptr))
	p1.e_ = unsafe2.Addr[byte](p2.scratch)
	return p1, p2
}

//go:nosplit
func parseRepeatedFixed32(p1 parser1, p2 parser2) (parser1, parser2) {
	return appendFixed32(p1.fixed32(p2))
}

//go:nosplit
func parseRepeatedFixed64(p1 parser1, p2 parser2) (parser1, parser2) {
	return appendFixed64(p1.fixed64(p2))
}

//go:nosplit
//fastpb:stencil parsePackedFixed32 parsePackedFixed[uint32]
//fastpb:stencil parsePackedFixed64 parsePackedFixed[uint64]
func parsePackedFixed[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var zc zc
	p1, p2, zc = p1.bytes(p2)

	size, _ := unsafe2.Layout[T]()
	if int(zc.len)%size != 0 {
		p1.fail(p2, errCodeTruncated)
	}

	rep := unsafe2.Cast[rep[T]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))

	switch {
	case !rep.isZC():
		// Already on an arena.
	case rep.cap == 0:
		// Empty repeated field. We can just shove the zc here.
		// This is the best-case scenario.
		rep.setZC(zc)
		goto exit
	default:
		// Already holds a borrow. Need to spill to arena.
		// This is the worst-case scenario.
		p1, p2, rep = spillArena(p1, p2, rep)
	}

	{
		size, _ := unsafe2.Layout[T]()
		borrowed := unsafe2.Slice(
			unsafe2.Cast[T](unsafe2.Add(p1.c().src, zc.offset)),
			int(zc.len)/size,
		)

		slice := rep.arena()
		slice = slice.Append(p1.arena(), borrowed...)
		rep.setArena(slice)
	}

exit:
	return p1, p2
}

//go:nosplit
func parseRepeatedBytes(p1 parser1, p2 parser2) (parser1, parser2) {
	var v zc
	p1, p2, v = p1.bytes(p2)

	rep := unsafe2.Cast[rep[zc]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
	rep.setArena(rep.arena().AppendOne(p1.arena(), v))

	return p1, p2
}

//go:nosplit
func parseRepeatedUTF8(p1 parser1, p2 parser2) (parser1, parser2) {
	var v zc
	p1, p2, v = p1.utf8(p2)

	rep := unsafe2.Cast[rep[zc]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
	rep.setArena(rep.arena().AppendOne(p1.arena(), v))

	return p1, p2
}

//go:nosplit
func parseRepeatedMessage(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.lengthPrefix(p2)

	var (
		r *rep[byte] = unsafe2.Cast[rep[byte]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
		m *message
	)
	if p2.m().getBit(p2.f().offset.bit) {
		goto outlined
	}

	{
		size := p2.f().message.ty.raw.size
		if r.ptr == 0 {
			p1, p2, r = newInlineRepeatedField(p1, p2, r)
		} else if r.len == r.cap {
			p1.log(p2, "repeated message spill", "%v[%d:%d]", r.ptr, r.len, r.cap)
			p1, p2, r = spillInlineRepeatedField(p1, p2, r)

			goto outlined
		}

		p := unsafe2.Add(r.ptr.AssertValid(), r.len*size)
		r.len++

		p1.log(p2, "inline repeated message", "%v[%d:%d], %p/%d", r.ptr, r.len, r.cap, p, size)
		p1, p2, m = p1.allocInPlace(p2, p)
		goto exit
	}

outlined:
	{
		p1, p2, m = p1.alloc(p2)

		r := unsafe2.Cast[rep[unsafe2.Addr[message]]](r)
		p1.log(p2, "outline repeated message", "%v[%d:%d], %p/", r.ptr, r.len, r.cap, m)
		if r.len == r.cap {
			p1, p2, m = appendOneMessage(p1, p2, m)
			goto exit
		}

		*unsafe2.Add(r.ptr.AssertValid(), r.len) = unsafe2.AddrOf(m)
		r.len++
	}

exit:
	return p1.message(p2, int(n), m)
}

//go:noinline
func newInlineRepeatedField(p1 parser1, p2 parser2, r *rep[byte]) (parser1, parser2, *rep[byte]) {
	// First element of this field. Allocate a byte array large enough to
	// hold one element.
	//
	// TODO: Add a profiling knob for setting the default number of
	// elements.
	size := p2.f().message.ty.raw.size
	s := arena.NewSlice[byte](p1.arena(), int(size))
	r.ptr = unsafe2.AddrOf(s.Ptr())
	r.cap = uint32(s.Cap()) / size

	return p1, p2, r
}

//go:noinline
func spillInlineRepeatedField(p1 parser1, p2 parser2, r *rep[byte]) (parser1, parser2, *rep[byte]) {
	size := p2.f().message.ty.raw.size

	// Spill all of the messages onto a pointer slice.
	s := arena.NewSlice[*message](p1.arena(), int(r.cap)*2)
	for i := range r.len {
		unsafe2.StoreNoWB(unsafe2.Add(s.Ptr(), i),
			unsafe2.Cast[message](unsafe2.ByteAdd(r.ptr.AssertValid(), int(i*size))))
	}

	r.ptr = unsafe2.Addr[byte](unsafe2.AddrOf(s.Ptr()))
	r.cap = uint32(s.Cap())
	p2.m().setBit(p2.f().offset.bit, true) // Mark this as an outlined message.

	return p1, p2, r
}

//go:noinline
func appendOneMessage(p1 parser1, p2 parser2, m *message) (parser1, parser2, *message) {
	r := unsafe2.Cast[rep[unsafe2.Addr[message]]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
	r.setArena(r.arena().AppendOne(p1.arena(), unsafe2.AddrOf(m)))
	return p1, p2, m
}
