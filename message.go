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
	"sync"
	"unsafe"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Message is a dynamic message value constructed with this package.
//
// Messages types returned by this package implement this interface.
// The Type() function on [Message] will return a [fastpb.Type]. Any functions
// that mutate the underlying message may panic.
type Message interface {
	proto.Message

	// Context returns the context that owns this message.
	Context() *Context

	// Unmarshal is like [proto.Unmarshal], but permits fastpb-specific
	// tuning options to be set.
	//
	// Calling this function may be much faster than calling proto.Unmarshal if
	// the message is small; proto.Unmarshal includes several nanoseconds of
	// overhead that can become noticeable for message in the 16 byte regime.
	//
	// The returned error may additionally implement a method with the signature
	//
	//	Offset() int
	//
	// This function will return the approximate offset into data at which the
	// error occurred.
	Unmarshal([]byte, ...UnmarshalOption) error

	ty() Type
}

// New allocates a new message of the given type.
//
// See [Context.New].
func New(ty Type) Message {
	return new(Context).New(ty)
}

// message is a dynamic message value.
//
// A *message lives on some arena, and all of its submessages do too. Because
// arenas are designed such that if a pointer to any of its allocated data is
// reachable, the whole arena is reachable, simply holding a *message into
// the arena will keep everything else alive, including the *root, which is
// arena-
//
// This means that *message values not being directly operated on by the
// application do not need to be marked by the GC, because their memory already
// gets marked whenever the GC sweeps a *root. As such, all of the fields of
// a message are laid out in memory that follows it.
type message struct {
	context  *Context
	tyOffset uint32
	coldIdx  int32 // Index into context.cold; negative means no cold pointer.

	// Fields follow this. The bitset words are allocated immediately after
	// the end of the message, so they are easy to offset to.
	//
	// The field data follows that, and offsets into the field data are already
	// baked to include both the message header and the bitset words.
}

var (
	_ proto.Message        = new(message)
	_ protoreflect.Message = new(message)
)

// cold is portions of a message that are located in context.cold.
type cold struct {
	unknown arena.Slice[zc] // Unknown field chunks.
}

// getField returns the field data for a given message.
//
// Returns nil if the field is cold and there is no cold region allocated.
func getField[T any](m *message, offset fieldOffset) *T {
	if offset.data < 0 {
		cold := m.cold()
		if cold == nil {
			return nil
		}
		return unsafe2.Cast[T](unsafe2.ByteAdd(cold, ^offset.data))
	}
	return unsafe2.Cast[T](unsafe2.ByteAdd(m, offset.data))
}

// getField returns the field data for a given message. Uses p2.m and p2.f for
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
	offset := p2.f().offset.data
	if offset >= 0 {
		return p1, p2, unsafe.Add(unsafe.Pointer(p2.m()), offset)
	}
	var cold *cold
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

// getBit gets the value of the nth bit from this message's bitset.
func (m *message) getBit(n uint32) bool {
	size, _ := unsafe2.Layout[message]()
	word := unsafe2.ByteLoad[uint32](m, size+int(n)/32*4)
	mask := uint32(1) << (n % 32)
	return word&mask != 0
}

// setBit sets the value of the nth bit from this message's bitset.
func (m *message) setBit(n uint32, flag bool) {
	word := unsafe2.Add(unsafe2.Cast[uint32](unsafe2.Add(m, 1)), int(n)/32)
	mask := uint32(1) << (n % 32)

	if flag {
		*word |= mask
	} else {
		*word &^= mask
	}
}

// setBit is like [message.setBit], but with a different ABI that keeps parser
// state in registers.
//
// It can only be used to set bits to true, and it draws m and n from
// p1 and p2.
func (p1 parser1) setBit(p2 parser2) (parser1, parser2) {
	n := int(p2.f().offset.bit)
	word := unsafe2.Add(unsafe2.Cast[uint32](unsafe2.Add(p2.m(), 1)), n/32)
	mask := uint32(1) << (n % 32)
	*word |= mask
	return p1, p2
}

func (m *message) Context() *Context {
	return m.context
}

func (m *message) Unmarshal(data []byte, options ...UnmarshalOption) error {
	return startParse(m, data, options...)
}

func (m *message) ty() Type {
	return m.context.lib.fromOffset(m.tyOffset)
}

// cold returns a pointer to the cold region, or nil if it hasn't been allocated.
func (m *message) cold() *cold {
	if m.coldIdx < 0 {
		return nil
	}
	return unsafe2.LoadSlice(m.context.cold, m.coldIdx)
}

// mutableCold returns a pointer to the cold region, allocating one if needed.
func (m *message) mutableCold() *cold {
	if m.coldIdx < 0 {
		size := int(m.ty().raw.coldSize)
		cold := unsafe2.Cast[cold](m.context.arena.Alloc(size))
		m.coldIdx = int32(len(m.context.cold))
		m.context.cold = append(m.context.cold, cold)
		return cold
	}
	return unsafe2.LoadSlice(m.context.cold, m.coldIdx)
}

// mutableCold is like [message.mutableCold], but with a parser-friendly ABI.
func (p1 parser1) mutableCold(p2 parser2) (parser1, parser2, *cold) {
	if p2.m().coldIdx < 0 {
		size := int(p2.m().ty().raw.coldSize)
		cold := unsafe2.Cast[cold](p1.arena().Alloc(size))
		p2.m().coldIdx = int32(len(p1.c().cold))
		p1.c().cold = append(p1.c().cold, cold)
		return p1, p2, cold
	}
	return p1, p2, unsafe2.LoadSlice(p1.c().cold, p2.m().coldIdx)
}

func unmarshalShim(in protoiface.UnmarshalInput) (out protoiface.UnmarshalOutput, err error) {
	m := in.Message.(*message) //nolint:errcheck // Only called on *message values.
	err = m.Unmarshal(in.Buf)
	return out, err
}

func requiredShim(in protoiface.CheckInitializedInput) (out protoiface.CheckInitializedOutput, err error) {
	// Required fields are not real.
	return out, nil
}

// Context is state that is shared by all messages in a particular tree of
// messages.
//
// A zero context is ready to use.
type Context struct {
	// Context is the only memory not allocated on the arena.
	arena arena.Arena
	lib   *Library
	src   *byte
	len   int

	// Synchronizes calls to startParse() with this context.
	lock sync.Mutex

	// Off-arena memory which holds arena pointers to "cold" parts of a message.
	cold []*cold
}

// New allocates a new message in this context.
func (c *Context) New(ty Type) Message {
	if c == nil {
		c = new(Context)
	}

	// Previously, this code was here:
	//
	// // Easy mistake to make: the memory allocated in alloc() contains no
	// // pointers, so even though ty is "reachable" through m, it's not reachable
	// // from the GC's perspective, so we need to mark it as alive here.
	// //
	// // This implicitly marks all other types reachable from ty as alive, meaning
	// // we only need to do this for top-level calls to New().
	// c.arena.KeepAlive(ty)
	//
	// It is now redundant, because Context stores ty.Library(). The comment is
	// kept for posterity about a nasty bug.

	return c.alloc(ty)
}

// Free releases any resources held by this context, allowing them to be re-used.
//
// Any messages previously parsed using this context must not be reused.
func (c *Context) Free() {
	c.arena.Free()
	c.lib = nil
	c.src = nil

	clear(c.cold)
	c.cold = c.cold[:0]
}

func (c *Context) alloc(ty Type) *message {
	c.lock.Lock()
	defer c.lock.Unlock()

	switch c.lib {
	case nil:
		c.lib = ty.Library()
	case ty.Library():
		break
	default:
		panic("fastpb: attempted to mix messages from different fastpb.Library pointers")
	}

	data := c.arena.Alloc(int(ty.raw.size))
	m := unsafe2.Cast[message](data)
	unsafe2.StoreNoWB(&m.context, c)
	m.tyOffset = uint32(unsafe2.Sub(ty.raw, c.lib.base))
	m.coldIdx = -1
	return m
}

//go:noinline
func (p1 parser1) alloc(p2 parser2) (parser1, parser2, *message) {
	ty := p1.c().lib.fromOffset(p2.f().message.tyOffset)
	size := int(ty.raw.size)

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
		m := unsafe2.Cast[message](p)
		unsafe2.StoreNoWB(&m.context, p1.c())
		m.tyOffset = p2.f().message.tyOffset
		m.coldIdx = -1
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
func (p1 parser1) allocInPlace(p2 parser2, data *byte) (parser1, parser2, *message) {
	m := unsafe2.Cast[message](data)
	unsafe2.StoreNoWB(&m.context, p1.c())
	m.tyOffset = p2.f().message.tyOffset
	m.coldIdx = -1
	return p1, p2, m
}

// ProtoReflect implements [proto.Message].
func (m *message) ProtoReflect() protoreflect.Message {
	return m
}

// Descriptor implements [protoreflect.Message].
func (m *message) Descriptor() protoreflect.MessageDescriptor {
	return m.ty().Descriptor()
}

// Type implements {protoreflect.Message}.
func (m *message) Type() protoreflect.MessageType {
	return m.ty()
}

// New implements [protoreflect.Message].
func (m *message) New() protoreflect.Message {
	return m.ty().New()
}

// Interface implements [protoreflect.Message].
func (m *message) Interface() protoreflect.ProtoMessage {
	return m
}

// Range implements [protoreflect.Message].
func (m *message) Range(yield func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {
	if !m.IsValid() {
		return
	}

	f := m.ty().byIndex(0)
	i := 0
	for f.valid() {
		fd := m.ty().raw.aux.fds[i]
		v := f.get(m)
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

		if !yield(m.ty().raw.aux.fds[i], v) {
			return
		}

	skip:
		f = unsafe2.Add(f, 1)
		i++
	}
}

// Has implements [protoreflect.Message].
func (m *message) Has(fd protoreflect.FieldDescriptor) bool {
	if !m.IsValid() {
		return false
	}

	f := m.ty().byDescriptor(fd)
	if !f.valid() {
		return false
	}

	v := f.get(m)
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
func (m *message) Clear(protoreflect.FieldDescriptor) {
	if m.context.src == nil {
		return
	}
	panic(dbg.Unsupported())
}

// Reset just calls [Clear]. This exists to speed up [proto.Reset].
func (m *message) Reset() { m.Clear(nil) }

// Get implements [protoreflect.Message].
func (m *message) Get(fd protoreflect.FieldDescriptor) protoreflect.Value {
	if !m.IsValid() {
		// We need to panic here because there's no "reasonable" way to return
		// a default for message-typed fields here.
		panic("called Get on nil fastpb.Message")
	}

	f := m.ty().byDescriptor(fd)
	if !f.valid() {
		return protoreflect.ValueOf(nil)
	}

	if v := f.get(m); v.IsValid() {
		// NOTE: non-scalar (message/repeated) fields always return a valid value.
		return v
	}
	return fd.Default()
}

// Set implements [protoreflect.Message].
//
// Panics when called.
func (m *message) Set(protoreflect.FieldDescriptor, protoreflect.Value) {
	panic(dbg.Unsupported())
}

// Mutable implements [protoreflect.Message].
//
// Panics when called.
func (m *message) Mutable(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(dbg.Unsupported())
}

// NewField implements [protoreflect.Message].
//
// Panics when called.
func (m *message) NewField(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(dbg.Unsupported())
}

// GetUnknown implements [protoreflect.Message].
func (m *message) GetUnknown() protoreflect.RawFields {
	cold := m.cold()
	if cold == nil {
		return nil
	}

	if cold.unknown.Len() == 1 {
		return cold.unknown.Ptr().bytes(m.context.src)
	}

	var out []byte
	for _, zc := range cold.unknown.Raw() {
		out = append(out, zc.bytes(m.context.src)...)
	}
	return out
}

// SetUnknown implements [protoreflect.Message].
//
// Panics when called.
func (m *message) SetUnknown(raw protoreflect.RawFields) {
	if len(raw) == 0 {
		return
	}
	panic(dbg.Unsupported())
}

// WhichOneof implements [protoreflect.Message].
func (m *message) WhichOneof(od protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	if !m.IsValid() {
		return nil
	}

	fd := od.Fields().Get(0)
	f := m.ty().byDescriptor(fd)
	if !f.valid() {
		return nil
	}

	if f.getter.offset.number == 0 {
		// Not implemented internally as a oneof.
		if !m.Has(fd) {
			return nil
		}
		return fd
	}

	which := unsafe2.ByteLoad[uint32](m, f.getter.offset.bit)
	return fd.ContainingMessage().Fields().ByNumber(protoreflect.FieldNumber(which))
}

// IsValid implements [protoreflect.Message].
func (m *message) IsValid() bool {
	return m != nil
}

// ProtoMethods implements [protoreflect.Message].
func (m *message) ProtoMethods() *protoiface.Methods {
	return &m.ty().raw.aux.methods
}

// empty is an empty value of any [Type].
type empty struct{ ty Type }

var (
	_ proto.Message        = empty{}
	_ protoreflect.Message = empty{}
)

// ProtoReflect implements [proto.Message].
func (e empty) ProtoReflect() protoreflect.Message {
	return e
}

// Descriptor implements [protoreflect.Message].
func (e empty) Descriptor() protoreflect.MessageDescriptor {
	return e.ty.Descriptor()
}

// Type implements {protoreflect.Message}.
func (e empty) Type() protoreflect.MessageType {
	return e.ty
}

// New implements [protoreflect.Message].
func (e empty) New() protoreflect.Message {
	return e.ty.New()
}

// Interface implements [protoreflect.Message].
func (e empty) Interface() protoreflect.ProtoMessage {
	return e
}

// Range implements [protoreflect.Message].
func (e empty) Range(yield func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {}

// Has implements [protoreflect.Message].
func (e empty) Has(fd protoreflect.FieldDescriptor) bool {
	return false
}

// Clear implements [protoreflect.Message].
func (e empty) Clear(protoreflect.FieldDescriptor) {}

// Get implements [protoreflect.Message].
func (e empty) Get(fd protoreflect.FieldDescriptor) protoreflect.Value {
	f := e.ty.byDescriptor(fd)
	if !f.valid() {
		return protoreflect.ValueOf(nil)
	}

	switch {
	case fd.IsList():
		return protoreflect.ValueOf(emptyList{})

	case fd.IsMap():
		panic(dbg.Unsupported())

	case fd.Message() != nil:
		return protoreflect.ValueOf(empty{f.message})

	default:
		return fd.Default()
	}
}

// Set implements [protoreflect.Message].
//
// Panics when called.
func (e empty) Set(protoreflect.FieldDescriptor, protoreflect.Value) {
	panic(dbg.Unsupported())
}

// Mutable implements [protoreflect.Message].
//
// Panics when called.
func (e empty) Mutable(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(dbg.Unsupported())
}

// NewField implements [protoreflect.Message].
//
// Panics when called.
func (e empty) NewField(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(dbg.Unsupported())
}

// GetUnknown implements [protoreflect.Message].
func (e empty) GetUnknown() protoreflect.RawFields {
	return nil
}

// SetUnknown implements [protoreflect.Message].
//
// Panics when called.
func (e empty) SetUnknown(raw protoreflect.RawFields) {
	if len(raw) == 0 {
		return
	}
	panic(dbg.Unsupported())
}

// WhichOneof implements [protoreflect.Message].
func (e empty) WhichOneof(protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	return nil
}

// IsValid implements [protoreflect.Message].
func (e empty) IsValid() bool {
	return false
}

// ProtoMethods implements [protoreflect.Message].
func (e empty) ProtoMethods() *protoiface.Methods {
	return &e.ty.raw.aux.methods
}
