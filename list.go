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

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// scalarList is a [protoreflect.List] implementation for non-bool scalar
// types.
type scalarList[E any] struct {
	unimplementedList
	raw []E
}

var _ protoreflect.List = scalarList[int32]{}

// Len implements protoreflect.List.
func (l scalarList[E]) Len() int {
	return len(l.raw)
}

// Get implements protoreflect.List.
func (l scalarList[E]) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(l.raw[n])
}

// scalarList is a [protoreflect.List] implementation for integer types that
// zig-zag decodes them on-demand.
type zigzagList[E integer] struct {
	unimplementedList
	raw []E
}

// Len implements protoreflect.List.
func (l zigzagList[E]) Len() int {
	return len(l.raw)
}

// Get implements protoreflect.List.
func (l zigzagList[E]) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(zigzag(l.raw[n]))
}

// byteScalarList is a [protoreflect.List] implementation for non-bool scalar
// types, where each value fits in a single byte.
type byteScalarList[E integer] struct {
	unimplementedList
	raw []byte
}

var _ protoreflect.List = scalarList[int32]{}

// Len implements protoreflect.List.
func (l byteScalarList[E]) Len() int {
	return len(l.raw)
}

// Get implements protoreflect.List.
func (l byteScalarList[E]) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(E(l.raw[n]))
}

// scalarList is a [protoreflect.List] implementation for integer types that
// zig-zag decodes them on-demand.
type byteZigZagList[E integer] struct {
	unimplementedList
	raw []byte
}

// Len implements protoreflect.List.
func (l byteZigZagList[E]) Len() int {
	return len(l.raw)
}

// Get implements protoreflect.List.
func (l byteZigZagList[E]) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(zigzag(E(l.raw[n])))
}

// scalarList is a [protoreflect.List] implementation for bool.
type boolList struct {
	unimplementedList
	raw []byte
}

// Len implements protoreflect.List.
func (l boolList) Len() int {
	return len(l.raw)
}

// Get implements protoreflect.List.
func (l boolList) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(l.raw[n] != 0)
}

// scalarList is a [protoreflect.List] implementation for string.
type stringList struct {
	unimplementedList
	raw    []zc
	shared *Context
}

// Len implements protoreflect.List.
func (l stringList) Len() int {
	return len(l.raw)
}

// Get implements protoreflect.List.
func (l stringList) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(l.raw[n].utf8(l.shared.src))
}

// scalarList is a [protoreflect.List] implementation for bytes.
type bytesList struct {
	unimplementedList
	raw    []zc
	shared *Context
}

// Len implements protoreflect.List.
func (l bytesList) Len() int {
	return len(l.raw)
}

// Get implements protoreflect.List.
func (l bytesList) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(l.raw[n].bytes(l.shared.src))
}

// messageList is a [protoreflect.List] implementation for message types.
type messageList struct {
	unimplementedList
	raw []*message
}

// Len implements protoreflect.List.
func (l messageList) Len() int {
	return len(l.raw)
}

// Get implements protoreflect.List.
func (l messageList) Get(n int) protoreflect.Value {
	return protoreflect.ValueOf(l.raw[n])
}

// inlineMessageList is a [protoreflect.List] implementation for message types.
type inlineMessageList struct {
	unimplementedList
	ty    Type
	raw   *message
	dummy []struct{}
}

// Len implements protoreflect.List.
func (l inlineMessageList) Len() int {
	return len(l.dummy)
}

// Get implements protoreflect.List.
func (l inlineMessageList) Get(n int) protoreflect.Value {
	_ = l.dummy[n] // Bounds check
	return protoreflect.ValueOf(unsafe2.ByteAdd(l.raw, n*int(l.ty.raw.size)))
}

// emptyList is an empty untyped list.
type emptyList struct {
	unimplementedList
}

// Len implements protoreflect.List.
func (emptyList) Len() int {
	return 0
}

// Get implements protoreflect.List.
func (emptyList) Get(n int) protoreflect.Value {
	_ = []byte{}[n] // Trigger a bounds check.s
	return protoreflect.Value{}
}

var _ protoreflect.List = unimplementedList{}

// unimplementedList is used to implement all of the methods of
// [protoreflect.List] by panicking.
type unimplementedList struct{}

var _ protoreflect.List = unimplementedList{}

// Append implements protoreflect.List.
func (unimplementedList) Append(protoreflect.Value) {
	panic(dbg.Unsupported())
}

// AppendMutable implements protoreflect.List.
func (unimplementedList) AppendMutable() protoreflect.Value {
	panic(dbg.Unsupported())
}

// Get implements protoreflect.List.
func (unimplementedList) Get(int) protoreflect.Value {
	panic(dbg.Unsupported())
}

// IsValid implements protoreflect.List.
func (unimplementedList) IsValid() bool {
	return true
}

// Len implements protoreflect.List.
func (unimplementedList) Len() int {
	panic(dbg.Unsupported())
}

// NewElement implements protoreflect.List.
func (unimplementedList) NewElement() protoreflect.Value {
	panic(dbg.Unsupported())
}

// Set implements protoreflect.List.
func (unimplementedList) Set(int, protoreflect.Value) {
	panic(dbg.Unsupported())
}

// Truncate implements protoreflect.List.
func (unimplementedList) Truncate(int) {
	panic(dbg.Unsupported())
}
