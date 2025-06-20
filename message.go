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

	"github.com/bufbuild/fastpb/internal/debug"
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
// is 4.
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

// Descriptor returns message descriptor, which contains only the Protobuf
// type information for the message.
//
// Descriptor implements [protoreflect.Message].
func (m *Message) Descriptor() protoreflect.MessageDescriptor {
	return m.impl.Type().Descriptor
}

// Type returns the message type, which encapsulates both Go and Protobuf
// type information. If the Go type information is not needed,
// it is recommended that the message descriptor be used instead.
//
// Type implements [protoreflect.Message]; Always returns *[Type].
func (m *Message) Type() protoreflect.MessageType {
	return newType(m.impl.Type())
}

// New returns a newly allocated empty message.
//
// New implements [protoreflect.Message].
func (m *Message) New() protoreflect.Message {
	return newType(m.impl.Type()).New()
}

// Interface returns m.
//
// Interface implements [protoreflect.Message].
func (m *Message) Interface() protoreflect.ProtoMessage {
	return m
}

// Range iterates over every populated field in an undefined order,
// calling f for each field descriptor and value encountered.
// Range returns immediately if f returns false.
//
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

// Has reports whether a field is populated.
//
// Some fields have the property of nullability where it is possible to
// distinguish between the default value of a field and whether the field
// was explicitly populated with the default value. Singular message fields,
// member fields of a oneof, and proto2 scalar fields are nullable. Such
// fields are populated only if explicitly set.
//
// In other cases (aside from the nullable cases above),
// a proto3 scalar field is populated if it contains a non-zero value, and
// a repeated field is populated if it is non-empty.
//
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

// Clear panics, unless this message has not been unmarshaled yet.
//
// Clear implements [protoreflect.Message].
func (m *Message) Clear(protoreflect.FieldDescriptor) {
	if m.Shared().impl.Src == nil {
		return
	}
	panic(debug.Unsupported())
}

// Reset panics, unless this message has not been unmarshaled yet
//
// Implements an interface used to speed up [proto.Reset].
func (m *Message) Reset() { m.Clear(nil) }

// Get retrieves the value for a field.
//
// For unpopulated scalars, it returns the default value, where
// the default value of a bytes scalar is guaranteed to be a copy.
// For unpopulated composite types, it returns an empty, read-only view
// of the value.
//
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

// Set panics.
//
// Set implements [protoreflect.Message].
func (m *Message) Set(protoreflect.FieldDescriptor, protoreflect.Value) {
	panic(debug.Unsupported())
}

// Mutable panics.
//
// Mutable implements [protoreflect.Message].
func (m *Message) Mutable(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(debug.Unsupported())
}

// NewField panics.
//
// NewField implements [protoreflect.Message].
func (m *Message) NewField(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(debug.Unsupported())
}

// WhichOneof reports which field within the oneof is populated,
// returning nil if none are populated.
// It panics if the oneof descriptor does not belong to this message.
//
// WhichOneof implements [protoreflect.Message].
func (m *Message) WhichOneof(od protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	if !m.IsValid() {
		return nil
	}

	fd := od.Fields().Get(0)
	f := m.impl.Type().ByDescriptor(fd)
	if !f.IsValid() {
		panic("invalid oneof descriptor " + string(od.FullName()) + " for message " + string(m.Descriptor().FullName()))
	}

	if f.Offset.Number == 0 {
		// Not implemented internally as a oneof.
		if !m.Has(fd) {
			return nil
		}
		return fd
	}

	which := unsafe2.ByteLoad[uint32](m, f.Offset.Bit)
	return fd.ContainingMessage().Fields().ByNumber(protoreflect.FieldNumber(which))
}

// GetUnknown retrieves the entire list of unknown fields.
//
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

// SetUnknown panics, unless raw is zero-length, in which case it does nothing.
//
// SetUnknown implements [protoreflect.Message].
func (m *Message) SetUnknown(raw protoreflect.RawFields) {
	if len(raw) == 0 {
		return
	}
	panic(debug.Unsupported())
}

// IsValid reports whether the message is valid.
//
// An invalid message is an empty, read-only value.
//
// An invalid message often corresponds to a nil pointer of the concrete
// message type, but the details are implementation dependent.
// Validity is not part of the protobuf data model, and may not
// be preserved in marshaling or other operations.
//
// IsValid implements [protoreflect.Message].
func (m *Message) IsValid() bool {
	return m != nil
}

// ProtoMethods returns optional fast-path implementations of various operations.
//
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
