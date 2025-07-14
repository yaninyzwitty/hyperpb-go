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

// Package maps contains shared layouts for maps field implementations,
// for sharing between the tdp packages and the gencode packages.
package maps

import (
	"google.golang.org/protobuf/reflect/protoreflect"

	"buf.build/go/hyperpb/internal/tdp/dynamic"
)

// Map is an interface that all map types in this package conform to.
type Map[K, V any] interface {
	// Len returns the number of entries in the map.
	Len() int

	// Get looks up an entry in the map.
	Get(K) (V, bool)

	// Range is an iterator over the entries in the map; entries are returned
	// in unspecified order.
	Range(yield func(K, V) bool)

	// ProtoReflect returns a reflection view for this value.
	ProtoReflect() protoreflect.Map

	isMap()
}

var (
	_ Map[int32, int32]            = (*IntToScalar[int32, int32])(nil)
	_ Map[int32, string]           = (*IntToString[int32])(nil)
	_ Map[int32, []byte]           = (*IntToBytes[int32])(nil)
	_ Map[int32, *dynamic.Message] = (*IntToMessage[int32, dynamic.Message])(nil)

	_ Map[string, int32]            = (*StringToScalar[int32])(nil)
	_ Map[string, string]           = (*StringToString)(nil)
	_ Map[string, []byte]           = (*StringToBytes)(nil)
	_ Map[string, *dynamic.Message] = (*StringToMessage[dynamic.Message])(nil)

	_ Map[bool, int32]            = (*BoolToScalar[int32])(nil)
	_ Map[bool, string]           = (*BoolToString)(nil)
	_ Map[bool, []byte]           = (*BoolToBytes)(nil)
	_ Map[bool, *dynamic.Message] = (*BoolToMessage[dynamic.Message])(nil)
)
