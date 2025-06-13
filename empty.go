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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/bufbuild/fastpb/internal/dbg"
)

// empty is an empty value of any [Type].
type empty struct{ ty *Type }

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
	f := e.ty.impl.ByDescriptor(fd)
	if !f.IsValid() {
		return protoreflect.ValueOf(nil)
	}

	switch {
	case fd.IsList():
		return protoreflect.ValueOf(emptyList{})

	case fd.IsMap():
		panic(dbg.Unsupported())

	case fd.Message() != nil:
		return protoreflect.ValueOf(empty{newType(f.Message)})

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
	return &e.ty.impl.Methods
}

// emptyList is an empty untyped list.
type emptyList struct {
	immutableList
}

func (emptyList) IsValid() bool { return false }
func (emptyList) Len() int      { return 0 }
func (emptyList) Get(n int) protoreflect.Value {
	_ = []byte{}[n] // Trigger a bounds check.
	return protoreflect.Value{}
}
