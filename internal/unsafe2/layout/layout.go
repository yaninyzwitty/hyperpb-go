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

// Package layout includes helpers for working with type layouts.
//
// It is separate from unsafe2, because nothing in this package is actually
// unsafe.

package layout

import "unsafe"

// Size returns T's size in bytes.
func Size[T any]() int {
	var z T
	return int(unsafe.Sizeof(z))
}

// Size returns T's size in bits.
func Bits[T any]() int {
	return Size[T]() * 8
}

// Size returns T's alignment in bytes.
func Align[T any]() int {
	var z T
	return int(unsafe.Alignof(z))
}

// Layout is the layout of some type.
type Layout struct {
	Size, Align int
}

// Of returns the size and alignment of a given type.
func Of[T any]() Layout {
	return Layout{Size[T](), Align[T]()}
}

// Max returns a layout whose size and alignment are both as large as the
// largest among l and that.
func (l Layout) Max(that Layout) Layout {
	return Layout{max(l.Size, that.Size), max(l.Align, that.Align)}
}
