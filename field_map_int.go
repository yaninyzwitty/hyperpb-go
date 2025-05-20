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

package fastpb

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/swiss"
)

// getMapIxI is a [getterThunk] for map<K, V> where K and V are both integer types.
func getMapIxI[K integer, V any](m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[K, V]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(mapIxI[K, V]{table: *v})
}

// mapIxI is a [protoreflect.Map] for map<K, V> where K and V are both integer types.
type mapIxI[K integer, V any] struct {
	unimplementedMap
	table *swiss.Table[K, V]
}

func (m mapIxI[K, V]) Len() int                        { return m.table.Len() }
func (m mapIxI[K, V]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxI[K, V]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := reflectValueScalar[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(*v)
}

func (m mapIxI[K, V]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v)) {
			return
		}
	}
}

// getMapIxS is a [getterThunk] for map<K, string> where K is an integer type.
func getMapIxS[K integer](m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[K, zc]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(mapIxS[K]{src: m.context.src, table: *v})
}

// mapIxS is a [protoreflect.Map] for map<K, string> where K is an integer type.
type mapIxS[K integer] struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[K, zc]
}

func (m mapIxS[K]) Len() int                        { return m.table.Len() }
func (m mapIxS[K]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxS[K]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := reflectValueScalar[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.utf8(m.src))
}

func (m mapIxS[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v.utf8(m.src))) {
			return
		}
	}
}

// getMapIxB is a [getterThunk] for map<K, bytes> where K is an integer type.
func getMapIxB[K integer](m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[K, zc]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(mapIxB[K]{src: m.context.src, table: *v})
}

// mapIxB is a [protoreflect.Map] for map<K, bytes> where K is an integer type.
type mapIxB[K integer] struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[K, zc]
}

func (m mapIxB[K]) Len() int                        { return m.table.Len() }
func (m mapIxB[K]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxB[K]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := reflectValueScalar[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.bytes(m.src))
}

func (m mapIxB[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v.bytes(m.src))) {
			return
		}
	}
}
