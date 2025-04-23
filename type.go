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
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/runtime/protoiface"

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/table"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Type is an optimized message descriptor.
//
// Type implements [protoreflect.MessageType].
type Type struct {
	// Pointer into The flattened graph containing this descriptor.
	raw *typeHeader
}

// NOTE: DO NOT USE *Type INSTEAD OF Type.
//
// Because Type's shape is equal to a pointer, Go will inline it into
// the data field of an interface, avoiding an indirection.
var _ protoreflect.MessageType = Type{}

// Descriptor implements [protoreflect.MessageType].
func (t Type) Descriptor() protoreflect.MessageDescriptor {
	if t.raw == nil {
		return nil
	}
	return t.raw.aux.desc
}

// New implements [protoreflect.MessageType].
func (t Type) New() protoreflect.Message {
	return New(t).ProtoReflect()
}

// Zero implements [protoreflect.MessageType].
func (t Type) Zero() protoreflect.Message {
	return empty{t}
}

// Format implements [fmt.Formatter].
func (t Type) Format(f fmt.State, verb rune) {
	if f.Flag('#') {
		fmt.Fprintf(f, fmt.FormatString(f, verb), t.Descriptor())
	} else {
		fmt.Fprint(f, t.Descriptor().FullName())
	}
}

// byIndex returns the nth byIndex (in byIndex number order) for this type.
//
// If n == 0 and this type has no fields, returns a byIndex with an invalid byIndex number.
//
// This function does not perform bounds checks.
func (t Type) byIndex(n int) *field {
	return unsafe2.Beyond[field](t.raw).Get(n)
}

// byNumber returns the field with the given number.
func (t Type) byDescriptor(fd protoreflect.FieldDescriptor) *field {
	i := int(t.raw.count)

	if fd.ContainingMessage() == t.Descriptor() && !fd.IsExtension() {
		i = fd.Index()
	}
	return t.byIndex(i)
}

// typeHeader is the raw header for compiled [Type] information. Each [Type]
// is a pointer to such a header.
type typeHeader struct {
	_   unsafe2.NoCopy
	aux *typeAux

	// The number of bytes of memory that must be allocated for a *message of
	// this type. This includes the size of the header. Alignment is implicitly
	// that of uint64.
	size uint32

	// The "unspecialized" parser for this type.
	parser *typeParser

	// Maps field numbers to offsets in fields.
	numbers table.Table[uint32]

	// The number of count that follow this type, not including the special
	// padding field with number equal to zero.
	count uint32

	// Followed by:
	// 1. An array of fields of length equal to count+1.
	// 2. A table.Table that maps field numbers to entires in the
	//    aforementioned field table.
}

// typeAux is data on a typeHeader that is stored behind a pointer and kept
// alive in the traces struct in [compiler.compile]. These rarely-accessed
// fields ensure that parser-relevant data is closer together in cache.
type typeAux struct {
	layout dbg.Value[typeLayout]

	desc    protoreflect.MessageDescriptor
	methods protoiface.Methods
}

// typeLayout is layout information for a [Type]. Only for debugging.
type typeLayout struct {
	bitWords int
	fields   []fieldLayout // Sorted in offset order.
}

// fieldLayout is layout information for a [field]. Only for debugging.
type fieldLayout struct {
	arch    *archetype
	index   int // Which field is this in the MessageDescriptor?
	offset  fieldOffset
	padding int // Tail padding after this field.
}

// typeParser is a parser for some [Type]. A [Type] may have multiple parsers.
type typeParser struct {
	_ unsafe2.NoCopy

	// The type that this parser parses.
	ty Type

	// Maps field tags to offsets in fields.
	tags table.Table[uint32]

	entry fieldParser

	// Followed by an unspecified number of fieldParser values.
}

func (p *typeParser) fields() *unsafe2.VLA[fieldParser] {
	return unsafe2.Beyond[fieldParser](p)
}
