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

// Map is an empty, untyped, immutable [protoreflect.Map].
type Map struct{}

var _ protoreflect.Map = Map{}

func (m Map) IsValid() bool                { return false }
func (m Map) Len() int                     { return 0 }
func (m Map) Has(protoreflect.MapKey) bool { return false }
func (m Map) Get(protoreflect.MapKey) protoreflect.Value {
	return protoreflect.ValueOf(nil)
}
func (m Map) Range(f func(protoreflect.MapKey, protoreflect.Value) bool) {}

func (m Map) Clear(protoreflect.MapKey)                      {}
func (m Map) Set(protoreflect.MapKey, protoreflect.Value)    { panic(debug.Unsupported()) }
func (m Map) Mutable(protoreflect.MapKey) protoreflect.Value { panic(debug.Unsupported()) }
func (m Map) NewValue() protoreflect.Value                   { panic(debug.Unsupported()) }
