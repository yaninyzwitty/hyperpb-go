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
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/tdp"
	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
)

// ir is analysis information about a message type for generating a parser
// and a dynamic type for it.
type ir struct {
	d protoreflect.MessageDescriptor

	// Each Protobuf field has three associated pieces of data that can be
	// sorted in different orders. There is the field inside of a [Type],
	// the field's parsers (which there may be more than one of per tField),
	// and the field's struct offsets (which may be shared by t).
	t []tField
	p []pField
	s []sField

	hot, cold int
	layout    tdp.TypeLayout
}

type tField struct {
	d      protoreflect.FieldDescriptor
	prof   FieldProfile
	arch   *Archetype
	offset tdp.Offset
}

type pField struct {
	tIdx int // Index in ir.t.
	aIdx int // Index in ir.t[tIdx].arch.parsers.

	hot  bool // If true, this parser should be in the "hot" part of the stream.
	next int  // The next parser to execute, as an index into ir.p.
}

type sField struct {
	tIdx []int // Index in ir.t. May be more than one!

	layout layout.Layout
	bits   uint32
	offset tdp.Offset
	hot    bool
}
