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
	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/zc"
)

// getMapSxI is a [getterThunk] for map<string, V> where V is an integer type.
func getMapSxI[V any](m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[zc.Range, V]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(mapSxI[V]{src: m.context.src, table: *v})
}

// mapSxI is a [protoreflect.Map] for map<string, V> where V is an integer type.
type mapSxI[V any] struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[zc.Range, V]
}

func (m mapSxI[V]) extract() func(zc.Range) []byte {
	return func(r zc.Range) []byte { return r.Bytes(m.src) }
}

func (m mapSxI[V]) Len() int                        { return m.table.Len() }
func (m mapSxI[V]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapSxI[V]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(unsafe2.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(*v)
}

func (m mapSxI[V]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.String(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v)) {
			return
		}
	}
}

// getMapSxS is a [protoreflect.Map] for map<string, string>.
func getMapSxS(m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[zc.Range, zc.Range]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(mapSxS{src: m.context.src, table: *v})
}

// mapSxS is a [protoreflect.Map] for map<string, string>.
type mapSxS struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[zc.Range, zc.Range]
}

func (m mapSxS) extract() func(zc.Range) []byte {
	return func(r zc.Range) []byte { return r.Bytes(m.src) }
}

func (m mapSxS) Len() int                        { return m.table.Len() }
func (m mapSxS) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapSxS) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(unsafe2.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.String(m.src))
}

func (m mapSxS) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.String(m.src)
		v := v.String(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v)) {
			return
		}
	}
}

// getMapSxS is a [protoreflect.Map] for map<string, bytes>.
func getMapSxB(m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[zc.Range, zc.Range]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(mapSxB{src: m.context.src, table: *v})
}

// stringScalarMap is a [protoreflect.Map] for map<string, bytes>.
type mapSxB struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[zc.Range, zc.Range]
}

func (m mapSxB) extract() func(zc.Range) []byte {
	return func(r zc.Range) []byte { return r.Bytes(m.src) }
}

func (m mapSxB) Len() int                        { return m.table.Len() }
func (m mapSxB) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapSxB) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(unsafe2.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.Bytes(m.src))
}

func (m mapSxB) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.String(m.src)
		v := v.Bytes(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v)) {
			return
		}
	}
}

// getMapSxM is a [getterThunk] for map<string, V> where V is a message type.
func getMapSxM(m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[zc.Range, *message]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(mapSxM{src: m.context.src, table: *v})
}

// mapSxM is a [protoreflect.Map] for map<string, V> where V is a message type.
type mapSxM struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[zc.Range, *message]
}

func (m mapSxM) extract() func(zc.Range) []byte {
	return func(r zc.Range) []byte { return r.Bytes(m.src) }
}

func (m mapSxM) Len() int                        { return m.table.Len() }
func (m mapSxM) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapSxM) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(unsafe2.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(*v)
}

func (m mapSxM) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.String(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v)) {
			return
		}
	}
}
