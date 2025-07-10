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

	"buf.build/go/hyperpb/internal/swiss"
	"buf.build/go/hyperpb/internal/tdp"
	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/empty"
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/zc"
)

// getMapIxI is a [getterThunk] for map<K, V> where K and V are both integer types.
func getMapIxI[K xprotoreflect.Int, V any](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[K, V]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOfMap(empty.Map{})
	}

	return protoreflect.ValueOfMap(mapIxI[K, V]{table: *v})
}

// mapIxI is a [protoreflect.Map] for map<K, V> where K and V are both integer types.
type mapIxI[K xprotoreflect.Int, V any] struct {
	empty.Map
	table *swiss.Table[K, V]
}

func (m mapIxI[K, V]) IsValid() bool                   { return m.table != nil }
func (m mapIxI[K, V]) Len() int                        { return m.table.Len() }
func (m mapIxI[K, V]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxI[K, V]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := xprotoreflect.GetInt[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.Value{}
	}

	return xprotoreflect.ValueOfScalar(*v)
}

func (m mapIxI[K, V]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(xprotoreflect.ValueOfScalar(k)), xprotoreflect.ValueOfScalar(v)) {
			return
		}
	}
}

// getMapIxS is a [getterThunk] for map<K, string> where K is an integer type.
func getMapIxS[K xprotoreflect.Int](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[K, zc.Range]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOfMap(empty.Map{})
	}

	return protoreflect.ValueOfMap(mapIxS[K]{src: m.Shared.Src, table: *v})
}

// mapIxS is a [protoreflect.Map] for map<K, string> where K is an integer type.
type mapIxS[K xprotoreflect.Int] struct {
	empty.Map
	src   *byte
	table *swiss.Table[K, zc.Range]
}

func (m mapIxS[K]) IsValid() bool                   { return m.table != nil }
func (m mapIxS[K]) Len() int                        { return m.table.Len() }
func (m mapIxS[K]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxS[K]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := xprotoreflect.GetInt[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.Value{}
	}

	return protoreflect.ValueOfString(v.String(m.src))
}

func (m mapIxS[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(xprotoreflect.ValueOfScalar(k)), protoreflect.ValueOfString(v.String(m.src))) {
			return
		}
	}
}

// getMapIxB is a [getterThunk] for map<K, bytes> where K is an integer type.
func getMapIxB[K xprotoreflect.Int](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[K, zc.Range]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOfMap(empty.Map{})
	}

	return protoreflect.ValueOfMap(mapIxB[K]{src: m.Shared.Src, table: *v})
}

// mapIxB is a [protoreflect.Map] for map<K, bytes> where K is an integer type.
type mapIxB[K xprotoreflect.Int] struct {
	empty.Map
	src   *byte
	table *swiss.Table[K, zc.Range]
}

func (m mapIxB[K]) IsValid() bool                   { return m.table != nil }
func (m mapIxB[K]) Len() int                        { return m.table.Len() }
func (m mapIxB[K]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxB[K]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := xprotoreflect.GetInt[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.Value{}
	}

	return protoreflect.ValueOfBytes(v.Bytes(m.src))
}

func (m mapIxB[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(xprotoreflect.ValueOfScalar(k)), protoreflect.ValueOfBytes(v.Bytes(m.src))) {
			return
		}
	}
}

// getMapIxM is a [getterThunk] for map<string, V> where V is an integer type.
func getMapIxM[K xprotoreflect.Int](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[K, *dynamic.Message]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOfMap(empty.Map{})
	}

	return protoreflect.ValueOfMap(mapIxM[K]{table: *v})
}

// mapIxM is a [protoreflect.Map] for map<string, V> where V is an integer type.
type mapIxM[K xprotoreflect.Int] struct {
	empty.Map
	table *swiss.Table[K, *dynamic.Message]
}

func (m mapIxM[K]) IsValid() bool                   { return m.table != nil }
func (m mapIxM[K]) Len() int                        { return m.table.Len() }
func (m mapIxM[K]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapIxM[K]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := xprotoreflect.GetInt[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.Value{}
	}

	return protoreflect.ValueOfMessage((*v).ProtoReflect())
}

func (m mapIxM[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(xprotoreflect.ValueOfScalar(k)), protoreflect.ValueOfMessage(v.ProtoReflect())) {
			return
		}
	}
}
