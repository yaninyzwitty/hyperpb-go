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

	"buf.build/go/hyperpb/internal/tdp/dynamic"
	"buf.build/go/hyperpb/internal/tdp/empty"
	"buf.build/go/hyperpb/internal/xprotoreflect"
	"buf.build/go/hyperpb/internal/xunsafe"
)

// raw unwraps a reflection object in such a way that, if r is nil, the return
// value is also nil; field access would panic in this case.
func raw[R ~struct {
	empty.Map
	_ M
}, M any](r *R) *M {
	return xunsafe.Cast[M](r)
}

// reflectIntToScalar wraps an IntToScalar so that it implements protoreflect.Map.
type reflectIntToScalar[K Int, V any] struct {
	empty.Map
	_ IntToScalar[K, V]
}

// IsValid implements [protoreflect.Map].
func (r *reflectIntToScalar[_, _]) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectIntToScalar[_, _]) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectIntToScalar[_, _]) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectIntToScalar[K, _]) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(xprotoreflect.GetInt[K](k.Value()))
	if !ok {
		return protoreflect.Value{}
	}
	return xprotoreflect.ValueOfScalar(v)
}

func (r *reflectIntToScalar[K, _]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(xprotoreflect.ValueOfScalar(k)), xprotoreflect.ValueOfScalar(v)) {
			return
		}
	}
}

// reflectIntToString wraps an IntToScalar so that it implements protoreflect.Map.
type reflectIntToString[K Int] struct {
	empty.Map
	_ IntToString[K]
}

// IsValid implements [protoreflect.Map].
func (r *reflectIntToString[_]) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectIntToString[_]) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectIntToString[_]) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectIntToString[K]) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(xprotoreflect.GetInt[K](k.Value()))
	if !ok {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfString(v)
}

func (r *reflectIntToString[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(xprotoreflect.ValueOfScalar(k)), protoreflect.ValueOfString(v)) {
			return
		}
	}
}

// reflectIntToBytes wraps an IntToBytes so that it implements protoreflect.Map.
type reflectIntToBytes[K Int] struct {
	empty.Map
	_ IntToBytes[K]
}

// IsValid implements [protoreflect.Map].
func (r *reflectIntToBytes[_]) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectIntToBytes[_]) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectIntToBytes[_]) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectIntToBytes[K]) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(xprotoreflect.GetInt[K](k.Value()))
	if !ok {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfBytes(v)
}

func (r *reflectIntToBytes[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(xprotoreflect.ValueOfScalar(k)), protoreflect.ValueOfBytes(v)) {
			return
		}
	}
}

// reflectIntToMessage wraps an IntToMessage so that it implements protoreflect.Map.
type reflectIntToMessage[K Int] struct {
	empty.Map
	_ IntToMessage[K, dynamic.Message]
}

// IsValid implements [protoreflect.Map].
func (r *reflectIntToMessage[_]) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectIntToMessage[_]) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectIntToMessage[_]) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectIntToMessage[K]) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(xprotoreflect.GetInt[K](k.Value()))
	if !ok {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfMessage(v.ProtoReflect())
}

func (r *reflectIntToMessage[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(xprotoreflect.ValueOfScalar(k)), protoreflect.ValueOfMessage(v.ProtoReflect())) {
			return
		}
	}
}

// reflectStringToScalar wraps an StringToScalar so that it implements protoreflect.Map.
type reflectStringToScalar[V any] struct {
	empty.Map
	_ StringToScalar[V]
}

// IsValid implements [protoreflect.Map].
func (r *reflectStringToScalar[_]) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectStringToScalar[_]) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectStringToScalar[_]) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectStringToScalar[_]) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(xprotoreflect.GetString(k.Value()))
	if !ok {
		return protoreflect.Value{}
	}
	return xprotoreflect.ValueOfScalar(v)
}

func (r *reflectStringToScalar[_]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(protoreflect.ValueOfString(k)), xprotoreflect.ValueOfScalar(v)) {
			return
		}
	}
}

// reflectStringToString wraps an StringToScalar so that it implements protoreflect.Map.
type reflectStringToString struct {
	empty.Map
	_ StringToString
}

// IsValid implements [protoreflect.Map].
func (r *reflectStringToString) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectStringToString) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectStringToString) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectStringToString) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(xprotoreflect.GetString(k.Value()))
	if !ok {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfString(v)
}

func (r *reflectStringToString) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(protoreflect.ValueOfString(k)), protoreflect.ValueOfString(v)) {
			return
		}
	}
}

// reflectStringToBytes wraps an StringToBytes so that it implements protoreflect.Map.
type reflectStringToBytes struct {
	empty.Map
	_ StringToBytes
}

// IsValid implements [protoreflect.Map].
func (r *reflectStringToBytes) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectStringToBytes) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectStringToBytes) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectStringToBytes) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(xprotoreflect.GetString(k.Value()))
	if !ok {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfBytes(v)
}

func (r *reflectStringToBytes) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(protoreflect.ValueOfString(k)), protoreflect.ValueOfBytes(v)) {
			return
		}
	}
}

// reflectStringToMessage wraps an StringToMessage so that it implements protoreflect.Map.
type reflectStringToMessage struct {
	empty.Map
	_ StringToMessage[dynamic.Message]
}

// IsValid implements [protoreflect.Map].
func (r *reflectStringToMessage) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectStringToMessage) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectStringToMessage) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectStringToMessage) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(xprotoreflect.GetString(k.Value()))
	if !ok {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfMessage(v.ProtoReflect())
}

func (r *reflectStringToMessage) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(protoreflect.ValueOfString(k)), protoreflect.ValueOfMessage(v.ProtoReflect())) {
			return
		}
	}
}

// reflectBoolToScalar wraps an BoolToScalar so that it implements protoreflect.Map.
type reflectBoolToScalar[V any] struct {
	empty.Map
	_ BoolToScalar[V]
}

// IsValid implements [protoreflect.Map].
func (r *reflectBoolToScalar[_]) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectBoolToScalar[_]) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectBoolToScalar[_]) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectBoolToScalar[_]) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(k.Value().Bool())
	if !ok {
		return protoreflect.Value{}
	}
	return xprotoreflect.ValueOfScalar(v)
}

func (r *reflectBoolToScalar[_]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(protoreflect.ValueOfBool(k)), xprotoreflect.ValueOfScalar(v)) {
			return
		}
	}
}

// reflectBoolToString wraps an BoolToScalar so that it implements protoreflect.Map.
type reflectBoolToString struct {
	empty.Map
	_ BoolToString
}

// IsValid implements [protoreflect.Map].
func (r *reflectBoolToString) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectBoolToString) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectBoolToString) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectBoolToString) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(k.Value().Bool())
	if !ok {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfString(v)
}

func (r *reflectBoolToString) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(protoreflect.ValueOfBool(k)), protoreflect.ValueOfString(v)) {
			return
		}
	}
}

// reflectBoolToBytes wraps an BoolToBytes so that it implements protoreflect.Map.
type reflectBoolToBytes struct {
	empty.Map
	_ BoolToBytes
}

// IsValid implements [protoreflect.Map].
func (r *reflectBoolToBytes) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectBoolToBytes) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectBoolToBytes) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectBoolToBytes) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(k.Value().Bool())
	if !ok {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfBytes(v)
}

func (r *reflectBoolToBytes) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(protoreflect.ValueOfBool(k)), protoreflect.ValueOfBytes(v)) {
			return
		}
	}
}

// reflectBoolToMessage wraps an BoolToMessage so that it implements protoreflect.Map.
type reflectBoolToMessage struct {
	empty.Map
	_ BoolToMessage[dynamic.Message]
}

// IsValid implements [protoreflect.Map].
func (r *reflectBoolToMessage) IsValid() bool { return r != nil }

// Len implements [protoreflect.Map].
func (r *reflectBoolToMessage) Len() int {
	return raw(r).Len()
}

// Has implements [protoreflect.Map].
func (r *reflectBoolToMessage) Has(k protoreflect.MapKey) bool {
	return r.Get(k).IsValid()
}

// Get implements [protoreflect.Map].
func (r *reflectBoolToMessage) Get(k protoreflect.MapKey) protoreflect.Value {
	v, ok := raw(r).Get(k.Value().Bool())
	if !ok {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfMessage(v.ProtoReflect())
}

func (r *reflectBoolToMessage) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range raw(r).Range {
		if !yield(protoreflect.MapKey(protoreflect.ValueOfBool(k)), protoreflect.ValueOfMessage(v.ProtoReflect())) {
			return
		}
	}
}
