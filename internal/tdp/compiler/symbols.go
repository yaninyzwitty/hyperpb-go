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
	"fmt"

	"google.golang.org/protobuf/reflect/protoreflect"
)

type typeSymbol struct {
	ty protoreflect.MessageDescriptor
}

type parserSymbol struct {
	ty       protoreflect.MessageDescriptor
	mapEntry bool
}

type tableSymbol struct{ sym any }

type fieldParserSymbol struct {
	parser any
	index  int
}

func (s typeSymbol) String() string {
	return fmt.Sprintf("type:%q", s.ty.FullName())
}

func (s parserSymbol) String() string {
	return fmt.Sprintf("parser:%q:%v", s.ty.FullName(), s.mapEntry)
}
