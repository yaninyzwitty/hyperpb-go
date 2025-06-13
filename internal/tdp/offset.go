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

package tdp

import (
	"fmt"

	"github.com/bufbuild/fastpb/internal/dbg"
)

// Offset is field offset information for a generated message type's field.
type Offset struct {
	// First Bit index for bits allocated to this field.
	//
	// If this is a oneof field, this is instead an offset into the
	// oneof word table.
	Bit uint32

	// Byte offset within the containing message to the Data for this field.
	//
	// If negative, this is a cold field, and this is the negation of an offset
	// into the cold field area.
	Data int32

	// This field's Number. Only used by oneof fields; all other fields have
	// a zero here.
	Number uint32
}

// Format implements [fmt.Formatter].
func (o Offset) Format(s fmt.State, verb rune) {
	dbg.Fprintf("%d:%d:%#04x", o.Bit, o.Number, o.Data).Format(s, verb)
}
