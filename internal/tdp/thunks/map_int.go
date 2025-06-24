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

package thunks

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/hyperpb/internal/swiss"
	"github.com/bufbuild/hyperpb/internal/tdp"
	"github.com/bufbuild/hyperpb/internal/tdp/dynamic"
	"github.com/bufbuild/hyperpb/internal/tdp/empty"
	"github.com/bufbuild/hyperpb/internal/unsafe2/protoreflect2"
	"github.com/bufbuild/hyperpb/internal/zc"
)

// getMapIxI is a [getterThunk] for map<K, V> where K and V are both integer types.
func getMapIxI[K protoreflect2.Int, V any](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[K, V]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(empty.Map{})
	}

	return protoreflect.ValueOf(mapIxI[K, V]{table: *v})
}

// mapIxI is a [protoreflect.Map] for map<K, V> where K and V are both integer types.
type mapIxI[K protoreflect2.Int, V any] struct {
	empty.Map
	table *swiss.Table[K, V]
}

func (m mapIxI[K, V]) IsValid() bool                   { return m.table != nil }
func (m mapIxI[K, V]) Len() int                        { return m.table.Len() }
func (m mapIxI[K, V]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxI[K, V]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := protoreflect2.GetInt[K](mk.Value())
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
func getMapIxS[K protoreflect2.Int](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[K, zc.Range]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(empty.Map{})
	}

	return protoreflect.ValueOf(mapIxS[K]{src: m.Shared.Src, table: *v})
}

// mapIxS is a [protoreflect.Map] for map<K, string> where K is an integer type.
type mapIxS[K protoreflect2.Int] struct {
	empty.Map
	src   *byte
	table *swiss.Table[K, zc.Range]
}

func (m mapIxS[K]) IsValid() bool                   { return m.table != nil }
func (m mapIxS[K]) Len() int                        { return m.table.Len() }
func (m mapIxS[K]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxS[K]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := protoreflect2.GetInt[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.String(m.src))
}

func (m mapIxS[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v.String(m.src))) {
			return
		}
	}
}

// getMapIxB is a [getterThunk] for map<K, bytes> where K is an integer type.
func getMapIxB[K protoreflect2.Int](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[K, zc.Range]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(empty.Map{})
	}

	return protoreflect.ValueOf(mapIxB[K]{src: m.Shared.Src, table: *v})
}

// mapIxB is a [protoreflect.Map] for map<K, bytes> where K is an integer type.
type mapIxB[K protoreflect2.Int] struct {
	empty.Map
	src   *byte
	table *swiss.Table[K, zc.Range]
}

func (m mapIxB[K]) IsValid() bool                   { return m.table != nil }
func (m mapIxB[K]) Len() int                        { return m.table.Len() }
func (m mapIxB[K]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxB[K]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := protoreflect2.GetInt[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.Bytes(m.src))
}

func (m mapIxB[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v.Bytes(m.src))) {
			return
		}
	}
}

// getMapIxM is a [getterThunk] for map<string, V> where V is an integer type.
func getMapIxM[K protoreflect2.Int](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[K, *dynamic.Message]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(empty.Map{})
	}

	return protoreflect.ValueOf(mapIxM[K]{table: *v})
}

// mapIxM is a [protoreflect.Map] for map<string, V> where V is an integer type.
type mapIxM[K protoreflect2.Int] struct {
	empty.Map
	table *swiss.Table[K, *dynamic.Message]
}

func (m mapIxM[K]) IsValid() bool                   { return m.table != nil }
func (m mapIxM[K]) Len() int                        { return m.table.Len() }
func (m mapIxM[K]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxM[K]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := protoreflect2.GetInt[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(WrapMessage(*v))
}

func (m mapIxM[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(WrapMessage(v))) {
			return
		}
	}
}
