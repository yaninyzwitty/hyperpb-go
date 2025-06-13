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

package empty

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/tdp"
)

// Callback for constructing the actual MessageType implementation defined in
// the root package.
var WrapType func(*tdp.Type) protoreflect.MessageType

// Message is an Message value of any [Type].
type Message struct{ ty *tdp.Type }

// NewMessage creates a new, empty message.
func NewMessage(ty *tdp.Type) Message {
	return Message{ty}
}

// ProtoReflect implements [proto.Message].
func (e Message) ProtoReflect() protoreflect.Message {
	return e
}

// Descriptor implements [protoreflect.Message].
func (e Message) Descriptor() protoreflect.MessageDescriptor {
	return e.ty.Descriptor
}

// Type implements {protoreflect.Message}.
func (e Message) Type() protoreflect.MessageType {
	return WrapType(e.ty)
}

// New implements [protoreflect.Message].
func (e Message) New() protoreflect.Message {
	return e.Type().New()
}

// Interface implements [protoreflect.Message].
func (e Message) Interface() protoreflect.ProtoMessage {
	return e
}

// Range implements [protoreflect.Message].
func (e Message) Range(yield func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {}

// Has implements [protoreflect.Message].
func (e Message) Has(fd protoreflect.FieldDescriptor) bool {
	return false
}

// Clear implements [protoreflect.Message].
func (e Message) Clear(protoreflect.FieldDescriptor) {}

// Get implements [protoreflect.Message].
func (e Message) Get(fd protoreflect.FieldDescriptor) protoreflect.Value {
	f := e.ty.ByDescriptor(fd)
	if !f.IsValid() {
		return protoreflect.ValueOf(nil)
	}

	switch {
	case fd.IsList():
		return protoreflect.ValueOf(List{})

	case fd.IsMap():
		panic(dbg.Unsupported())

	case fd.Message() != nil:
		return protoreflect.ValueOf(Message{f.Message})

	default:
		return fd.Default()
	}
}

// Set implements [protoreflect.Message].
//
// Panics when called.
func (e Message) Set(protoreflect.FieldDescriptor, protoreflect.Value) {
	panic(dbg.Unsupported())
}

// Mutable implements [protoreflect.Message].
//
// Panics when called.
func (e Message) Mutable(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(dbg.Unsupported())
}

// NewField implements [protoreflect.Message].
//
// Panics when called.
func (e Message) NewField(protoreflect.FieldDescriptor) protoreflect.Value {
	panic(dbg.Unsupported())
}

// GetUnknown implements [protoreflect.Message].
func (e Message) GetUnknown() protoreflect.RawFields {
	return nil
}

// SetUnknown implements [protoreflect.Message].
//
// Panics when called.
func (e Message) SetUnknown(raw protoreflect.RawFields) {
	if len(raw) == 0 {
		return
	}
	panic(dbg.Unsupported())
}

// WhichOneof implements [protoreflect.Message].
func (e Message) WhichOneof(protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	return nil
}

// IsValid implements [protoreflect.Message].
func (e Message) IsValid() bool {
	return false
}

// ProtoMethods implements [protoreflect.Message].
func (e Message) ProtoMethods() *protoiface.Methods {
	return nil
}

var (
	_ proto.Message        = Message{}
	_ protoreflect.Message = Message{}
)
