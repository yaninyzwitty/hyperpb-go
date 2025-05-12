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
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/unsafe2"
)

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
	//
	// If this is a oneof field, this is instead an offset into the
	// oneof word table.
	bit uint32

	// Byte offset within the containing message to the data for this field.
	data uint32

	// This field's number. Only used by oneof fields; all other fields have
	// a zero here.
	number uint32
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

	// Set if this is a oneof field.
	oneof bool

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
	od := fd.ContainingOneof()
	switch {
	case fd.Cardinality() == protoreflect.Repeated:
		a = &repeatedFields[fd.Kind()]
	case od != nil && od.Fields().Len() > 1:
		// One-element oneofs are treated like optional fields.
		a = &oneofFields[fd.Kind()]
	case fd.HasPresence():
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

func storeField[T any](p1 parser1, p2 parser2, v T) {
	unsafe2.ByteStore(p2.m(), p2.f().offset.data, v)
}
