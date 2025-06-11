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
	"math/bits"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/arena/slice"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
	"github.com/bufbuild/fastpb/internal/zc"
)

// Repeated fields are implemented as one an slice.Slice for some element
// type, with various optimizations to avoid materializing a slice in cases
// where we can zero-copy.
//
// If the pointer part of an slice.Slice is nil, that means that its length
// and capacity are actually the contents of a [zc], and the slice is actually
// a zero-copy alias of the input buffer. This is most notable for fixed-size
// fields, which we can almost always zero-copy.
//
// Packed varint fields also make this optimization, but only when every element
// of the packed field is one byte long. There is a special list implementation
// that handles this case.

var repeatedFields = map[protoreflect.Kind]*archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		layout: layout.Of[repeatedScalar[byte, int32]](),
		getter: getRepeatedScalar[byte, int32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint32},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint32},
		},
	},
	protoreflect.Uint32Kind: {
		layout: layout.Of[repeatedScalar[byte, uint32]](),
		getter: getRepeatedScalar[byte, uint32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint32},
			{kind: protowire.VarintType, parser: parseRepeatedVarint32},
		},
	},
	protoreflect.Sint32Kind: {
		layout: layout.Of[repeatedZigzag[byte, uint32]](),
		getter: getRepeatedZigzag[byte, int32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint32},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint32},
		},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		layout: layout.Of[repeatedScalar[byte, int64]](),
		getter: getRepeatedScalar[byte, int64],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint64},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint64},
		},
	},
	protoreflect.Uint64Kind: {
		layout: layout.Of[repeatedScalar[byte, uint64]](),
		getter: getRepeatedScalar[byte, uint64],
		parsers: []parseKind{
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint64},
			{kind: protowire.BytesType, parser: parsePackedVarint64},
		},
	},
	protoreflect.Sint64Kind: {
		layout: layout.Of[repeatedZigzag[byte, int64]](),
		getter: getRepeatedZigzag[byte, int64],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint64},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint64},
		},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		layout: layout.Of[repeatedScalar[uint32, uint32]](),
		getter: getRepeatedScalar[uint32, uint32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed32},
			{kind: protowire.Fixed32Type, retry: true, parser: parseRepeatedFixed32},
		},
	},
	protoreflect.Sfixed32Kind: {
		layout: layout.Of[repeatedScalar[int32, int32]](),
		getter: getRepeatedScalar[int32, int32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed32},
			{kind: protowire.Fixed32Type, retry: true, parser: parseRepeatedFixed32},
		},
	},
	protoreflect.FloatKind: {
		layout: layout.Of[repeatedScalar[float32, float32]](),
		getter: getRepeatedScalar[float32, float32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed32},
			{kind: protowire.Fixed32Type, retry: true, parser: parseRepeatedFixed32},
		},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		layout: layout.Of[repeatedScalar[uint64, uint64]](),
		getter: getRepeatedScalar[uint64, uint64],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed64},
			{kind: protowire.Fixed64Type, retry: true, parser: parseRepeatedFixed64},
		},
	},
	protoreflect.Sfixed64Kind: {
		layout: layout.Of[repeatedScalar[int64, int64]](),
		getter: getRepeatedScalar[int64, int64],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed64},
			{kind: protowire.Fixed64Type, retry: true, parser: parseRepeatedFixed64},
		},
	},
	protoreflect.DoubleKind: {
		layout: layout.Of[repeatedScalar[float64, float64]](),
		getter: getRepeatedScalar[float64, float64],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed64},
			{kind: protowire.Fixed64Type, retry: true, parser: parseRepeatedFixed64},
		},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		layout: layout.Of[repeatedBool](),
		getter: getRepeatedBool,
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint8},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint8},
		},
	},
	protoreflect.EnumKind: {
		layout: layout.Of[repeatedScalar[byte, protoreflect.EnumNumber]](),
		getter: getRepeatedScalar[byte, protoreflect.EnumNumber],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint32},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint32},
		},
	},

	// String types.
	protoreflect.StringKind: {
		layout:  layout.Of[repeatedString](),
		getter:  getRepeatedString,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedUTF8}},
	},
	proto2StringKind: {
		layout:  layout.Of[repeatedString](),
		getter:  getRepeatedString,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedBytes}},
	},
	protoreflect.BytesKind: {
		layout:  layout.Of[repeatedBytes](),
		getter:  getRepeatedBytes,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		layout:  layout.Of[repeatedMessage](),
		getter:  getRepeatedMessage,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedMessage}},
	},
	protoreflect.GroupKind: {},
}

type repeatedScalarElement interface {
	integer | ~float32 | ~float64
}

func getRepeatedScalar[Z, E repeatedScalarElement](m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[repeatedScalar[Z, E]](m, getter.offset)
	return protoreflect.ValueOf(p)
}

type repeatedScalar[Z, E repeatedScalarElement] struct {
	immutableList
	raw slice.Untyped
}

// IsValid implements [protoreflect.List].
func (r *repeatedScalar[_, _]) IsValid() bool { return r != nil }

// Len implements [protoreflect.List].
func (r *repeatedScalar[_, _]) Len() int {
	if r == nil {
		return 0
	}
	return int(r.raw.Len)
}

// Get implements [protoreflect.List].
func (r *repeatedScalar[Z, E]) Get(n int) protoreflect.Value {
	raw := r.raw
	if raw.OffArena() {
		v := slice.CastUntyped[Z](raw).Raw()[n]
		return protoreflect.ValueOf(E(v))
	}
	v := slice.CastUntyped[E](raw).Raw()[n]
	return protoreflect.ValueOf(v)
}

func getRepeatedZigzag[Z, E integer](m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[repeatedZigzag[Z, E]](m, getter.offset)
	return protoreflect.ValueOf(p)
}

type repeatedZigzag[Z, E integer] struct {
	immutableList
	raw slice.Untyped
}

// IsValid implements [protoreflect.List].
func (r *repeatedZigzag[_, _]) IsValid() bool { return r != nil }

// Len implements [protoreflect.List].
func (r *repeatedZigzag[_, _]) Len() int {
	if r == nil {
		return 0
	}
	return int(r.raw.Len)
}

// Get implements [protoreflect.List].
func (r *repeatedZigzag[Z, E]) Get(n int) protoreflect.Value {
	raw := r.raw
	if raw.Ptr.SignBit() {
		v := slice.CastUntyped[Z](raw).Raw()[n]
		return protoreflect.ValueOf(zigzag(E(v)))
	}
	v := slice.CastUntyped[E](raw).Raw()[n]
	return protoreflect.ValueOf(zigzag(v))
}

func getRepeatedBool(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[repeatedString](m, getter.offset)
	return protoreflect.ValueOf(p)
}

type repeatedBool struct {
	immutableList
	raw slice.Addr[byte]
}

// IsValid implements [protoreflect.List].
func (r *repeatedBool) IsValid() bool { return r != nil }

// Len implements [protoreflect.List].
func (r *repeatedBool) Len() int {
	if r == nil {
		return 0
	}
	return int(r.raw.Len)
}

// Get implements [protoreflect.List].
func (r *repeatedBool) Get(n int) protoreflect.Value {
	v := r.raw.AssertValid().Raw()[n]
	return protoreflect.ValueOf(v != 0)
}

func getRepeatedString(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[repeatedString](m, getter.offset)
	return protoreflect.ValueOf(p)
}

// repeatedString is a [protoreflect.List] implementation for string.
type repeatedString struct {
	immutableList
	src *byte
	raw slice.Addr[zc.Range]
}

// IsValid implements [protoreflect.List].
func (r *repeatedString) IsValid() bool { return r != nil }

func (r *repeatedString) Len() int {
	if r == nil {
		return 0
	}
	return int(r.raw.Len)
}

func (r *repeatedString) Get(n int) protoreflect.Value {
	zc := r.raw.AssertValid().Raw()[n]
	return protoreflect.ValueOf(zc.String(r.src))
}

func getRepeatedBytes(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[repeatedBytes](m, getter.offset)
	return protoreflect.ValueOf(p)
}

// repeatedBytes is a [protoreflect.List] implementation for string.
type repeatedBytes struct {
	immutableList
	src *byte
	raw slice.Addr[zc.Range]
}

// IsValid implements [protoreflect.List].
func (r *repeatedBytes) IsValid() bool { return r != nil }

func (r *repeatedBytes) Len() int {
	if r == nil {
		return 0
	}
	return int(r.raw.Len)
}

func (r *repeatedBytes) Get(n int) protoreflect.Value {
	zc := r.raw.AssertValid().Raw()[n]
	return protoreflect.ValueOf(zc.Bytes(r.src))
}

//go:nosplit
//fastpb:stencil parseRepeatedVarint8 parseRepeatedVarint[uint8] appendVarint -> appendVarint8
//fastpb:stencil parseRepeatedVarint32 parseRepeatedVarint[uint32] appendVarint -> appendVarint32
//fastpb:stencil parseRepeatedVarint64 parseRepeatedVarint[uint64] appendVarint -> appendVarint64
func parseRepeatedVarint[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)

	var r *repeatedScalar[byte, T]
	p1, p2, r = getMutableField[repeatedScalar[byte, T]](p1, p2)
	p1.log(p2, "slot", "%v", r.raw)

	// Check if we're already an arena, or an empty repeated field which looks like
	// an empty arena slice.
	if r.raw.OffArena() {
		borrow := slice.CastUntyped[byte](r.raw).Raw()
		s := slice.Make[T](p1.arena(), len(borrow)+1)
		for i, b := range borrow {
			s.Store(i, T(b))
		}
		s.Store(s.Len()-1, T(n))
		p1.log(p2, "spill", "%v->%v %v", r.raw, s.Addr(), s)

		r.raw = s.Addr().Untyped()
		return p1, p2
	}

	s := slice.CastUntyped[T](r.raw)
	if s.Len() < s.Cap() {
		s = s.SetLen(s.Len() + 1)
		s.Store(s.Len()-1, T(n))

		p1.log(p2, "store", "%v %v", s.Addr(), s)
		r.raw = s.Addr().Untyped()
		return p1, p2
	}

	s = s.AppendOne(p1.arena(), T(n))
	p1.log(p2, "append", "%v %v", s.Addr(), s)
	r.raw = s.Addr().Untyped()
	return p1, p2
}

//go:nosplit
//fastpb:stencil parsePackedVarint8 parsePackedVarint[uint8]
//fastpb:stencil parsePackedVarint32 parsePackedVarint[uint32]
//fastpb:stencil parsePackedVarint64 parsePackedVarint[uint64]
func parsePackedVarint[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var n int
	p1, p2, n = p1.lengthPrefix(p2)
	if n == 0 {
		return p1, p2
	}

	p2.scratch = uint64(p1.e_)
	p1.e_ = p1.b_.Add(n)

	// Count the number of varints in this packed field. We do this by counting
	// bytes without the sign bit set, in groups of 8.
	var count int
	for p := p1.b_; p < p1.e_; p += 8 {
		n := min(8, p1.e_-p)
		bytes := *unsafe2.Cast[uint64](p.AssertValid())
		bytes |= signBits << (n * 8)
		count += bits.OnesCount64(signBits &^ bytes)
	}

	var r *repeatedScalar[byte, T]
	p1, p2, r = getMutableField[repeatedScalar[byte, T]](p1, p2)
	var s slice.Slice[T]
	switch {
	case r.raw.Ptr == 0:
		if count == n {
			r.raw = slice.OffArena(p1.b(), n)
			p1.log(p2, "zc", "%v", r.raw)

			p1.b_ = p1.e_
			p1.e_ = unsafe2.Addr[byte](p2.scratch)
			return p1, p2
		}
		s = s.Grow(p1.arena(), count)
		p1.log(p2, "grow", "%v %v", s.Addr(), s)

	case r.raw.OffArena():
		// Already holds a borrow. Need to spill to arena.
		// This is the worst-case scenario.
		borrow := slice.CastUntyped[byte](r.raw).Raw()
		s = slice.Make[T](p1.arena(), len(borrow)+count)
		for i, b := range borrow {
			s.Store(i, T(b))
		}
		s = s.SetLen(len(borrow))

		p1.log(p2, "spill", "%v->%v %v", r.raw, s.Addr(), s)

	default:
		s = slice.CastUntyped[T](r.raw)
		if spare := s.Cap() - s.Len(); spare < count {
			s = s.Grow(p1.arena(), count-spare)
			p1.log(p2, "grow", "%v %v, %d", s.Addr(), s, spare)
		}
	}

	p := unsafe2.AddrOf(s.Ptr()).Add(s.Len())
	p1.log(p2, "store at", "%v", p)

	// There are three variants of this loop: one for the cases where every
	// varint is small (one byte; common). One for the cases where most varints
	// are small (so the special-case branches are likely to be well-predicted)
	// and when many varints are large so the aforementioned branches would
	// not be predicted well.
	switch {
	case count == p1.len():
		for {
			*p.AssertValid() = T(*p1.b())
			p1.b_++
			p = p.Add(1)

			if p1.b_ != p1.e_ {
				continue
			}

			break
		}
	case count >= p1.len()/2:
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

	s = s.SetLen(p.Sub(unsafe2.AddrOf(s.Ptr())))
	p1.log(p2, "append", "%v %v", s.Addr(), s)

	r.raw = s.Addr().Untyped()
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
//fastpb:stencil appendFixed32 appendFixed[uint32] spillArena -> spillArena32
//fastpb:stencil appendFixed64 appendFixed[uint64] spillArena -> spillArena64
func appendFixed[T uint32 | uint64](p1 parser1, p2 parser2, v T) (parser1, parser2) {
	var r *repeatedScalar[T, T]
	p1, p2, r = getMutableField[repeatedScalar[T, T]](p1, p2)
	s := slice.CastUntyped[T](r.raw)

	if s.Len() < s.Cap() {
		s = s.SetLen(s.Len() + 1)
		s.Store(s.Len()-1, v)
		p1.log(p2, "repeated fixed store", "%v %v", s.Addr(), s)

		r.raw = s.Addr().Untyped()
		return p1, p2
	}

	s = s.AppendOne(p1.arena(), v)
	p1.log(p2, "repeated fixed append", "%v %v", s.Addr(), s)
	r.raw = s.Addr().Untyped()
	return p1, p2
}

//go:nosplit
//fastpb:stencil parsePackedFixed32 parsePackedFixed[uint32]
//fastpb:stencil parsePackedFixed64 parsePackedFixed[uint64]
func parsePackedFixed[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var n int
	p1, p2, n = p1.lengthPrefix(p2)
	if n == 0 {
		return p1, p2
	}

	size := layout.Size[T]()
	if n%size != 0 {
		p1.fail(p2, errCodeTruncated)
	}

	var r *repeatedScalar[T, T]
	p1, p2, r = getMutableField[repeatedScalar[T, T]](p1, p2)

	if r.raw.Ptr == 0 {
		// Empty repeated field. We can just shove the zc here.
		// This is the best-case scenario.
		r.raw = slice.OffArena(p1.b(), n/size)
		if dbg.Enabled {
			p1.log(p2, "zc", "%v, %v", r.raw, slice.CastUntyped[T](r.raw))
		}

		p1 = p1.advance(n)
		goto exit
	}

	// If r.raw is off-arena, it will have len==cap, so it will force a
	// realloc when we call Append.

	{
		s := slice.CastUntyped[T](r.raw)
		size := layout.Size[T]()
		borrowed := unsafe2.Slice(unsafe2.Cast[T](p1.b()), n/size)
		if dbg.Enabled {
			p1.log(p2, "appending", "%v, %v", borrowed, s.Raw())
		}

		p1 = p1.advance(n)

		s = s.Append(p1.arena(), borrowed...)
		r.raw = s.Addr().Untyped()
		if dbg.Enabled {
			p1.log(p2, "append", "%v, %v", r.raw, s.Raw())
		}
	}

exit:
	return p1, p2
}

//go:nosplit
func parseRepeatedBytes(p1 parser1, p2 parser2) (parser1, parser2) {
	var v zc.Range
	p1, p2, v = p1.bytes(p2)

	var r *repeatedBytes
	p1, p2, r = getMutableField[repeatedBytes](p1, p2)
	r.raw = r.raw.AssertValid().AppendOne(p1.arena(), v).Addr()
	unsafe2.StoreNoWB(&r.src, p1.c().src)

	return p1, p2
}

//go:nosplit
func parseRepeatedUTF8(p1 parser1, p2 parser2) (parser1, parser2) {
	var v zc.Range
	p1, p2, v = p1.utf8(p2)

	var r *repeatedString
	p1, p2, r = getMutableField[repeatedString](p1, p2)
	r.raw = r.raw.AssertValid().AppendOne(p1.arena(), v).Addr()
	unsafe2.StoreNoWB(&r.src, p1.c().src)

	return p1, p2
}

// immutableList implements the mutation operations of a [protoreflect.List]
// by panicking.
type immutableList struct{}

func (immutableList) Append(protoreflect.Value)         { panic(dbg.Unsupported()) }
func (immutableList) AppendMutable() protoreflect.Value { panic(dbg.Unsupported()) }
func (immutableList) NewElement() protoreflect.Value    { panic(dbg.Unsupported()) }
func (immutableList) Set(int, protoreflect.Value)       { panic(dbg.Unsupported()) }
func (immutableList) Truncate(int)                      { panic(dbg.Unsupported()) }
