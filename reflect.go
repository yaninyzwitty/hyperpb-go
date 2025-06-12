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
	"fmt"
	"unsafe"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Helpers for working around inefficiencies in protoreflect.

// rawValue matches the layout of protoreflect.Value exactly.
type rawValue struct {
	// It is slightly funny that they store typ as an unsafe.Pointer, since most
	// of the runtime stores itabs as uintptrs, because all itabs live in
	// permanent memory.
	typ uintptr
	_   unsafe.Pointer
	num uint64
}

// reflectValueScalar extracts a scalar value out of a protoreflect.Value.
//
// Panics if this is the wrong type.
func reflectValueScalar[T integer](v protoreflect.Value) T {
	var z T
	rv := unwrapReflectValue(v)
	if rv.typ != unsafe2.AnyType(z) {
		panic(reflectValuePanicMessage(protoreflect.ValueOf(z), v))
	}

	return T(rv.num)
}

// unwrapValue unwraps a protoreflect.Value so that we can access its internal
// structures.
func unwrapReflectValue(v protoreflect.Value) rawValue {
	return unsafe2.BitCast[rawValue](v)
}

func reflectValuePanicMessage(want, got protoreflect.Value) string {
	return fmt.Sprintf("type mismatch: cannot convert %v to %s",
		reflectValueTypeName(want), reflectValueTypeName(got))
}

// reflectTypeName is a copy of [protoreflect.Value.typeName].
func reflectValueTypeName(v protoreflect.Value) string {
	switch v.Interface().(type) {
	case nil:
		return "nil"
	case bool:
		return "bool"
	case int32:
		return "int32"
	case int64:
		return "int64"
	case uint32:
		return "uint32"
	case uint64:
		return "uint64"
	case float32:
		return "float32"
	case float64:
		return "float64"
	case string:
		return "string"
	case []byte:
		return "bytes"
	case protoreflect.EnumNumber:
		return "enum"
	case protoreflect.Message:
		return "message"
	case protoreflect.List:
		return "list"
	case protoreflect.Map:
		return "map"
	default:
		return fmt.Sprintf("<unknown: %T>", v)
	}
}
