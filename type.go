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
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/tdp"
	"github.com/bufbuild/fastpb/internal/tdp/empty"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Type is an optimized message descriptor.
//
// *Type implements [protoreflect.MessageType].
type Type struct {
	impl tdp.Type
}

// Descriptor returns the message descriptor.
//
// Descriptor implements [protoreflect.MessageType].
func (t *Type) Descriptor() protoreflect.MessageDescriptor {
	if t == nil {
		return nil
	}
	return t.impl.Descriptor
}

// New returns a newly allocated empty message.
// It may return nil for synthetic messages representing a map entry.
//
// New implements [protoreflect.MessageType].
func (t *Type) New() protoreflect.Message {
	return new(Shared).New(t).ProtoReflect()
}

// Zero returns an empty, read-only message.
// It may return nil for synthetic messages representing a map entry.
//
// Zero implements [protoreflect.MessageType].
func (t *Type) Zero() protoreflect.Message {
	return empty.NewMessage(&t.impl)
}

// Format implements [fmt.Formatter].
func (t *Type) Format(f fmt.State, verb rune) {
	if f.Flag('#') {
		fmt.Fprintf(f, fmt.FormatString(f, verb), t.Descriptor())
	} else {
		fmt.Fprint(f, t.Descriptor().FullName())
	}
}

// newType wraps an internal Type pointer.
func newType(s *tdp.Type) *Type {
	return unsafe2.Cast[Type](s)
}

func init() {
	empty.WrapType = func(t *tdp.Type) protoreflect.MessageType {
		return newType(t)
	}
}
