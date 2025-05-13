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

//go:generate go run ./internal/stencil

func isZC[T any](slice arena.Slice[T]) bool {
	return slice.Ptr() == nil
}

func wrapZC[T any](zc zc) arena.Slice[T] {
	return arena.SliceFromParts[T](nil, zc.offset, zc.len)
}

func unwrapRawZC[T any](slice arena.Slice[T]) zc {
	return zc{offset: uint32(slice.Len()), len: uint32(slice.Cap())}
}

func unwrapZC[T any](slice arena.Slice[T], src *byte) []T {
	size, _ := unsafe2.Layout[T]()
	// This is a borrow from src, and len and cap are a zc. Note that both
	// are denominated in bytes in this mode.
	return unsafe2.Slice(
		unsafe2.Cast[T](unsafe2.Add(src, slice.Len())),
		slice.Cap()/size,
	)
}

var repeatedFields = [...]archetype{
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
	protoreflect.BytesKind: {
		size:    uint32(arena.SliceSize),
		align:   uint32(arena.SliceAlign),
		getter:  getRepeatedBytes,
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
	v := unsafe2.ByteLoad[arena.Slice[T]](m, getter.offset.data)
	var raw []T
	if isZC(v) {
		raw = unwrapZC(v, m.context.src)
	} else {
		raw = v.Raw()
	}
	return protoreflect.ValueOf(scalarList[T]{raw: raw})
}

func getRepeatedScalarMaybeBytes[T integer](m *message, _ Type, getter getter) protoreflect.Value {
	v := unsafe2.ByteLoad[arena.Slice[T]](m, getter.offset.data)
	if isZC(v) {
		raw := unwrapRawZC(v).bytes(m.context.src)
		return protoreflect.ValueOf(byteScalarList[T]{raw: raw})
	}

	return protoreflect.ValueOf(scalarList[T]{raw: v.Raw()})
}

func getRepeatedZigZag[T integer](m *message, _ Type, getter getter) protoreflect.Value {
	v := unsafe2.ByteLoad[arena.Slice[T]](m, getter.offset.data)
	if isZC(v) {
		raw := unwrapRawZC(v).bytes(m.context.src)
		return protoreflect.ValueOf(byteZigZagList[T]{raw: raw})
	}

	return protoreflect.ValueOf(zigzagList[T]{raw: v.Raw()})
}

func getRepeatedBool(m *message, _ Type, getter getter) protoreflect.Value {
	v := unsafe2.ByteLoad[arena.Slice[byte]](m, getter.offset.data)
	var raw []byte
	if isZC(v) {
		raw = unwrapZC(v, m.context.src)
	} else {
		raw = v.Raw()
	}

	return protoreflect.ValueOf(boolList{raw: raw})
}

func getRepeatedBytes(m *message, _ Type, getter getter) protoreflect.Value {
	raw := unsafe2.ByteLoad[arena.Slice[zc]](m, getter.offset.data).Raw()
	return protoreflect.ValueOf(bytesList{raw: raw, shared: m.context})
}

func getRepeatedString(m *message, _ Type, getter getter) protoreflect.Value {
	raw := unsafe2.ByteLoad[arena.Slice[zc]](m, getter.offset.data).Raw()
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

	first := unsafe2.Cast[message](raw.Ptr()) // Get the type from the first element of the list.
	return protoreflect.ValueOf(inlineMessageList{
		ty:    first.ty(),
		raw:   first,
		dummy: make([]struct{}, raw.Len()/int(first.ty().raw.size)),
	})
}

//go:nosplit
//fastpb:stencil spillArena8 spillArena[uint8]
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

	slot := unsafe2.Cast[arena.SliceAddr[T]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
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

	slot := unsafe2.Cast[arena.SliceAddr[T]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
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
			*slot = wrapZC[T](zc{
				offset: uint32(p1.b_.Sub(unsafe2.AddrOf(p1.c().src))),
				len:    n,
			}).Addr()

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
	slot := unsafe2.Cast[arena.SliceAddr[T]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
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
	if zc.len == 0 {
		return p1, p2
	}

	size, _ := unsafe2.Layout[T]()
	if int(zc.len)%size != 0 {
		p1.fail(p2, errCodeTruncated)
	}

	slot := unsafe2.Cast[arena.SliceAddr[T]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
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
			unsafe2.Cast[T](unsafe2.Add(p1.c().src, zc.offset)),
			int(zc.len)/size,
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

	slice := unsafe2.Cast[arena.SliceAddr[zc]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
	*slice = slice.AssertValid().AppendOne(p1.arena(), v).Addr()

	return p1, p2
}

//go:nosplit
func parseRepeatedUTF8(p1 parser1, p2 parser2) (parser1, parser2) {
	var v zc
	p1, p2, v = p1.utf8(p2)

	slice := unsafe2.Cast[arena.SliceAddr[zc]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
	*slice = slice.AssertValid().AppendOne(p1.arena(), v).Addr()

	return p1, p2
}

//go:nosplit
func parseRepeatedMessage(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.lengthPrefix(p2)

	slot := unsafe2.Cast[arena.SliceAddr[byte]](unsafe2.ByteAdd(p2.m(), p2.f().offset.data))
	slice := slot.AssertValid()
	p1.log(p2, "repeated message", "%v", slice.Addr())

	var m *message

	if p2.m().getBit(p2.f().offset.bit) {
		goto pointers
	}

	{
		ty := p1.c().lib.fromOffset(p2.f().message.tyOffset)
		size := int(ty.raw.size)
		if slice.Ptr() == nil {
			p1, p2, slot = newInlineRepeatedField(p1, p2, slot)
		} else if slice.Len()+size > slice.Cap() {
			p1, p2 = spillInlineRepeatedField(p1, p2, slot)
			p1.log(p2, "repeated message spill", "%v->%v", slice.Addr(), *slot)

			goto pointers
		}

		slice = slot.AssertValid()
		p := unsafe2.Add(slice.Ptr(), slice.Len())
		slice = slice.SetLen(slice.Len() + size)
		*slot = slice.Addr()

		p1.log(p2, "inline repeated message", "%v, %p/%d", slice.Addr(), p, size)
		p1, p2, m = p1.allocInPlace(p2, p)
		goto exit
	}

pointers:
	{
		p1, p2, m = p1.alloc(p2)

		slot := unsafe2.Cast[arena.SliceAddr[unsafe2.Addr[message]]](slot)
		slice := slot.AssertValid()
		if slice.Len() == slice.Cap() {
			p1, p2, m = appendOneMessage(p1, p2, m)
			p1.log(p2, "outline repeated message", "%v, %p", *slot, m)
			goto exit
		}

		slice = slice.SetLen(slice.Len() + 1)
		slice.Store(slice.Len()-1, unsafe2.AddrOf(m))
		p1.log(p2, "outline repeated message", "%v, %p", slice.Addr(), m)
		*slot = slice.Addr()
	}

exit:
	return p1.message(p2, int(n), m)
}

//go:noinline
func newInlineRepeatedField(p1 parser1, p2 parser2, slot *arena.SliceAddr[byte]) (parser1, parser2, *arena.SliceAddr[byte]) {
	// First element of this field. Allocate a byte array large enough to
	// hold one element.
	//
	// TODO: Add a profiling knob for setting the default number of
	// elements.
	ty := p1.c().lib.fromOffset(p2.f().message.tyOffset)
	size := ty.raw.size
	slice := arena.NewSlice[byte](p1.arena(), int(size))
	slice = slice.SetLen(0)
	*slot = slice.Addr()

	return p1, p2, slot
}

//go:noinline
func spillInlineRepeatedField(p1 parser1, p2 parser2, slot *arena.SliceAddr[byte]) (parser1, parser2) {
	ty := p1.c().lib.fromOffset(p2.f().message.tyOffset)
	size := int(ty.raw.size)
	slice := slot.AssertValid()

	// Spill all of the messages onto a pointer slice.
	spill := arena.NewSlice[unsafe2.Addr[message]](p1.arena(), slice.Cap()/size*2)
	var spillIdx int
	for i := 0; i < slice.Len(); i += size {
		m := unsafe2.Cast[message](unsafe2.Add(slice.Ptr(), i))
		spill.Store(spillIdx, unsafe2.AddrOf(m))
		spillIdx++
	}
	spill = spill.SetLen(spillIdx)

	*unsafe2.Cast[arena.SliceAddr[unsafe2.Addr[message]]](slot) = spill.Addr()
	p2.m().setBit(p2.f().offset.bit, true) // Mark this as an outlined message.

	return p1, p2
}

//go:noinline
func appendOneMessage(p1 parser1, p2 parser2, m *message) (parser1, parser2, *message) {
	slot := unsafe2.Cast[arena.SliceAddr[unsafe2.Addr[message]]](
		unsafe2.ByteAdd(p2.m(), p2.f().offset.data),
	)
	*slot = slot.AssertValid().AppendOne(p1.arena(), unsafe2.AddrOf(m)).Addr()
	return p1, p2, m
}
