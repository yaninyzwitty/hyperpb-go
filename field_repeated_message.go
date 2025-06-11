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
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/arena/slice"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Repeated messages use two different layouts, and a hasbit is used to
// differentiate them. The messages can either be packed into an arena slice,
// or the arena slice can contain *message pointers. These are called inlined
// and outlined modes; the hasbit is set in the latter case. We switch to the
// outlined mode to avoid needing to copy parsed messages on slice resize.

func getRepeatedMessage(m *message, _ Type, getter getter) protoreflect.Value {
	p := getField[repeatedMessage](m, getter.offset)
	return protoreflect.ValueOf(p)
}

// repeatedMessage is a [protoreflect.List] implementation for message types.
type repeatedMessage struct {
	immutableList

	// Slice[byte] if stride is non-nil, Slice[*message] otherwise.
	raw slice.Untyped

	// The array stride for when raw is an inlined message list.
	stride uint32
}

// IsValid implements [protoreflect.List].
func (r *repeatedMessage) IsValid() bool { return r != nil }

// Len implements [protoreflect.List].
func (r *repeatedMessage) Len() int {
	if r == nil {
		return 0
	}

	if r.stride != 0 {
		return int(r.raw.Len) / int(r.stride)
	}

	return int(r.raw.Len)
}

// Get implements [protoreflect.List].
func (r *repeatedMessage) Get(n int) protoreflect.Value {
	if r.stride != 0 {
		unsafe2.BoundsCheck(n, int(r.raw.Len)/int(r.stride))
		p := unsafe2.ByteAdd(r.raw.Ptr.AssertValid(), n*int(r.stride))
		return protoreflect.ValueOf(unsafe2.Cast[message](p))
	}

	raw := slice.CastUntyped[*message](r.raw).Raw()
	return protoreflect.ValueOf(raw[n])
}

//go:nosplit
func parseRepeatedMessage(p1 parser1, p2 parser2) (parser1, parser2) {
	var n int
	p1, p2, n = p1.lengthPrefix(p2)

	var r *repeatedMessage
	p1, p2, r = getMutableField[repeatedMessage](p1, p2)
	p1.log(p2, "repeated message", "%v", r.raw)

	var m *message

	if r.raw.Ptr != 0 && r.stride == 0 {
		goto pointers
	}

	{
		ty := p1.c().lib.fromOffset(p2.f().message.tyOffset)
		stride := int(ty.raw.size)
		s := slice.CastUntyped[byte](r.raw)

		if r.raw.Ptr == 0 {
			p1, p2, r = newInlineRepeatedField(p1, p2, r)
		} else if s.Len()+stride > s.Cap() {
			p1, p2 = spillInlineRepeatedField(p1, p2, r)
			p1.log(p2, "repeated message spill", "%v->%v", s.Addr(), *r)

			goto pointers
		}

		s = slice.CastUntyped[byte](r.raw)
		p := unsafe2.Add(s.Ptr(), s.Len())
		s = s.SetLen(s.Len() + stride)
		r.raw = s.Addr().Untyped()

		p1.log(p2, "inline repeated message", "%v, %p/%d", s.Addr(), p, stride)
		p1, p2, m = p1.allocInPlace(p2, p)
		goto exit
	}

pointers:
	{
		p1, p2, m = p1.alloc(p2)

		r := unsafe2.Cast[slice.Addr[unsafe2.Addr[message]]](r)
		s := r.AssertValid()
		if s.Len() == s.Cap() {
			p1, p2, m = appendOneMessage(p1, p2, m)
			p1.log(p2, "outline repeated message", "%v, %p", *r, m)
			goto exit
		}

		s = s.SetLen(s.Len() + 1)
		s.Store(s.Len()-1, unsafe2.AddrOf(m))
		p1.log(p2, "outline repeated message", "%v, %p", s.Addr(), m)
		*r = s.Addr()
	}

exit:
	return p1.message(p2, n, m)
}

//go:noinline
func newInlineRepeatedField(p1 parser1, p2 parser2, r *repeatedMessage) (parser1, parser2, *repeatedMessage) {
	// First element of this field. Allocate a byte array large enough to
	// hold one element.
	//
	// TODO: Add a profiling knob for setting the default number of
	// elements.
	ty := p1.c().lib.fromOffset(p2.f().message.tyOffset)
	stride := ty.raw.size

	s := slice.Make[byte](p1.arena(), int(stride))
	s = s.SetLen(0)

	r.raw = s.Addr().Untyped()
	r.stride = stride

	return p1, p2, r
}

//go:noinline
func spillInlineRepeatedField(p1 parser1, p2 parser2, r *repeatedMessage) (parser1, parser2) {
	ty := p1.c().lib.fromOffset(p2.f().message.tyOffset)
	stride := int(ty.raw.size)
	s := slice.CastUntyped[byte](r.raw)

	// Spill all of the messages onto a pointer slice.
	spill := slice.Make[unsafe2.Addr[message]](p1.arena(), s.Cap()/stride*2)
	var j int
	for i := 0; i < s.Len(); i += stride {
		m := unsafe2.Cast[message](unsafe2.Add(s.Ptr(), i))
		spill.Store(j, unsafe2.AddrOf(m))
		j++
	}
	spill = spill.SetLen(j)

	r.raw = spill.Addr().Untyped()
	r.stride = 0 // Mark this as an outlined message.

	return p1, p2
}

//go:noinline
func appendOneMessage(p1 parser1, p2 parser2, m *message) (parser1, parser2, *message) {
	var slot *repeatedMessage
	p1, p2, slot = getMutableField[repeatedMessage](p1, p2)
	s := slice.CastUntyped[unsafe2.Addr[message]](slot.raw)
	slot.raw = s.AppendOne(p1.arena(), unsafe2.AddrOf(m)).Addr().Untyped()
	return p1, p2, m
}
