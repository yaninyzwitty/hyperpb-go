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

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Repeated messages use two different layouts, and a hasbit is used to
// differentiate them. The messages can either be packed into an arena slice,
// or the arena slice can contain *message pointers. These are called inlined
// and outlined modes; the hasbit is set in the latter case. We switch to the
// outlined mode to avoid needing to copy parsed messages on slice resize.

func getRepeatedMessage(m *message, _ Type, getter getter) protoreflect.Value {
	if m.getBit(getter.offset.bit) {
		raw := *getField[arena.Slice[*message]](m, getter.offset)
		return protoreflect.ValueOf(messageList{raw: raw.Raw()})
	}

	p := getField[arena.Slice[byte]](m, getter.offset)
	if p == nil {
		return protoreflect.ValueOf(repeatedBool{raw: nil})
	}

	v := *p
	if v.Ptr() == nil {
		return protoreflect.ValueOf(emptyList{})
	}

	first := unsafe2.Cast[message](v.Ptr()) // Get the type from the first element of the list.
	return protoreflect.ValueOf(inlineMessageList{
		ty:    first.ty(),
		raw:   first,
		dummy: make([]struct{}, v.Len()/int(first.ty().raw.size)),
	})
}

// messageList is a [protoreflect.List] implementation for message types.
type messageList struct {
	unimplementedList
	raw []*message
}

func (l messageList) Len() int { return len(l.raw) }
func (l messageList) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(l.raw[n])
}

// inlineMessageList is a [protoreflect.List] implementation for message types.
type inlineMessageList struct {
	unimplementedList
	ty    Type
	raw   *message
	dummy []struct{}
}

func (l inlineMessageList) Len() int { return len(l.dummy) }
func (l inlineMessageList) Get(n int) protoreflect.Value {
	_ = l.dummy[n] // Bounds check
	return protoreflect.ValueOf(unsafe2.ByteAdd(l.raw, n*int(l.ty.raw.size)))
}

//go:nosplit
func parseRepeatedMessage(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.lengthPrefix(p2)

	var slot *arena.SliceAddr[byte]
	p1, p2, slot = getMutableField[arena.SliceAddr[byte]](p1, p2)
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
	p1, p2 = p1.setBit(p2) // Mark this as an outlined message.

	return p1, p2
}

//go:noinline
func appendOneMessage(p1 parser1, p2 parser2, m *message) (parser1, parser2, *message) {
	var slot *arena.SliceAddr[unsafe2.Addr[message]]
	p1, p2, slot = getMutableField[arena.SliceAddr[unsafe2.Addr[message]]](p1, p2)
	*slot = slot.AssertValid().AppendOne(p1.arena(), unsafe2.AddrOf(m)).Addr()
	return p1, p2, m
}

// emptyList is an empty untyped list.
type emptyList struct {
	unimplementedList
}

func (emptyList) Len() int { return 0 }
func (emptyList) Get(n int) protoreflect.Value {
	_ = []byte{}[n] // Trigger a bounds check.
	return protoreflect.Value{}
}

// unimplementedList implements [protoreflect.List] by panicking.
type unimplementedList struct{}

func (unimplementedList) IsValid() bool                     { return true }
func (unimplementedList) Append(protoreflect.Value)         { panic(dbg.Unsupported()) }
func (unimplementedList) AppendMutable() protoreflect.Value { panic(dbg.Unsupported()) }
func (unimplementedList) Get(int) protoreflect.Value        { panic(dbg.Unsupported()) }
func (unimplementedList) Len() int                          { panic(dbg.Unsupported()) }
func (unimplementedList) NewElement() protoreflect.Value    { panic(dbg.Unsupported()) }
func (unimplementedList) Set(int, protoreflect.Value)       { panic(dbg.Unsupported()) }
func (unimplementedList) Truncate(int)                      { panic(dbg.Unsupported()) }
