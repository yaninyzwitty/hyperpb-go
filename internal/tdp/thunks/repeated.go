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

package thunks

import (
	"math/bits"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/hyperpb/internal/arena/slice"
	"github.com/bufbuild/hyperpb/internal/debug"
	"github.com/bufbuild/hyperpb/internal/tdp"
	"github.com/bufbuild/hyperpb/internal/tdp/compiler"
	"github.com/bufbuild/hyperpb/internal/tdp/dynamic"
	"github.com/bufbuild/hyperpb/internal/tdp/empty"
	"github.com/bufbuild/hyperpb/internal/tdp/vm"
	"github.com/bufbuild/hyperpb/internal/unsafe2"
	"github.com/bufbuild/hyperpb/internal/unsafe2/layout"
	"github.com/bufbuild/hyperpb/internal/zc"
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

var repeatedFields = map[protoreflect.Kind]*compiler.Archetype{
	// 32-bit varint types.
	protoreflect.Int32Kind: {
		Layout: layout.Of[repeatedScalar[byte, int32]](),
		Getter: getRepeatedScalar[byte, int32],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedVarint32},
			{Kind: protowire.VarintType, Retry: true, Thunk: parseRepeatedVarint32},
		},
	},
	protoreflect.Uint32Kind: {
		Layout: layout.Of[repeatedScalar[byte, uint32]](),
		Getter: getRepeatedScalar[byte, uint32],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedVarint32},
			{Kind: protowire.VarintType, Thunk: parseRepeatedVarint32},
		},
	},
	protoreflect.Sint32Kind: {
		Layout: layout.Of[repeatedZigzag[byte, uint32]](),
		Getter: getRepeatedZigzag[byte, int32],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedVarint32},
			{Kind: protowire.VarintType, Retry: true, Thunk: parseRepeatedVarint32},
		},
	},

	// 64-bit varint types.
	protoreflect.Int64Kind: {
		Layout: layout.Of[repeatedScalar[byte, int64]](),
		Getter: getRepeatedScalar[byte, int64],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedVarint64},
			{Kind: protowire.VarintType, Retry: true, Thunk: parseRepeatedVarint64},
		},
	},
	protoreflect.Uint64Kind: {
		Layout: layout.Of[repeatedScalar[byte, uint64]](),
		Getter: getRepeatedScalar[byte, uint64],
		Parsers: []compiler.Parser{
			{Kind: protowire.VarintType, Retry: true, Thunk: parseRepeatedVarint64},
			{Kind: protowire.BytesType, Thunk: parsePackedVarint64},
		},
	},
	protoreflect.Sint64Kind: {
		Layout: layout.Of[repeatedZigzag[byte, int64]](),
		Getter: getRepeatedZigzag[byte, int64],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedVarint64},
			{Kind: protowire.VarintType, Retry: true, Thunk: parseRepeatedVarint64},
		},
	},

	// 32-bit fixed types.
	protoreflect.Fixed32Kind: {
		Layout: layout.Of[repeatedScalar[uint32, uint32]](),
		Getter: getRepeatedScalar[uint32, uint32],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedFixed32},
			{Kind: protowire.Fixed32Type, Retry: true, Thunk: parseRepeatedFixed32},
		},
	},
	protoreflect.Sfixed32Kind: {
		Layout: layout.Of[repeatedScalar[int32, int32]](),
		Getter: getRepeatedScalar[int32, int32],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedFixed32},
			{Kind: protowire.Fixed32Type, Retry: true, Thunk: parseRepeatedFixed32},
		},
	},
	protoreflect.FloatKind: {
		Layout: layout.Of[repeatedScalar[float32, float32]](),
		Getter: getRepeatedScalar[float32, float32],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedFixed32},
			{Kind: protowire.Fixed32Type, Retry: true, Thunk: parseRepeatedFixed32},
		},
	},

	// 64-bit fixed types.
	protoreflect.Fixed64Kind: {
		Layout: layout.Of[repeatedScalar[uint64, uint64]](),
		Getter: getRepeatedScalar[uint64, uint64],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedFixed64},
			{Kind: protowire.Fixed64Type, Retry: true, Thunk: parseRepeatedFixed64},
		},
	},
	protoreflect.Sfixed64Kind: {
		Layout: layout.Of[repeatedScalar[int64, int64]](),
		Getter: getRepeatedScalar[int64, int64],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedFixed64},
			{Kind: protowire.Fixed64Type, Retry: true, Thunk: parseRepeatedFixed64},
		},
	},
	protoreflect.DoubleKind: {
		Layout: layout.Of[repeatedScalar[float64, float64]](),
		Getter: getRepeatedScalar[float64, float64],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedFixed64},
			{Kind: protowire.Fixed64Type, Retry: true, Thunk: parseRepeatedFixed64},
		},
	},

	// Special scalar types.
	protoreflect.BoolKind: {
		Layout: layout.Of[repeatedBool](),
		Getter: getRepeatedBool,
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedVarint8},
			{Kind: protowire.VarintType, Retry: true, Thunk: parseRepeatedVarint8},
		},
	},
	protoreflect.EnumKind: {
		Layout: layout.Of[repeatedScalar[byte, protoreflect.EnumNumber]](),
		Getter: getRepeatedScalar[byte, protoreflect.EnumNumber],
		Parsers: []compiler.Parser{
			{Kind: protowire.BytesType, Thunk: parsePackedVarint32},
			{Kind: protowire.VarintType, Retry: true, Thunk: parseRepeatedVarint32},
		},
	},

	// String types.
	protoreflect.StringKind: {
		Layout:  layout.Of[repeatedString](),
		Getter:  getRepeatedString,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Retry: true, Thunk: parseRepeatedUTF8}},
	},
	proto2StringKind: {
		Layout:  layout.Of[repeatedString](),
		Getter:  getRepeatedString,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Retry: true, Thunk: parseRepeatedBytes}},
	},
	protoreflect.BytesKind: {
		Layout:  layout.Of[repeatedBytes](),
		Getter:  getRepeatedBytes,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Retry: true, Thunk: parseRepeatedBytes}},
	},

	// Message types.
	protoreflect.MessageKind: {
		Layout:  layout.Of[repeatedMessage](),
		Getter:  getRepeatedMessage,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Retry: true, Thunk: parseRepeatedMessage}},
	},
	protoreflect.GroupKind: {
		Layout:  layout.Of[repeatedMessage](),
		Getter:  getRepeatedMessage,
		Parsers: []compiler.Parser{{Kind: protowire.StartGroupType, Retry: true, Thunk: parseRepeatedGroup}},
	},
}

type repeatedScalarElement interface {
	tdp.Int | ~float32 | ~float64
}

func getRepeatedScalar[Z, E repeatedScalarElement](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[repeatedScalar[Z, E]](m, getter.Offset)
	return protoreflect.ValueOf(p)
}

type repeatedScalar[Z, E repeatedScalarElement] struct {
	empty.List
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

func getRepeatedZigzag[Z, E tdp.Int](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[repeatedZigzag[Z, E]](m, getter.Offset)
	return protoreflect.ValueOf(p)
}

type repeatedZigzag[Z, E tdp.Int] struct {
	empty.List
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

func getRepeatedBool(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[repeatedString](m, getter.Offset)
	return protoreflect.ValueOf(p)
}

type repeatedBool struct {
	empty.List
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

func getRepeatedString(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[repeatedString](m, getter.Offset)
	return protoreflect.ValueOf(p)
}

// repeatedString is a [protoreflect.List] implementation for string.
type repeatedString struct {
	empty.List
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

func getRepeatedBytes(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	p := dynamic.GetField[repeatedBytes](m, getter.Offset)
	return protoreflect.ValueOf(p)
}

// repeatedBytes is a [protoreflect.List] implementation for string.
type repeatedBytes struct {
	empty.List
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
//hyperpb:stencil parseRepeatedVarint8 parseRepeatedVarint[uint8] appendVarint -> appendVarint8
//hyperpb:stencil parseRepeatedVarint32 parseRepeatedVarint[uint32] appendVarint -> appendVarint32
//hyperpb:stencil parseRepeatedVarint64 parseRepeatedVarint[uint64] appendVarint -> appendVarint64
func parseRepeatedVarint[T tdp.Int](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n uint64
	p1, p2, n = p1.Varint(p2)

	var r *repeatedScalar[byte, T]
	p1, p2, r = vm.GetMutableField[repeatedScalar[byte, T]](p1, p2)
	p1.Log(p2, "slot", "%v", r.raw)

	// Check if we're already an arena, or an empty repeated field which looks like
	// an empty arena slice.
	if r.raw.OffArena() {
		borrow := slice.CastUntyped[byte](r.raw).Raw()
		s := slice.Make[T](p1.Arena(), len(borrow)+1)
		for i, b := range borrow {
			s.Store(i, T(b))
		}
		s.Store(s.Len()-1, T(n))
		p1.Log(p2, "spill", "%v->%v", r.raw, s.Addr())

		r.raw = s.Addr().Untyped()
		return p1, p2
	}

	s := slice.CastUntyped[T](r.raw)
	if s.Len() < s.Cap() {
		s = s.SetLen(s.Len() + 1)
		s.Store(s.Len()-1, T(n))

		p1.Log(p2, "store", "%v", s.Addr())
		r.raw = s.Addr().Untyped()
		return p1, p2
	}

	s = s.AppendOne(p1.Arena(), T(n))
	p1.Log(p2, "append", "%v", s.Addr())
	r.raw = s.Addr().Untyped()
	return p1, p2
}

//go:nosplit
//hyperpb:stencil parsePackedVarint8 parsePackedVarint[uint8]
//hyperpb:stencil parsePackedVarint32 parsePackedVarint[uint32]
//hyperpb:stencil parsePackedVarint64 parsePackedVarint[uint64]
func parsePackedVarint[T tdp.Int](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n int
	p1, p2, n = p1.LengthPrefix(p2)
	if n == 0 {
		return p1, p2
	}

	p1, p2 = p1.SetScratch(p2, uint64(p1.EndAddr))
	p1.EndAddr = p1.PtrAddr.Add(n)

	// Count the number of varints in this packed field. We do this by counting
	// bytes without the sign bit set, in groups of 8.
	var count int
	for p := p1.PtrAddr; p < p1.EndAddr; p += 8 {
		n := min(8, p1.EndAddr.Sub(p))
		bytes := *unsafe2.Cast[uint64](p.AssertValid())
		bytes |= tdp.SignBits << uint(n*8)
		count += bits.OnesCount64(tdp.SignBits &^ bytes)
	}

	var r *repeatedScalar[byte, T]
	p1, p2, r = vm.GetMutableField[repeatedScalar[byte, T]](p1, p2)
	var s slice.Slice[T]
	switch {
	case r.raw.Ptr == 0:
		if count == n {
			r.raw = slice.OffArena(p1.Ptr(), n)
			p1.Log(p2, "zc", "%v", r.raw)

			p1.PtrAddr = p1.EndAddr
			p1.EndAddr = unsafe2.Addr[byte](p2.Scratch())
			return p1, p2
		}
		s = s.Grow(p1.Arena(), count)
		p1.Log(p2, "grow", "%v", s.Addr())

	case r.raw.OffArena():
		// Already holds a borrow. Need to spill to arena.
		// This is the worst-case scenario.
		borrow := slice.CastUntyped[byte](r.raw).Raw()
		s = slice.Make[T](p1.Arena(), len(borrow)+count)
		for i, b := range borrow {
			s.Store(i, T(b))
		}
		s = s.SetLen(len(borrow))

		p1.Log(p2, "spill", "%v->%v", r.raw, s.Addr())

	default:
		s = slice.CastUntyped[T](r.raw)
		if spare := s.Cap() - s.Len(); spare < count {
			s = s.Grow(p1.Arena(), count-spare)
			p1.Log(p2, "grow", "%v, %d", s.Addr(), spare)
		}
	}

	p := unsafe2.AddrOf(s.Ptr()).Add(s.Len())
	p1.Log(p2, "store at", "%v", p)

	// There are three variants of this loop: one for the cases where every
	// varint is small (one byte; common). One for the cases where most varints
	// are small (so the special-case branches are likely to be well-predicted)
	// and when many varints are large so the aforementioned branches would
	// not be predicted well.
	switch {
	case count == p1.Len():
		for {
			*p.AssertValid() = T(*p1.Ptr())
			p1.PtrAddr++
			p = p.Add(1)

			if p1.PtrAddr != p1.EndAddr {
				continue
			}

			break
		}
	case count >= p1.Len()/2:
		for {
			var x uint64
			if v := *p1.Ptr(); int8(v) >= 0 {
				x = uint64(v)
				p1.PtrAddr++
			} else if c := p1.PtrAddr.Add(1); c != p1.EndAddr && int8(*c.AssertValid()) >= 0 {
				x = uint64(*p1.Ptr()&0x7f) | uint64(*c.AssertValid())<<7
				p1.PtrAddr += 2
			} else {
				p1, p2, x = p1.Varint(p2)
			}

			*p.AssertValid() = T(x)
			p = p.Add(1)
			if p1.PtrAddr != p1.EndAddr {
				continue
			}

			break
		}
	default:
		for {
			var x uint64
			p1, p2, x = p1.Varint(p2)

			*p.AssertValid() = T(x)
			p = p.Add(1)
			if p1.PtrAddr != p1.EndAddr {
				continue
			}

			break
		}
	}

	s = s.SetLen(p.Sub(unsafe2.AddrOf(s.Ptr())))
	p1.Log(p2, "append", "%v", s.Addr())

	r.raw = s.Addr().Untyped()
	p1.EndAddr = unsafe2.Addr[byte](p2.Scratch())
	return p1, p2
}

//go:nosplit
func parseRepeatedFixed32(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	return appendFixed32(p1.Fixed32(p2))
}

//go:nosplit
func parseRepeatedFixed64(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	return appendFixed64(p1.Fixed64(p2))
}

//go:nosplit
//hyperpb:stencil appendFixed32 appendFixed[uint32] spillArena -> spillArena32
//hyperpb:stencil appendFixed64 appendFixed[uint64] spillArena -> spillArena64
func appendFixed[T uint32 | uint64](p1 vm.P1, p2 vm.P2, v T) (vm.P1, vm.P2) {
	var r *repeatedScalar[T, T]
	p1, p2, r = vm.GetMutableField[repeatedScalar[T, T]](p1, p2)
	s := slice.CastUntyped[T](r.raw)

	if s.Len() < s.Cap() {
		s = s.SetLen(s.Len() + 1)
		s.Store(s.Len()-1, v)
		p1.Log(p2, "repeated fixed store", "%v %v", s.Addr(), s)

		r.raw = s.Addr().Untyped()
		return p1, p2
	}

	s = s.AppendOne(p1.Arena(), v)
	p1.Log(p2, "repeated fixed append", "%v %v", s.Addr(), s)
	r.raw = s.Addr().Untyped()
	return p1, p2
}

//go:nosplit
//hyperpb:stencil parsePackedFixed32 parsePackedFixed[uint32]
//hyperpb:stencil parsePackedFixed64 parsePackedFixed[uint64]
func parsePackedFixed[T tdp.Int](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n int
	p1, p2, n = p1.LengthPrefix(p2)
	if n == 0 {
		return p1, p2
	}

	size := layout.Size[T]()
	if n%size != 0 {
		p1.Fail(p2, vm.ErrorTruncated)
	}

	var r *repeatedScalar[T, T]
	p1, p2, r = vm.GetMutableField[repeatedScalar[T, T]](p1, p2)

	if r.raw.Ptr == 0 {
		// Empty repeated field. We can just shove the zc here.
		// This is the best-case scenario.
		r.raw = slice.OffArena(p1.Ptr(), n/size)
		if debug.Enabled {
			p1.Log(p2, "zc", "%v, %v", r.raw, slice.CastUntyped[T](r.raw))
		}

		p1 = p1.Advance(n)
		goto exit
	}

	// If r.raw is off-arena, it will have len==cap, so it will force a
	// realloc when we call Append.

	{
		s := slice.CastUntyped[T](r.raw)
		size := layout.Size[T]()
		borrowed := unsafe2.Slice(unsafe2.Cast[T](p1.Ptr()), n/size)
		if debug.Enabled {
			p1.Log(p2, "appending", "%v, %v", borrowed, s.Raw())
		}

		p1 = p1.Advance(n)

		s = s.Append(p1.Arena(), borrowed...)
		r.raw = s.Addr().Untyped()
		if debug.Enabled {
			p1.Log(p2, "append", "%v, %v", r.raw, s.Raw())
		}
	}

exit:
	return p1, p2
}

//go:nosplit
func parseRepeatedBytes(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var v zc.Range
	p1, p2, v = p1.Bytes(p2)

	var r *repeatedBytes
	p1, p2, r = vm.GetMutableField[repeatedBytes](p1, p2)
	if r.raw.Ptr == 0 {
		if preload := p2.Field().Preload; preload > 0 {
			r.raw = slice.Make[zc.Range](p1.Arena(), int(preload)).Addr()
		}
	}

	r.raw = r.raw.AssertValid().AppendOne(p1.Arena(), v).Addr()
	unsafe2.StoreNoWB(&r.src, p1.Src())

	return p1, p2
}

//go:nosplit
func parseRepeatedUTF8(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var v zc.Range
	p1, p2, v = p1.UTF8(p2)

	var r *repeatedString
	p1, p2, r = vm.GetMutableField[repeatedString](p1, p2)
	if r.raw.Ptr == 0 {
		if preload := p2.Field().Preload; preload > 0 {
			r.raw = slice.Make[zc.Range](p1.Arena(), int(preload)).Addr()
		}
	}

	r.raw = r.raw.AssertValid().AppendOne(p1.Arena(), v).Addr()
	unsafe2.StoreNoWB(&r.src, p1.Src())

	return p1, p2
}
