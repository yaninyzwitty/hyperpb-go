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

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/tdp"
	"github.com/bufbuild/fastpb/internal/tdp/dynamic"
	"github.com/bufbuild/fastpb/internal/tdp/vm"
	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
)

const (
	// Custom field kinds used by archetype selection; they're all negative.
	proto2StringKind protoreflect.Kind = ^iota
)

func zigzag64[T tdp.Int](raw uint64) T {
	return zigzag(T(raw))
}

func zigzag[T tdp.Int](raw T) T {
	n := uint64(raw)
	n &= (1 << (unsafe.Sizeof(raw) * 8)) - 1

	return T(protowire.DecodeZigZag(n))
}

type (
	getterThunk func(*dynamic.Message, *tdp.Type, *tdp.Accessor) protoreflect.Value
)

// archetype represents a class of fields that have the same layout within a
// *message. This includes parsing and access information.
//
// Archetypes are used to organize field allocation and parsing strategies for
// use in the construction of a [fastpb.Type].
type archetype struct {
	// The layout for the field's storage in the message.
	layout layout.Layout
	// Bits to allocate for this field.
	bits uint32

	// Set if this is a oneof field.
	oneof bool

	// The getter thunk for this field.
	//
	// This func MUST be a reference to a function or a global closure, so that
	// it is not a GC-managed pointer.
	getter tdp.Getter

	// Parsers available for different forms of this field.
	parsers []parseKind
}

func adaptGetter(f getterThunk) tdp.Getter {
	return unsafe2.BitCast[tdp.Getter](f)
}

type parseKind struct {
	kind protowire.Type

	// If set, the parser will always retry this field instead of going to the
	// next one if it parses successfully. Used for repeated fields.
	retry bool

	// The bool return must always be true.
	//
	// This func MUST be a reference to a function or a global closure, so that
	// it is not a GC-managed pointer.
	parser vm.Thunk
}

// selectArchetype classifies a field into a particular archetype.
//
// prof is a profile that inquires for the profile of a field within the context
// of parsing fd. It takes a FieldDescriptor rather than a FieldSite because
// the caller is responsible for constructing the FieldSite.
//
// Returns nil if the field is not supported yet.
func selectArchetype(
	fd protoreflect.FieldDescriptor,
	prof FieldProfile,
) (a *archetype) {
	od := fd.ContainingOneof()
	switch {
	case fd.IsMap():
		k := fieldKind(fd.MapKey())
		v := fieldKind(fd.MapValue())
		a = mapFields[k][v]
	case fd.IsList():
		a = repeatedFields[fieldKind(fd)]
	case od != nil && od.Fields().Len() > 1:
		// One-element oneofs are treated like optional fields.
		a = oneofFields[fieldKind(fd)]
	case fd.HasPresence():
		a = optionalFields[fieldKind(fd)]
	default:
		a = singularFields[fieldKind(fd)]
	}

	return a
}

// fieldKind extracts the field kind from a descriptor.
//
// This returns negative values for "custom" kinds.
func fieldKind(fd protoreflect.FieldDescriptor) protoreflect.Kind {
	switch k := fd.Kind(); k {
	case protoreflect.StringKind:
		fd2, ok := fd.(interface{ EnforceUTF8() bool })
		if fd.Syntax() == protoreflect.Proto3 || (ok && fd2.EnforceUTF8()) {
			return protoreflect.StringKind
		}
		return proto2StringKind

	default:
		return k
	}
}
