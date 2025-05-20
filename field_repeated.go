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

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Repeated fields are implemented as one an arena.Slice for some element
// type, with various optimizations to avoid materializing a slice in cases
// where we can zero-copy.
//
// If the pointer part of an arena.Slice is nil, that means that its length
// and capacity are actually the contents of a [zc], and the slice is actually
// a zero-copy alias of the input buffer. This is most notable for fixed-size
// fields, which we can almost always zero-copy.
//
// Packed varint fields also make this optimization, but only when every element
// of the packed field is one byte long. There is a special list implementation
// that handles this case.

// isZC returns whether a slice is secretly a [zc].
func isZC[T any](slice arena.Slice[T]) bool {
	return slice.Ptr() == nil
}

// wrapZC wraps a [zc] up as an arena slice.
func wrapZC[T any](zc zc) arena.Slice[T] {
	return arena.SliceFromParts[T](nil, uint32(zc.start()), uint32(zc.len()))
}

// unwrapRawZC is like unwrapZC, but it does not dereference the zc into a
// slice.
func unwrapRawZC[T any](slice arena.Slice[T]) zc {
	return newRawZC(slice.Len(), slice.Cap())
}

// unwrapZC unwraps a slice that is secretly a [zc], and uses it to obtain a
// slice of T values from src.
func unwrapZC[T any](slice arena.Slice[T], src *byte) []T {
	size, _ := unsafe2.Layout[T]()
	// This is a borrow from src, and len and cap are a zc. Note that both
	// are denominated in bytes in this mode.
	return unsafe2.Slice(
		unsafe2.Cast[T](unsafe2.Add(src, slice.Len())),
		slice.Cap()/size,
	)
}

var repeatedFields = map[protoreflect.Kind]*archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalarMaybeBytes[int32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint32},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint32},
		},
	},
	protoreflect.Uint32Kind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalarMaybeBytes[uint32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint32},
			{kind: protowire.VarintType, parser: parseRepeatedVarint32},
		},
	},
	protoreflect.Sint32Kind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedZigZag[int32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint32},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint32},
		},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalarMaybeBytes[int64],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint64},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint64},
		},
	},
	protoreflect.Uint64Kind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalarMaybeBytes[uint64],
		parsers: []parseKind{
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint64},
			{kind: protowire.BytesType, parser: parsePackedVarint64},
		},
	},
	protoreflect.Sint64Kind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedZigZag[int64],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint64},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint64},
		},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalar[uint32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed32},
			{kind: protowire.Fixed32Type, retry: true, parser: parseRepeatedFixed32},
		},
	},
	protoreflect.Sfixed32Kind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalar[int32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed32},
			{kind: protowire.Fixed32Type, retry: true, parser: parseRepeatedFixed32},
		},
	},
	protoreflect.FloatKind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalar[float32],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed32},
			{kind: protowire.Fixed32Type, retry: true, parser: parseRepeatedFixed32},
		},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalar[uint64],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed64},
			{kind: protowire.Fixed64Type, retry: true, parser: parseRepeatedFixed64},
		},
	},
	protoreflect.Sfixed64Kind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalar[int64],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed64},
			{kind: protowire.Fixed64Type, retry: true, parser: parseRepeatedFixed64},
		},
	},
	protoreflect.DoubleKind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalar[float64],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedFixed64},
			{kind: protowire.Fixed64Type, retry: true, parser: parseRepeatedFixed64},
		},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedBool,
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint8},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint8},
		},
	},
	protoreflect.EnumKind: {
		size:   uint32(arena.SliceSize),
		align:  uint32(arena.SliceAlign),
		getter: getRepeatedScalar[protoreflect.EnumNumber],
		parsers: []parseKind{
			{kind: protowire.BytesType, parser: parsePackedVarint32},
			{kind: protowire.VarintType, retry: true, parser: parseRepeatedVarint32},
		},
	},

	// String types.
	protoreflect.StringKind: {
		size:    uint32(arena.SliceSize),
		align:   uint32(arena.SliceAlign),
		getter:  getRepeatedString,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedUTF8}},
	},
	proto2StringKind: {
		size:    uint32(arena.SliceSize),
		align:   uint32(arena.SliceAlign),
		getter:  getRepeatedString,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedBytes}},
	},
	protoreflect.BytesKind: {
		size:    uint32(arena.SliceSize),
		align:   uint32(arena.SliceAlign),
		getter:  repeatedBytes,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		size:    uint32(arena.SliceSize),
		align:   uint32(arena.SliceAlign),
		bits:    1, // This bit determines whether the field is inlined or pointer mode.
		getter:  getRepeatedMessage,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseRepeatedMessage}},
	},
	protoreflect.GroupKind: {},
}

func getRepeatedScalar[T scalar](m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[arena.Slice[T]](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(repeatedScalar[T]{raw: nil})
	}

	v := *p
	var raw []T
	if isZC(v) {
		raw = unwrapZC(v, m.context.src)
	} else {
		raw = v.Raw()
	}
	return protoreflect.ValueOf(repeatedScalar[T]{raw: raw})
}

// repeatedScalar is a [protoreflect.List] implementation for non-bool scalar
// types.
type repeatedScalar[E any] struct {
	unimplementedList
	raw []E
}

var _ protoreflect.List = repeatedScalar[int32]{}

func (l repeatedScalar[E]) Len() int { return len(l.raw) }
func (l repeatedScalar[E]) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(l.raw[n])
}

func getRepeatedScalarMaybeBytes[T integer](m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[arena.Slice[T]](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(repeatedScalar[T]{raw: nil})
	}

	v := *p
	if isZC(v) {
		raw := unwrapRawZC(v).bytes(m.context.src)
		return protoreflect.ValueOf(repeatedScalarBytes[T]{raw: raw})
	}

	return protoreflect.ValueOf(repeatedScalar[T]{raw: v.Raw()})
}

// repeatedScalarBytes is a [protoreflect.List] implementation for non-bool scalar
// types, where each value fits in a single byte.
type repeatedScalarBytes[E integer] struct {
	unimplementedList
	raw []byte
}

var _ protoreflect.List = repeatedScalar[int32]{}

func (l repeatedScalarBytes[E]) Len() int { return len(l.raw) }
func (l repeatedScalarBytes[E]) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(E(l.raw[n]))
}

func getRepeatedZigZag[T integer](m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[arena.Slice[T]](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(repeatedZigZag[T]{raw: nil})
	}

	v := *p
	if isZC(v) {
		raw := unwrapRawZC(v).bytes(m.context.src)
		return protoreflect.ValueOf(repeatedZigZagBytes[T]{raw: raw})
	}

	return protoreflect.ValueOf(repeatedZigZag[T]{raw: v.Raw()})
}

// scalarList is a [protoreflect.List] implementation for integer types that
// zig-zag decodes them on-demand.
type repeatedZigZag[E integer] struct {
	unimplementedList
	raw []E
}

func (l repeatedZigZag[E]) Len() int { return len(l.raw) }
func (l repeatedZigZag[E]) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(zigzag(l.raw[n]))
}

// repeatedZigZagBytes is a zigzag version of byteScalarList.
type repeatedZigZagBytes[E integer] struct {
	unimplementedList
	raw []byte
}

func (l repeatedZigZagBytes[E]) Len() int { return len(l.raw) }
func (l repeatedZigZagBytes[E]) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(zigzag(E(l.raw[n])))
}

func getRepeatedBool(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[arena.Slice[byte]](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(repeatedBool{raw: nil})
	}

	v := *p
	var raw []byte
	if isZC(v) {
		raw = unwrapZC(v, m.context.src)
	} else {
		raw = v.Raw()
	}

	return protoreflect.ValueOf(repeatedBool{raw: raw})
}

// repeatedBool is a [protoreflect.List] implementation for bool.
type repeatedBool struct {
	unimplementedList
	raw []byte
}

func (l repeatedBool) Len() int { return len(l.raw) }
func (l repeatedBool) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(l.raw[n] != 0)
}

func getRepeatedString(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[arena.Slice[zc]](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(repeatedBool{raw: nil})
	}

	v := *p
	return protoreflect.ValueOf(repeatedString{raw: v.Raw(), shared: m.context})
}

// repeatedString is a [protoreflect.List] implementation for string.
type repeatedString struct {
	unimplementedList
	raw    []zc
	shared *Context
}

func (l repeatedString) Len() int { return len(l.raw) }
func (l repeatedString) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(l.raw[n].utf8(l.shared.src))
}

func repeatedBytes(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[arena.Slice[zc]](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(repeatedBool{raw: nil})
	}

	v := *p
	return protoreflect.ValueOf(bytesList{raw: v.Raw(), shared: m.context})
}

// bytesList is a [protoreflect.List] implementation for bytes.
type bytesList struct {
	unimplementedList
	raw    []zc
	shared *Context
}

func (l bytesList) Len() int { return len(l.raw) }
func (l bytesList) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(l.raw[n].bytes(l.shared.src))
}

//go:nosplit
//fastpb:stencil spillArena32 spillArena[uint32]
//fastpb:stencil spillArena64 spillArena[uint64]
func spillArena[T any](p1 parser1, p2 parser2, rep arena.Slice[T]) (parser1, parser2, arena.Slice[T]) {
	return p1, p2, arena.SliceOf(p1.arena(), unwrapZC(rep, p1.c().src)...)
}

//go:nosplit
//fastpb:stencil parseRepeatedVarint8 parseRepeatedVarint[uint8] appendVarint -> appendVarint8
//fastpb:stencil parseRepeatedVarint32 parseRepeatedVarint[uint32] appendVarint -> appendVarint32
//fastpb:stencil parseRepeatedVarint64 parseRepeatedVarint[uint64] appendVarint -> appendVarint64
func parseRepeatedVarint[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint64
	p1, p2, n = p1.varint(p2)

	var slot *arena.SliceAddr[T]
	p1, p2, slot = getMutableField[arena.SliceAddr[T]](p1, p2)
	slice := slot.AssertValid()

	// Check if we're already an arena, or an empty repeated field which looks like
	// an empty arena slice.
	if isZC(slice) && slice.Cap() > 0 {
		// Already holds a borrow. Need to spill to arena.
		// This is the worst-case scenario.
		zc := unwrapRawZC(slice).bytes(p1.c().src)
		slice := arena.NewSlice[T](p1.arena(), len(zc)+1)
		for i, b := range zc {
			slice.Store(i, T(b))
		}
		slice.Store(slice.Len()-1, T(n))
		p1.log(p2, "spill", "%v %v", slice.Addr(), slice)

		*slot = slice.Addr()
		return p1, p2
	}

	if slice.Len() < slice.Cap() {
		slice = slice.SetLen(slice.Len() + 1)
		slice.Store(slice.Len()-1, T(n))

		p1.log(p2, "store", "%v %v", slice.Addr(), slice)
		*slot = slice.Addr()
		return p1, p2
	}

	slice = slice.AppendOne(p1.arena(), T(n))
	p1.log(p2, "append", "%v %v", slice.Addr(), slice)
	*slot = slice.Addr()
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
	var count int
	for p := p1.b_; p < p1.e_; p += 8 {
		n := min(8, p1.e_-p)
		bytes := *unsafe2.Cast[uint64](p.AssertValid())
		bytes |= signBits << (n * 8)
		count += bits.OnesCount64(signBits &^ bytes)
	}

	var slot *arena.SliceAddr[T]
	p1, p2, slot = getMutableField[arena.SliceAddr[T]](p1, p2)
	slice := slot.AssertValid()
	if isZC(slice) {
		// Check if we're already on-arena, or an empty repeated field which looks
		// like an empty arena slice.
		switch {
		case slice.Cap() > 0:
			// Already holds a borrow. Need to spill to arena.
			// This is the worst-case scenario.
			zc := unwrapRawZC(slice).bytes(p1.c().src)
			slice = arena.NewSlice[T](p1.arena(), len(zc)+count)
			for i, b := range zc {
				slice.Store(i, T(b))
			}
			slice = slice.SetLen(len(zc))

			p1.log(p2, "spill", "%v %v", slice.Addr(), slice)

		case count == int(n):
			*slot = wrapZC[T](newZC(p1.c().src, p1.b(), int(n))).Addr()

			if dbg.Enabled {
				raw := unwrapRawZC(slot.AssertValid()).bytes(p1.c().src)
				p1.log(p2, "zc", "%v %v", *slot, raw)
			}

			p1.b_ = p1.e_
			p1.e_ = unsafe2.Addr[byte](p2.scratch)
			return p1, p2

		default:
			slice = slice.Grow(p1.arena(), count)
			p1.log(p2, "grow", "%v %v", slice.Addr(), slice)
		}
	} else if spare := slice.Cap() - slice.Len(); spare < count {
		slice = slice.Grow(p1.arena(), count-spare)
		p1.log(p2, "grow", "%v %v, %d", slice.Addr(), slice, spare)
	}

	// Manual inlining of AppendOne. Previously, we called AppendOne, but
	// Go would not inline it, which resulted in a lot of spilling in a hot
	// loop!
	p := unsafe2.AddrOf(slice.Ptr()).Add(slice.Len())
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

	slice = slice.SetLen(p.Sub(unsafe2.AddrOf(slice.Ptr())))
	p1.log(p2, "append", "%v %v", slice.Addr(), slice)

	*slot = slice.Addr()
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
	var slot *arena.SliceAddr[T]
	p1, p2, slot = getMutableField[arena.SliceAddr[T]](p1, p2)
	slice := slot.AssertValid()

	// Check if we're already an arena, or an empty repeated field which looks like
	// an empty arena slice.
	if isZC(slice) && slice.Cap() > 0 {
		// Already holds a borrow. Need to spill to arena.
		// This is the worst-case scenario.
		p1, p2, slice = spillArena(p1, p2, slice)
		p1.log(p2, "repeated fixed spill", "%v %v", slice.Addr(), slice)
	}

	if slice.Len() < slice.Cap() {
		slice = slice.SetLen(slice.Len() + 1)
		slice.Store(slice.Len()-1, v)
		p1.log(p2, "repeated fixed store", "%v %v", slice.Addr(), slice)

		*slot = slice.Addr()
		return p1, p2
	}

	slice = slice.AppendOne(p1.arena(), v)
	p1.log(p2, "repeated fixed append", "%v %v", slice.Addr(), slice)
	*slot = slice.Addr()
	return p1, p2
}

//go:nosplit
//fastpb:stencil parsePackedFixed32 parsePackedFixed[uint32]
//fastpb:stencil parsePackedFixed64 parsePackedFixed[uint64]
func parsePackedFixed[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var zc zc
	p1, p2, zc = p1.bytes(p2)
	if zc.len() == 0 {
		return p1, p2
	}

	size, _ := unsafe2.Layout[T]()
	if zc.len()%size != 0 {
		p1.fail(p2, errCodeTruncated)
	}

	var slot *arena.SliceAddr[T]
	p1, p2, slot = getMutableField[arena.SliceAddr[T]](p1, p2)
	slice := slot.AssertValid()

	switch {
	case !isZC(slice):
		// Already on an arena.
	case slice.Cap() == 0:
		// Empty repeated field. We can just shove the zc here.
		// This is the best-case scenario.
		*slot = wrapZC[T](zc).Addr()
		goto exit
	default:
		// Already holds a borrow. Need to spill to arena.
		// This is the worst-case scenario.
		p1, p2, slice = spillArena(p1, p2, slice)
	}

	{
		size, _ := unsafe2.Layout[T]()
		borrowed := unsafe2.Slice(
			unsafe2.Cast[T](unsafe2.Add(p1.c().src, zc.start())),
			zc.len()/size,
		)

		*slot = slice.Append(p1.arena(), borrowed...).Addr()
	}

exit:
	return p1, p2
}

//go:nosplit
func parseRepeatedBytes(p1 parser1, p2 parser2) (parser1, parser2) {
	var v zc
	p1, p2, v = p1.bytes(p2)

	var slice *arena.SliceAddr[zc]
	p1, p2, slice = getMutableField[arena.SliceAddr[zc]](p1, p2)
	*slice = slice.AssertValid().AppendOne(p1.arena(), v).Addr()

	return p1, p2
}

//go:nosplit
func parseRepeatedUTF8(p1 parser1, p2 parser2) (parser1, parser2) {
	var v zc
	p1, p2, v = p1.utf8(p2)

	var slice *arena.SliceAddr[zc]
	p1, p2, slice = getMutableField[arena.SliceAddr[zc]](p1, p2)
	*slice = slice.AssertValid().AppendOne(p1.arena(), v).Addr()

	return p1, p2
}
