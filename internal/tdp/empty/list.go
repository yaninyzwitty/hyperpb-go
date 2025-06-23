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

package empty

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/hyperpb/internal/debug"
)

// List is an empty, untyped, immutable [protoreflect.List].
type List struct{}

var _ protoreflect.List = List{}

func (List) IsValid() bool { return false }
func (List) Len() int      { return 0 }
func (List) Get(n int) protoreflect.Value {
	_ = []byte{}[n] // Trigger a bounds check.
	return protoreflect.Value{}
}

func (List) Append(protoreflect.Value)         { panic(debug.Unsupported()) }
func (List) AppendMutable() protoreflect.Value { panic(debug.Unsupported()) }
func (List) NewElement() protoreflect.Value    { panic(debug.Unsupported()) }
func (List) Set(int, protoreflect.Value)       { panic(debug.Unsupported()) }
func (List) Truncate(int)                      { panic(debug.Unsupported()) }
