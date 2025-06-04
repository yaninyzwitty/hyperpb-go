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

// map<bool, V> is implemented as a uint8-keyed map. They could be implemented
// as a pair of optional fields, but map<bool> is not common and so the
// maintenance cost is hard to justify.
//
// This was previously the case; if we ever want to bring back that
// optimization, this file's history contains it.

func b8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// getMap2xI is a [getterThunk] for map<bool, V> where V is an integer type.
func getMap2xI[V any](m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[uint8, V]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(map2xI[V]{table: *v})
}

// map2xI is a [protoreflect.Map] for map<bool, V> where V is an integer type.
type map2xI[V any] struct {
	unimplementedMap
	table *swiss.Table[uint8, V]
}

func (m map2xI[V]) Len() int                        { return m.table.Len() }
func (m map2xI[V]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m map2xI[V]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	v := m.table.Lookup(b8(k))
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(*v)
}

func (m map2xI[V]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k != 0)), protoreflect.ValueOf(v)) {
			return
		}
	}
}

// getMap2xS is a [getterThunk] for map<bool, string>.
func getMap2xS(m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[uint8, zc]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(map2xS{src: m.context.src, table: *v})
}

// map2xS is a [protoreflect.Map] for map<bool, string>.
type map2xS struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[uint8, zc]
}

func (m map2xS) Len() int                        { return m.table.Len() }
func (m map2xS) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m map2xS) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	v := m.table.Lookup(b8(k))
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.utf8(m.src))
}

func (m map2xS) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k != 0)), protoreflect.ValueOf(v.utf8(m.src))) {
			return
		}
	}
}

// getMap2xB is a [getterThunk] for map<bool, bytes>.
func getMap2xB(m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[uint8, zc]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(map2xB{src: m.context.src, table: *v})
}

// map2xB is a [protoreflect.Map] for map<bool, bytes>.
type map2xB struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[uint8, zc]
}

func (m map2xB) Len() int                        { return m.table.Len() }
func (m map2xB) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m map2xB) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	v := m.table.Lookup(b8(k))
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.bytes(m.src))
}

func (m map2xB) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k != 0)), protoreflect.ValueOf(v.bytes(m.src))) {
			return
		}
	}
}

// getMap2x2 is a [getterThunk] for map<bool, bytes>.
func getMap2x2(m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[uint8, uint8]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(map2x2{table: *v})
}

// map2xB is a [protoreflect.Map] for map<bool, bytes> where K is an integer type.
type map2x2 struct {
	unimplementedMap
	table *swiss.Table[uint8, uint8]
}

func (m map2x2) Len() int                        { return m.table.Len() }
func (m map2x2) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m map2x2) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	v := m.table.Lookup(b8(k))
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(*v != 0)
}

func (m map2x2) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k != 0)), protoreflect.ValueOf(v != 0)) {
			return
		}
	}
}

// getMap2xM is a [getterThunk] for map<bool, V> where V is a message type.
func getMap2xM(m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[uint8, *message]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(map2xM{table: *v})
}

// map2xM is a [protoreflect.Map] for map<bool, V> where V is a message type.
type map2xM struct {
	unimplementedMap
	table *swiss.Table[uint8, *message]
}

func (m map2xM) Len() int                        { return m.table.Len() }
func (m map2xM) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m map2xM) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	v := m.table.Lookup(b8(k))
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(*v)
}

func (m map2xM) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k != 0)), protoreflect.ValueOf(v)) {
			return
		}
	}
}
