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

package unsafe2

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

// iface is the internal representation an a Go interface value.
type iface struct {
	itab uintptr
	data *byte
}

// AnyData extracts the pointer value from an any.
func AnyData(v any) *byte {
	return Cast[iface](&v).data
}

// AnyData extracts the opaque type from an any.
func AnyType(v any) uintptr {
	return Cast[iface](&v).itab
}

// AnyBytes extracts a slice pointing to the variable-length data of an any.
func AnyBytes(v any) []byte {
	if v == nil {
		return nil
	}

	t := reflect.TypeOf(v)
	p := AnyData(v)
	if t.Kind() == reflect.Pointer || t.Kind() == reflect.UnsafePointer {
		p = Cast[byte](&p)
	}

	return unsafe.Slice(p, reflect.TypeOf(v).Size())
}

// InlinedAny returns whether converting T into an interface requires calling
// the allocator.
//
// This is true for any type which is one of the inlined primitives
// (pointers, interfaces, channels, maps) or one-field structs/arrays whose
// sole element is inlined. Note that this does *not* include all pointer-shaped
// structs, such as struct{struct{}; int}.
func InlinedAny[T any]() bool {
	var x T
	p := AnyData(any(x))

	// x's bit pattern is all zero, no matter what T is. If T is indirect,
	// the pointer extracted by AnyData will be nil. Otherwise, it will be a
	// stack pointer of some kind.
	return p == nil
}

// AssertInlinedAny is a helper for testing that T does not allocate when
// converted to an interface.
func AssertInlinedAny[T any](t testing.TB) {
	t.Helper()
	var z T
	assert.True(t, InlinedAny[T](), "expected %T to be pointer-shaped", z)
}
