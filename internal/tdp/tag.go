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

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Tag is a specially-formatted tag for the parser.
//
// The tag is formatted in the way it would be when encoded in a Protobuf
// message, but with the high bit of each byte cleared.
type Tag uint64

// encode encodes this field tag from the given number and type.
func EncodeTag(n protowire.Number, t protowire.Type) Tag {
	var tag Tag
	protowire.AppendTag(unsafe2.Bytes(&tag)[:0], n, t)
	tag &^= SignBits
	return tag
}

// Decode decodes this field tag into a number and a type.
func (t Tag) Decode() uint64 {
	var tag uint64
	mask := uint64(0x7f)
	i := 0

	tag |= (uint64(t) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(t) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(t) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(t) & mask) >> i
	mask <<= 8
	i++
	tag |= (uint64(t) & mask) >> i
	mask <<= 8
	i++

	_, _ = i, mask
	return tag
}

// Format implements [fmt.Formatter].
func (t Tag) Format(s fmt.State, verb rune) {
	v := t.Decode()
	n, ty := protowire.DecodeTag(v)
	dbg.Fprintf("%#x:%d:%d", uint64(t), n, ty).Format(s, verb)
}
