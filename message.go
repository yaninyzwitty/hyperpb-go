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

package fastpb

import (
	"unsafe"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/tdp/dynamic"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Message is a dynamic message value constructed with this package.
//
// Messages types returned by this package implement this interface.
// The Type() function on [Message] will return a [fastpb.Type]. Any functions
// that mutate the underlying message may panic.
type Message struct {
	impl dynamic.Message
}

var (
	_ proto.Message        = new(Message)
	_ protoreflect.Message = new(Message)
)

// New allocates a new [Message] of the given [Type].
//
// See [Shared.New].
func New(ty *Type) *Message {
	return new(Shared).New(ty)
}

// Shared returns state shared by this message and its submessages.
func (m *Message) Shared() *Shared {
	return newShared(m.impl.Shared)
}

func (m *Message) Unmarshal(data []byte, options ...UnmarshalOption) error {
	return startParse(&m.impl, data, options...)
}

// ProtoReflect implements [proto.Message].
func (m *Message) ProtoReflect() protoreflect.Message {
	return m
}

// Descriptor implements [protoreflect.Message].
func (m *Message) Descriptor() protoreflect.MessageDescriptor {
	return m.impl.Type().Descriptor
}

// Type implements [protoreflect.Message].
//
// Always returns *[Type].
func (m *Message) Type() protoreflect.MessageType {
	return newType(m.impl.Type())
}

// New implements [protoreflect.Message].
func (m *Message) New() protoreflect.Message {
	return newType(m.impl.Type()).New()
}

// Interface implements [protoreflect.Message].
func (m *Message) Interface() protoreflect.ProtoMessage {
	return m
}

// Range implements [protoreflect.Message].
func (m *Message) Range(yield func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {
	if !m.IsValid() {
		return
	}

	ty := m.impl.Type()
	f := ty.ByIndex(0)
	i := 0
	for f.IsValid() {
		fd := ty.FieldDescriptors[i]
		v := f.Get(unsafe.Pointer(m))
		switch {
		case !v.IsValid():
			goto skip

		case fd.IsList():
			if v.List().Len() == 0 {
				goto skip
			}

		case fd.IsMap():
			if v.Map().Len() == 0 {
				goto skip
			}

		case fd.Message() != nil:
			if _, empty := v.Interface().(empty); empty {
				goto skip
			}
		}

		if !yield(ty.FieldDescriptors[i], v) {
			return
		}

	skip:
		f = unsafe2.Add(f, 1)
		i++
	}
}

// Has implements [protoreflect.Message].
func (m *Message) Has(fd protoreflect.FieldDescriptor) bool {
	if !m.IsValid() {
		return false
	}

	f := m.impl.Type().ByDescriptor(fd)
	if !f.IsValid() {
		return false
	}

	v := f.Get(unsafe.Pointer(m))
	switch {
	case !v.IsValid():
		return false

	case fd.IsList():
		return v.List().Len() > 0

	case fd.IsMap():
		return v.Map().Len() > 0

	case fd.Message() != nil:
		_, empty := v.Interface().(empty)
		return !empty

	default:
		return true
	}
}

// Clear implements [protoreflect.Message].
func (m *Message) Clear(protoreflect.FieldDescriptor) {
	if m.Shared().impl.Src == nil {
		return
	}
	panic(dbg.Unsupported())
}

// Reset just calls [Clear]. This exists to speed up [proto.Reset].
func (m *Message) Reset() { m.Clear(nil) }

// Get implements [protoreflect.Message].
func (m *Message) Get(fd protoreflect.FieldDescriptor) protoreflect.Value {
	if !m.IsValid() {
		// We need to panic here because there's no "reasonable" way to return
		// a default for message-typed fields here.
		panic("called Get on nil fastpb.Message")
	}

	f := m.impl.Type().ByDescriptor(fd)
	if !f.IsValid() {
		return protoreflect.ValueOf(nil)
	}

	if v := f.Get(unsafe.Pointer(m)); v.IsValid() {
		// NOTE: non-scalar (message/repeated) fields always return a valid value.
		return v
	}
	return fd.Default()
}

// Set implements [protoreflect.Message].
//
// Panics when called.
func (m *Message) Set(protoreflect.FieldDescriptor, protoreflect.Value) {
	panic(dbg.Unsupported())
}

// Mutable implements [protoreflect.Message].
//
// Panics when called.
func (m *Message) Mutable(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(dbg.Unsupported())
}

// NewField implements [protoreflect.Message].
//
// Panics when called.
func (m *Message) NewField(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(dbg.Unsupported())
}

// GetUnknown implements [protoreflect.Message].
func (m *Message) GetUnknown() protoreflect.RawFields {
	cold := m.impl.Cold()
	if cold == nil {
		return nil
	}

	if cold.Unknown.Len() == 1 {
		return cold.Unknown.Ptr().Bytes(m.Shared().impl.Src)
	}

	var out []byte
	for _, zc := range cold.Unknown.Raw() {
		out = append(out, zc.Bytes(m.Shared().impl.Src)...)
	}
	return out
}

// SetUnknown implements [protoreflect.Message].
//
// Panics when called.
func (m *Message) SetUnknown(raw protoreflect.RawFields) {
	if len(raw) == 0 {
		return
	}
	panic(dbg.Unsupported())
}

// WhichOneof implements [protoreflect.Message].
func (m *Message) WhichOneof(od protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	if !m.IsValid() {
		return nil
	}

	fd := od.Fields().Get(0)
	f := m.impl.Type().ByDescriptor(fd)
	if !f.IsValid() {
		return nil
	}

	if f.Accessor.Offset.Number == 0 {
		// Not implemented internally as a oneof.
		if !m.Has(fd) {
			return nil
		}
		return fd
	}

	which := unsafe2.ByteLoad[uint32](m, f.Accessor.Offset.Bit)
	return fd.ContainingMessage().Fields().ByNumber(protoreflect.FieldNumber(which))
}

// IsValid implements [protoreflect.Message].
func (m *Message) IsValid() bool {
	return m != nil
}

// ProtoMethods implements [protoreflect.Message].
func (m *Message) ProtoMethods() *protoiface.Methods {
	return &m.impl.Type().Methods
}

// newMessage wraps an internal Message pointer.
func newMessage(s *dynamic.Message) *Message {
	return unsafe2.Cast[Message](s)
}

func unmarshalShim(in protoiface.UnmarshalInput) (out protoiface.UnmarshalOutput, err error) {
	m := in.Message.(*Message) //nolint:errcheck // Only called on *Message values.
	err = m.Unmarshal(in.Buf)
	return out, err
}

func requiredShim(in protoiface.CheckInitializedInput) (out protoiface.CheckInitializedOutput, err error) {
	// Required fields are not real.
	return out, nil
}

// mutableCold is like [message.mutableCold], but with a parser-friendly ABI.
func (p1 parser1) mutableCold(p2 parser2) (parser1, parser2, *dynamic.Cold) {
	if p2.m().ColdIndex < 0 {
		size := int(p2.m().Type().ColdSize)
		cold := unsafe2.Cast[dynamic.Cold](p1.arena().Alloc(size))
		p2.m().ColdIndex = int32(len(p1.c().Cold))
		p1.c().Cold = append(p1.c().Cold, cold)
		return p1, p2, cold
	}
	return p1, p2, unsafe2.LoadSlice(p1.c().Cold, p2.m().ColdIndex)
}

// getMutableField returns the field data for a given message. Uses p2.m and p2.f for
// the message and offset.
//
// If this field is in the cold region, it allocates one.
func getMutableField[T any](p1 parser1, p2 parser2) (parser1, parser2, *T) {
	var p unsafe.Pointer
	p1, p2, p = getUntypedMutableField(p1, p2)
	return p1, p2, (*T)(p)
}

//go:nosplit
func getUntypedMutableField(p1 parser1, p2 parser2) (parser1, parser2, unsafe.Pointer) {
	offset := p2.f().Offset.Data
	if offset >= 0 {
		return p1, p2, unsafe.Add(unsafe.Pointer(p2.m()), offset)
	}
	var cold *dynamic.Cold
	p1, p2, cold = p1.mutableCold(p2)
	return p1, p2, unsafe.Add(unsafe.Pointer(cold), ^offset)
}

// storeField loads a field pointer using [getMutableField] and stores v to it.
//
//nolint:unused
func storeField[T any](p1 parser1, p2 parser2, v T) (parser1, parser2) {
	var p unsafe.Pointer
	p1, p2, p = getUntypedMutableField(p1, p2)
	*(*T)(p) = v
	return p1, p2
}

// storeFromScratch is like storeField, but it uses p2.scratch as the value to
// store. This is useful for avoiding spills, by writing the temporary parsed
// value into the call-preserved scratch register.
func storeFromScratch[T integer](p1 parser1, p2 parser2) (parser1, parser2) {
	var p unsafe.Pointer
	p1, p2, p = getUntypedMutableField(p1, p2)
	*(*T)(p) = T(p2.scratch)
	return p1, p2
}

// setBit is like [message.setBit], but with a different ABI that keeps parser
// state in registers.
//
// It can only be used to set bits to true, and it draws m and n from
// p1 and p2.
func (p1 parser1) setBit(p2 parser2) (parser1, parser2) {
	n := int(p2.f().Offset.Bit)
	word := unsafe2.Add(unsafe2.Cast[uint32](unsafe2.Add(p2.m(), 1)), n/32)
	mask := uint32(1) << (n % 32)
	*word |= mask
	return p1, p2
}

//go:noinline
func (p1 parser1) alloc(p2 parser2) (parser1, parser2, *dynamic.Message) {
	ty := p1.c().Library().AtOffset(p2.f().Message.TypeOffset)
	size := int(ty.Size)

	// Open-coded copy of arena.Alloc, which otherwise would not inline.
	a := p1.arena()
	// Messages are always pointer-aligned, so we can skip this part.
	// size += arena.Align - 1
	// size &^= arena.Align - 1

	var n unsafe2.Addr[byte]
	n, a.Next = a.Next, a.Next.Add(size)
	if a.Next <= a.End {
		p := n.AssertValid()
		a.Log("alloc", "%v:%v, %d:%d", p, a.Next, size, arena.Align)

		// Go seems unwilling to inline allocInPlace() here.
		m := unsafe2.Cast[dynamic.Message](p)
		unsafe2.StoreNoWB(&m.Shared, p1.c())
		m.TypeOffset = p2.f().Message.TypeOffset
		m.ColdIndex = -1
		return p1, p2, m
	}

	a.Next = n
	a.Grow(size)

	// This call is guaranteed to not infinite recurse.
	// Doing a goto to the top of the function seems to confuse Go's register
	// allocator, causing it to spill p1 and p2 in the prologue.
	return p1.alloc(p2)
}

//go:nosplit
func (p1 parser1) allocInPlace(p2 parser2, data *byte) (parser1, parser2, *dynamic.Message) {
	m := unsafe2.Cast[dynamic.Message](data)
	unsafe2.StoreNoWB(&m.Shared, p1.c())
	m.TypeOffset = p2.f().Message.TypeOffset
	m.ColdIndex = -1
	return p1, p2, m
}
