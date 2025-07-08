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

package hyperpb

import (
	"fmt"
	"slices"
	_ "unsafe"

	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/empty"
	"buf.build/go/hyperpb/internal/tdp/profile"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// MessageType implements [protoreflect.MessageType].
//
// To obtain an optimized [MessageType], use any of the Compile* functions.
type MessageType struct {
	impl tdp.Type
}

// Profile can be used to profile messages associated with the same
// collection of types. It can later be used to re-compile the original types
// to be more efficient.
//
// Profile itself is an opaque pointer; it only exists to be passed into
// different calls to [WithProfile].
//
// See [MessageType.NewProfile].
type Profile struct {
	impl profile.Recorder
}

// Descriptor returns the message descriptor.
//
// Descriptor implements [protoreflect.MessageType].
func (t *MessageType) Descriptor() protoreflect.MessageDescriptor {
	if t == nil {
		return nil
	}
	return t.impl.Descriptor
}

// New returns a newly allocated empty message.
// It may return nil for synthetic messages representing a map entry.
//
// New implements [protoreflect.MessageType].
func (t *MessageType) New() protoreflect.Message {
	return new(Shared).NewMessage(t).ProtoReflect()
}

// Zero returns an empty, read-only message.
// It may return nil for synthetic messages representing a map entry.
//
// Zero implements [protoreflect.MessageType].
func (t *MessageType) Zero() protoreflect.Message {
	return empty.NewMessage(&t.impl)
}

// Format implements [fmt.Formatter].
func (t *MessageType) Format(f fmt.State, verb rune) {
	if f.Flag('#') {
		fmt.Fprintf(f, fmt.FormatString(f, verb), t.Descriptor())
	} else {
		fmt.Fprint(f, t.Descriptor().FullName())
	}
}

// NewProfile creates a new profiler for this type, which can be used to
// profile messages of this type when unmarshaling.
//
// The returned profiler cannot be used with messages of other types.
func (t *MessageType) NewProfile() *Profile {
	return xunsafe.Cast[Profile](profile.NewRecorder(t.impl.Library))
}

// Recompile recompiles this type with a recorded profile.
//
// Note that this profile cannot be used with the new type; you must create a
// fresh profile using [MessageType.NewProfile] and begin recording anew.
func (t *MessageType) Recompile(profile *Profile) *MessageType {
	options := slices.Clone(t.impl.Library.Metadata.([]CompileOption)) //nolint:errcheck
	options = append(options, WithProfile(profile))

	return CompileMessageDescriptor(t.Descriptor(), options...)
}

// wrapType wraps an internal Type pointer.
//
//go:linkname wrapType buf.build/go/hyperpb/internal/tdp/empty.wrapType
func wrapType(s *tdp.Type) *MessageType {
	return xunsafe.Cast[MessageType](s)
}
