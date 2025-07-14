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

// StringToScalar is a map<string, V> field where V is a scalar type.
type StringToScalar[V any] struct {
	table swiss.Table[zc.Range, V]
}

// Len implements [Map].
func (m *StringToScalar[V]) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *StringToScalar[V]) Get(key string) (V, bool) {
	var z V
	if m == nil {
		return z, false
	}

	v := m.table.LookupFunc(xunsafe.StringToSlice[[]byte](key), m.extract)
	if v == nil {
		var z V
		return z, false
	}

	return *v, true
}

// Range implements [Map].
func (m *StringToScalar[V]) Range(yield func(string, V) bool) {
	if m == nil {
		return
	}
	for k, v := range m.table.All() {
		k := k.String(m.table.Scratch)
		if !yield(k, v) {
			return
		}
	}
}

// ProtoReflect implements [Map].
func (m *StringToScalar[V]) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectStringToScalar[V]](m)
}

func (m *StringToScalar[V]) extract(r zc.Range) []byte {
	return r.Bytes(m.table.Scratch)
}

func (*StringToScalar[_]) isMap() {} //nolint:unused

// StringToString is a map<string, string>.
type StringToString struct {
	_     [0]string // Prevent naughty casts.
	table swiss.Table[zc.Range, zc.Range]
}

// Len implements [Map].
func (m *StringToString) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *StringToString) Get(key string) (string, bool) {
	if m == nil {
		return "", false
	}

	v := m.table.LookupFunc(xunsafe.StringToSlice[[]byte](key), m.extract)
	if v == nil {
		return "", false
	}

	return v.String(m.table.Scratch), true
}

// Range implements [Map].
func (m *StringToString) Range(yield func(string, string) bool) {
	if m == nil {
		return
	}
	for k, v := range m.table.All() {
		k := k.String(m.table.Scratch)
		v := v.String(m.table.Scratch)
		if !yield(k, v) {
			return
		}
	}
}

// ProtoReflect implements [Map].
func (m *StringToString) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectStringToString](m)
}

func (*StringToString) isMap() {}

func (m *StringToString) extract(r zc.Range) []byte {
	return r.Bytes(m.table.Scratch)
}

// StringToBytes is a map<string, bytes>.
type StringToBytes struct {
	_     [0][]byte // Prevent naughty casts.
	table swiss.Table[zc.Range, zc.Range]
}

// Len implements [Map].
func (m *StringToBytes) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *StringToBytes) Get(key string) ([]byte, bool) {
	if m == nil {
		return nil, false
	}

	v := m.table.LookupFunc(xunsafe.StringToSlice[[]byte](key), m.extract)
	if v == nil {
		return nil, false
	}

	return v.Bytes(m.table.Scratch), true
}

// Range implements [Map].
func (m *StringToBytes) Range(yield func(string, []byte) bool) {
	if m == nil {
		return
	}
	for k, v := range m.table.All() {
		k := k.String(m.table.Scratch)
		v := v.Bytes(m.table.Scratch)
		if !yield(k, v) {
			return
		}
	}
}

// ProtoReflect implements [Map].
func (m *StringToBytes) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectStringToBytes](m)
}

func (*StringToBytes) isMap() {}

func (m *StringToBytes) extract(r zc.Range) []byte {
	return r.Bytes(m.table.Scratch)
}

// StringToMessage is a map<string, M> where M is a message type.
//
// M must be dynamic.Message or a type that wraps it.
type StringToMessage[M any] struct {
	table swiss.Table[zc.Range, *M]
}

// Len implements [Map].
func (m *StringToMessage[M]) Len() int {
	if m == nil {
		return 0
	}
	return m.table.Len()
}

// Get implements [Map].
func (m *StringToMessage[M]) Get(key string) (*M, bool) {
	if m == nil {
		return nil, false
	}

	v := m.table.LookupFunc(xunsafe.StringToSlice[[]byte](key), m.extract)
	if v == nil {
		return nil, false
	}

	return *v, true
}

// Range implements [Map].
func (m *StringToMessage[M]) Range(yield func(string, *M) bool) {
	if m == nil {
		return
	}
	for k, v := range m.table.All() {
		k := k.String(m.table.Scratch)
		if !yield(k, v) {
			return
		}
	}
}

// ProtoReflect implements [Map].
func (m *StringToMessage[_]) ProtoReflect() protoreflect.Map {
	return xunsafe.Cast[reflectStringToMessage](m)
}

func (*StringToMessage[_]) isMap() {} //nolint:unused

func (m *StringToMessage[M]) extract(r zc.Range) []byte {
	return r.Bytes(m.table.Scratch)
}
