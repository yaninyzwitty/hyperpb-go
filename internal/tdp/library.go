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
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/hyperpb/internal/unsafe2"
)

// Library represents the full output of [Compile]. Given any [Type], it can
// be used to obtain the [Type] for any other message type that was compiled
// as part of the same call to [Compile].
type Library struct {
	Base  *Type
	Types map[protoreflect.MessageDescriptor]*Type
	Bytes int

	// Used to store compilation metadata. Actually a []hyperpb.CompileOptions.
	Metadata any
}

// Type returns the [Type] for the given descriptor in this library.
//
// If not present, returns false.
func (l *Library) Type(md protoreflect.MessageDescriptor) (*Type, bool) {
	t, ok := l.Types[md]
	return t, ok
}

// AtOffset the [Type] at the give byte offset in this Library.
func (l *Library) AtOffset(n uint32) *Type {
	return unsafe2.ByteAdd[Type](l.Base, n)
}
