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

package maps

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/swiss"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/zc"
)

// BoolToScalar is a map<bool, V> field where V is a scalar type.
type BoolToScalar[V any] struct {
	table swiss.Table[byte, V]
}

// Len implements [Map].
func (m *BoolToScalar[V]) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *BoolToScalar[V]) Get(key bool) (V, bool) {
	var z V
	if m == nil {
		return z, false
	}

	v := m.table.Lookup(b8(key))
	if v == nil {
		return z, false
	}

	return *v, true
}

// Range implements [Map].
func (m *BoolToScalar[V]) Range(yield func(bool, V) bool) {
	if m == nil {
		return
	}
	for k, v := range m.table.All() {
		if !yield(k != 0, v) {
			return
		}
	}
}

// ProtoReflect implements [Map].
func (m *BoolToScalar[V]) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectBoolToScalar[V]](m)
}

func (*BoolToScalar[_]) isMap() {} //nolint:unused

// BoolToString is a map<bool, string>.
type BoolToString struct {
	_     [0]string // Prevent naughty casts.
	table swiss.Table[byte, zc.Range]
}

// Len implements [Map].
func (m *BoolToString) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *BoolToString) Get(key bool) (string, bool) {
	if m == nil {
		return "", false
	}

	v := m.table.Lookup(b8(key))
	if v == nil {
		return "", false
	}

	return v.String(m.table.Scratch), true
}

// Range implements [Map].
func (m *BoolToString) Range(yield func(bool, string) bool) {
	if m == nil {
		return
	}
	for k, v := range m.table.All() {
		v := v.String(m.table.Scratch)
		if !yield(k != 0, v) {
			return
		}
	}
}

// ProtoReflect implements [Map].
func (m *BoolToString) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectBoolToString](m)
}

func (*BoolToString) isMap() {}

// BoolToBytes is a map<bool, bytes>.
type BoolToBytes struct {
	_     [0][]byte // Prevent naughty casts.
	table swiss.Table[byte, zc.Range]
}

// Len implements [Map].
func (m *BoolToBytes) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *BoolToBytes) Get(key bool) ([]byte, bool) {
	if m == nil {
		return nil, false
	}

	v := m.table.Lookup(b8(key))
	if v == nil {
		return nil, false
	}

	return v.Bytes(m.table.Scratch), true
}

// Range implements [Map].
func (m *BoolToBytes) Range(yield func(bool, []byte) bool) {
	if m == nil {
		return
	}
	for k, v := range m.table.All() {
		v := v.Bytes(m.table.Scratch)
		if !yield(k != 0, v) {
			return
		}
	}
}

// ProtoReflect implements [Map].
func (m *BoolToBytes) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectBoolToBytes](m)
}

func (*BoolToBytes) isMap() {}

// BoolToMessage is a map<bool, M> where M is a message type.
//
// M must be dynamic.Message or a type that wraps it.
type BoolToMessage[M any] struct {
	table swiss.Table[byte, *M]
}

// Len implements [Map].
func (m *BoolToMessage[M]) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *BoolToMessage[M]) Get(key bool) (*M, bool) {
	if m == nil {
		return nil, false
	}

	v := m.table.Lookup(b8(key))
	if v == nil {
		return nil, false
	}

	return *v, true
}

// Range implements [Map].
func (m *BoolToMessage[M]) Range(yield func(bool, *M) bool) {
	if m == nil {
		return
	}
	for k, v := range m.table.All() {
		if !yield(k != 0, v) {
			return
		}
	}
}

// ProtoReflect implements [Map].
func (m *BoolToMessage[_]) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectBoolToMessage](m)
}

func (*BoolToMessage[_]) isMap() {} //nolint:unused

func b8(b bool) byte {
	if b {
		return 1
	}
	return 0
}
