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

package compiler

import (
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/vm"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

// Archetype represents a class of fields that have the same layout within a
// *message. This includes parsing and access information.
//
// Archetypes are used to organize field allocation and parsing strategies for
// use in the construction of a [hyperpb.Type].
type Archetype struct {
	// The Layout for the field's storage in the message.
	Layout layout.Layout
	// Bits to allocate for this field.
	Bits uint32

	// Set if this is a Oneof field.
	Oneof bool

	// The Getter thunk for this field.
	//
	// This func MUST be a reference to a function or a global closure, so that
	// it is not a GC-managed pointer.
	Getter Getter

	// Parsers available for different forms of this field.
	Parsers []Parser
}

// Parser is a parser within an [Archetype].
type Parser struct {
	Kind protowire.Type

	// If set, the parser will always Retry this field instead of going to the
	// next one if it parses successfully. Used for repeated fields.
	Retry bool

	// The bool return must always be true.
	//
	// This func MUST be a reference to a function or a global closure, so that
	// it is not a GC-managed pointer.
	Thunk vm.Thunk
}

// Getter is a strongly-typed version of [tdp.Getter].
type Getter func(*dynamic.Message, *tdp.Type, *tdp.Accessor) protoreflect.Value

func (g Getter) adapt() tdp.Getter {
	return xunsafe.BitCast[tdp.Getter](g)
}
