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
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/zc"
)

// Int is any integer type that can be a key to a map.
type Int = xprotoreflect.Int

// IntToScalar is a map<K, V> field where K and V are both scalar types.
type IntToScalar[K Int, V any] struct {
	table swiss.Table[K, V]
}

// Len implements [Map].
func (m *IntToScalar[K, V]) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *IntToScalar[K, V]) Get(key K) (V, bool) {
	var z V
	if m == nil {
		return z, false
	}

	v := m.table.Lookup(key)
	if v == nil {
		var z V
		return z, false
	}

	return *v, true
}

// Range implements [Map].
func (m *IntToScalar[K, V]) Range(yield func(K, V) bool) {
	if m == nil {
		return
	}
	m.table.All()(yield)
}

// ProtoReflect implements [Map].
func (m *IntToScalar[K, V]) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectIntToScalar[K, V]](m)
}

func (*IntToScalar[_, _]) isMap() {} //nolint:unused

// IntToString is a map<K, string> where K is an integer type.
type IntToString[K Int] struct {
	_     [0]string // Prevent naughty casts.
	table swiss.Table[K, zc.Range]
}

// Len implements [Map].
func (m *IntToString[K]) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *IntToString[K]) Get(key K) (string, bool) {
	if m == nil {
		return "", false
	}

	v := m.table.Lookup(key)
	if v == nil {
		return "", false
	}

	return v.String(m.table.Scratch), true
}

// Range implements [Map].
func (m *IntToString[K]) Range(yield func(K, string) bool) {
	if m == nil {
		return
	}
	for k, v := range m.table.All() {
		v := v.String(m.table.Scratch)
		if !yield(k, v) {
			return
		}
	}
}

// ProtoReflect implements [Map].
func (m *IntToString[K]) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectIntToString[K]](m)
}

func (*IntToString[_]) isMap() {} //nolint:unused

// IntToBytes is a map<K, string> where K is an integer type.
type IntToBytes[K Int] struct {
	_     [0][]byte // Prevent naughty casts.
	table swiss.Table[K, zc.Range]
}

// Len implements [Map].
func (m *IntToBytes[K]) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *IntToBytes[K]) Get(key K) ([]byte, bool) {
	if m == nil {
		return nil, false
	}

	v := m.table.Lookup(key)
	if v == nil {
		return nil, false
	}

	return v.Bytes(m.table.Scratch), true
}

// Range implements [Map].
func (m *IntToBytes[K]) Range(yield func(K, []byte) bool) {
	if m == nil {
		return
	}
	for k, v := range m.table.All() {
		v := v.Bytes(m.table.Scratch)
		if !yield(k, v) {
			return
		}
	}
}

// ProtoReflect implements [Map].
func (m *IntToBytes[K]) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectIntToBytes[K]](m)
}

func (*IntToBytes[_]) isMap() {} //nolint:unused

// IntToMessage is a map<K, M> where K is an integer type and M is a message type.
//
// M must be dynamic.Message or a type that wraps it.
type IntToMessage[K xprotoreflect.Int, M any] struct {
	table swiss.Table[K, *M]
}

// Len implements [Map].
func (m *IntToMessage[K, M]) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *IntToMessage[K, M]) Get(key K) (*M, bool) {
	if m == nil {
		return nil, false
	}

	v := m.table.Lookup(key)
	if v == nil {
		return nil, false
	}

	return *v, true
}

// Range implements [Map].
func (m *IntToMessage[K, M]) Range(yield func(K, *M) bool) {
	if m == nil {
		return
	}
	m.table.All()(yield)
}

// ProtoReflect implements [Map].
func (m *IntToMessage[K, _]) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectIntToMessage[K]](m)
}

func (*IntToMessage[_, _]) isMap() {} //nolint:unused
