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
	"math"
	"unsafe"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/tdp/dynamic"
	"github.com/bufbuild/fastpb/internal/tdp/empty"
	"github.com/bufbuild/fastpb/internal/tdp/thunks"
	"github.com/bufbuild/fastpb/internal/tdp/vm"
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

// New allocates a new [Message] of the given [Type].
//
// See [Shared.New].
func New(ty *Type) *Message {
	return new(Shared).New(ty)
}

// Unmarshal is like [proto.Unmarshal], but permits fastpb-specific
// tuning options to be set.
//
// Calling this function may be much faster than calling proto.Unmarshal ifAdd commentMore actions
// the message is small; proto.Unmarshal includes several nanoseconds of
// overhead that can become noticeable for message in the 16 byte regime.
//
// The returned error may additionally implement a method with the signature
//
//	Offset() int
//
// This function will return the approximate offset into data at which the
// error occurred.
func (m *Message) Unmarshal(data []byte, options ...UnmarshalOption) error {
	opts := vm.NewOptions()
	for _, opt := range options {
		if opt != nil {
			opt(&opts)
		}
	}
	return vm.Run(&m.impl, data, opts)
}

// CompileOption is a configuration setting for [Type.Unmarshal].
type UnmarshalOption func(*vm.Options)

// WithMaxDecodeMisses sets the number of decode misses allowed in the parser before
// switching to the slow path.
//
// Large values may improve performance for common protos, but introduce a
// potential DoS vector due to quadratic worst case performance. The default
// is 8.
func WithMaxDecodeMisses(n int) UnmarshalOption {
	return func(opts *vm.Options) { opts.MaxMisses = n }
}

// WithMaxDepth sets the maximum recursion depth for the parser.
//
// Setting a large value enables potential DoS vectors.
func WithMaxDepth(n int) UnmarshalOption {
	return func(opts *vm.Options) { opts.MaxDepth = min(n, math.MaxUint32) }
}

// Shared returns state shared by this message and its submessages.
func (m *Message) Shared() *Shared {
	return newShared(m.impl.Shared)
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
			if _, empty := v.Interface().(empty.Message); empty {
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
		_, empty := v.Interface().(empty.Message)
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
func newMessage(m *dynamic.Message) *Message {
	return unsafe2.Cast[Message](m)
}

func init() {
	thunks.WrapMessage = func(m *dynamic.Message) protoreflect.Message {
		return newMessage(m)
	}
}

// unmarshalShim implements [protoiface.Methods].Unmarshal.
func unmarshalShim(in protoiface.UnmarshalInput) (out protoiface.UnmarshalOutput, err error) {
	m := in.Message.(*Message) //nolint:errcheck // Only called on *Message values.
	err = m.Unmarshal(in.Buf)
	return out, err
}

// requiredShim implements [protoiface.Methods].CheckInitialized.
func requiredShim(in protoiface.CheckInitializedInput) (out protoiface.CheckInitializedOutput, err error) {
	// Required fields are not real.
	return out, nil
}

var (
	_ proto.Message        = new(Message)
	_ protoreflect.Message = new(Message)
)
