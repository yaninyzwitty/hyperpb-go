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
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/zc"
)

// getMapSxI is a [getterThunk] for map<string, V> where V is an integer type.
func getMapSxI[V any](m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[zc.Range, V]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOfMap(empty.Map{})
	}

	return protoreflect.ValueOfMap(mapSxI[V]{src: m.Shared.Src, table: *v})
}

// mapSxI is a [protoreflect.Map] for map<string, V> where V is an integer type.
type mapSxI[V any] struct {
	empty.Map
	src   *byte
	table *swiss.Table[zc.Range, V]
}

func (m mapSxI[V]) extract() func(zc.Range) []byte {
	return func(r zc.Range) []byte { return r.Bytes(m.src) }
}

func (m mapSxI[V]) IsValid() bool                   { return m.table != nil }
func (m mapSxI[V]) Len() int                        { return m.table.Len() }
func (m mapSxI[V]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapSxI[V]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(xunsafe.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.Value{}
	}

	return xprotoreflect.ValueOfScalar(*v)
}

func (m mapSxI[V]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.String(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOfString(k)), xprotoreflect.ValueOfScalar(v)) {
			return
		}
	}
}

// getMapSxS is a [protoreflect.Map] for map<string, string>.
func getMapSxS(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[zc.Range, zc.Range]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOfMap(empty.Map{})
	}

	return protoreflect.ValueOfMap(mapSxS{src: m.Shared.Src, table: *v})
}

// mapSxS is a [protoreflect.Map] for map<string, string>.
type mapSxS struct {
	empty.Map
	src   *byte
	table *swiss.Table[zc.Range, zc.Range]
}

func (m mapSxS) IsValid() bool                   { return m.table != nil }
func (m mapSxS) Len() int                        { return m.table.Len() }
func (m mapSxS) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapSxS) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(xunsafe.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.Value{}
	}

	return protoreflect.ValueOfString(v.String(m.src))
}

func (m mapSxS) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.String(m.src)
		v := v.String(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOfString(k)), protoreflect.ValueOfString(v)) {
			return
		}
	}
}

func (m mapSxS) extract() func(zc.Range) []byte {
	return func(r zc.Range) []byte { return r.Bytes(m.src) }
}

// getMapSxS is a [protoreflect.Map] for map<string, bytes>.
func getMapSxB(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[zc.Range, zc.Range]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOfMap(empty.Map{})
	}

	return protoreflect.ValueOfMap(mapSxB{src: m.Shared.Src, table: *v})
}

// stringScalarMap is a [protoreflect.Map] for map<string, bytes>.
type mapSxB struct {
	empty.Map
	src   *byte
	table *swiss.Table[zc.Range, zc.Range]
}

func (m mapSxB) IsValid() bool                   { return m.table != nil }
func (m mapSxB) Len() int                        { return m.table.Len() }
func (m mapSxB) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapSxB) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(xunsafe.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.Value{}
	}

	return protoreflect.ValueOfBytes(v.Bytes(m.src))
}

func (m mapSxB) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.String(m.src)
		v := v.Bytes(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOfString(k)), protoreflect.ValueOfBytes(v)) {
			return
		}
	}
}

func (m mapSxB) extract() func(zc.Range) []byte {
	return func(r zc.Range) []byte { return r.Bytes(m.src) }
}

// getMapSxM is a [getterThunk] for map<string, V> where V is a message type.
func getMapSxM(m *dynamic.Message, _ *tdp.Type, getter *tdp.Accessor) protoreflect.Value {
	v := dynamic.GetField[*swiss.Table[zc.Range, *dynamic.Message]](m, getter.Offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOfMap(empty.Map{})
	}

	return protoreflect.ValueOfMap(mapSxM{src: m.Shared.Src, table: *v})
}

// mapSxM is a [protoreflect.Map] for map<string, V> where V is a message type.
type mapSxM struct {
	empty.Map
	src   *byte
	table *swiss.Table[zc.Range, *dynamic.Message]
}

func (m mapSxM) IsValid() bool                   { return m.table != nil }
func (m mapSxM) Len() int                        { return m.table.Len() }
func (m mapSxM) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m mapSxM) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(xunsafe.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.Value{}
	}

	return protoreflect.ValueOfMessage(wrapMessage(*v))
}

func (m mapSxM) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.String(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOfString(k)), protoreflect.ValueOfMessage(wrapMessage(v))) {
			return
		}
	}
}

func (m mapSxM) extract() func(zc.Range) []byte {
	return func(r zc.Range) []byte { return r.Bytes(m.src) }
}
