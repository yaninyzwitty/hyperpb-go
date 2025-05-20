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
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/swiss"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

//go:generate go run ./internal/stencil

// mapFields consists of archetypes for map fields. The first index is the key,
// the second is the value.
var mapFields = [19][19]archetype{
	protoreflect.Int32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[int32, int32], parseScalarMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[int32, uint32], parseScalarMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[int32, int32], parseScalarMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[int32, int64], parseScalarMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[int32, uint64], parseScalarMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[int32, int64], parseScalarMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[int32, uint32], parseScalarMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[int32, int32], parseScalarMapV32xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[int32, float32], parseScalarMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[int32, uint64], parseScalarMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[int32, int64], parseScalarMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[int32, float64], parseScalarMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[int32, bool], parseScalarMapV32xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[int32, protoreflect.EnumNumber], parseScalarMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[int32], parseScalarMapV32xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[int32], parseScalarMapV32xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},
	protoreflect.Int64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[int64, int32], parseScalarMapV64xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[int64, uint32], parseScalarMapV64xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[int64, int32], parseScalarMapV64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[int64, int64], parseScalarMapV64xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[int64, uint64], parseScalarMapV64xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[int64, int64], parseScalarMapV64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[int64, uint32], parseScalarMapV64xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[int64, int32], parseScalarMapV64xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[int64, float32], parseScalarMapV64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[int64, uint64], parseScalarMapV64xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[int64, int64], parseScalarMapV64xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[int64, float64], parseScalarMapV64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[int64, bool], parseScalarMapV64xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[int64, protoreflect.EnumNumber], parseScalarMapV64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[int64], parseScalarMapV64xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[int64], parseScalarMapV64xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},
	protoreflect.Uint32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[uint32, int32], parseScalarMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[uint32, uint32], parseScalarMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[uint32, int32], parseScalarMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[uint32, int64], parseScalarMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[uint32, uint64], parseScalarMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[uint32, int64], parseScalarMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[uint32, uint32], parseScalarMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[uint32, int32], parseScalarMapV32xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[uint32, float32], parseScalarMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[uint32, uint64], parseScalarMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[uint32, int64], parseScalarMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[uint32, float64], parseScalarMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[uint32, bool], parseScalarMapV32xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[uint32, protoreflect.EnumNumber], parseScalarMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[uint32], parseScalarMapV32xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[uint32], parseScalarMapV32xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},
	protoreflect.Uint64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[uint64, int32], parseScalarMapV64xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[uint64, uint32], parseScalarMapV64xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[uint64, int32], parseScalarMapV64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[uint64, int64], parseScalarMapV64xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[uint64, uint64], parseScalarMapV64xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[uint64, int64], parseScalarMapV64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[uint64, uint32], parseScalarMapV64xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[uint64, int32], parseScalarMapV64xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[uint64, float32], parseScalarMapV64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[uint64, uint64], parseScalarMapV64xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[uint64, int64], parseScalarMapV64xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[uint64, float64], parseScalarMapV64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[uint64, bool], parseScalarMapV64xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[uint64, protoreflect.EnumNumber], parseScalarMapV64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[uint64], parseScalarMapV64xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[uint64], parseScalarMapV64xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},
	protoreflect.Sint32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[int32, int32], parseScalarMapZ32xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[int32, uint32], parseScalarMapZ32xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[int32, int32], parseScalarMapZ32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[int32, int64], parseScalarMapZ32xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[int32, uint64], parseScalarMapZ32xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[int32, int64], parseScalarMapZ32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[int32, uint32], parseScalarMapZ32xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[int32, int32], parseScalarMapZ32xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[int32, float32], parseScalarMapZ32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[int32, uint64], parseScalarMapZ32xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[int32, int64], parseScalarMapZ32xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[int32, float64], parseScalarMapZ32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[int32, bool], parseScalarMapZ32xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[int32, protoreflect.EnumNumber], parseScalarMapZ32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[int32], parseScalarMapZ32xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[int32], parseScalarMapZ32xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},
	protoreflect.Sint64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[int64, int32], parseScalarMapZ64xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[int64, uint32], parseScalarMapZ64xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[int64, int32], parseScalarMapZ64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[int64, int64], parseScalarMapZ64xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[int64, uint64], parseScalarMapZ64xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[int64, int64], parseScalarMapZ64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[int64, uint32], parseScalarMapZ64xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[int64, int32], parseScalarMapZ64xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[int64, float32], parseScalarMapZ64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[int64, uint64], parseScalarMapZ64xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[int64, int64], parseScalarMapZ64xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[int64, float64], parseScalarMapZ64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[int64, bool], parseScalarMapZ64xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[int64, protoreflect.EnumNumber], parseScalarMapZ64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[int64], parseScalarMapZ64xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[int64], parseScalarMapZ64xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},

	protoreflect.Fixed32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[uint32, int32], parseScalarMapF32xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[uint32, uint32], parseScalarMapF32xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[uint32, int32], parseScalarMapF32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[uint32, int64], parseScalarMapF32xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[uint32, uint64], parseScalarMapF32xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[uint32, int64], parseScalarMapF32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[uint32, uint32], parseScalarMapF32xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[uint32, int32], parseScalarMapF32xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[uint32, float32], parseScalarMapF32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[uint32, uint64], parseScalarMapF32xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[uint32, int64], parseScalarMapF32xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[uint32, float64], parseScalarMapF32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[uint32, bool], parseScalarMapF32xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[uint32, protoreflect.EnumNumber], parseScalarMapF32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[uint32], parseScalarMapF32xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[uint32], parseScalarMapF32xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},
	protoreflect.Fixed64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[uint64, int32], parseScalarMapF64xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[uint64, uint32], parseScalarMapF64xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[uint64, int32], parseScalarMapF64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[uint64, int64], parseScalarMapF64xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[uint64, uint64], parseScalarMapF64xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[uint64, int64], parseScalarMapF64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[uint64, uint32], parseScalarMapF64xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[uint64, int32], parseScalarMapF64xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[uint64, float32], parseScalarMapF64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[uint64, uint64], parseScalarMapF64xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[uint64, int64], parseScalarMapF64xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[uint64, float64], parseScalarMapF64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[uint64, bool], parseScalarMapF64xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[uint64, protoreflect.EnumNumber], parseScalarMapF64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[uint64], parseScalarMapF64xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[uint64], parseScalarMapF64xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},
	protoreflect.Sfixed32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[int32, int32], parseScalarMapF32xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[int32, uint32], parseScalarMapF32xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[int32, int32], parseScalarMapF32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[int32, int64], parseScalarMapF32xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[int32, uint64], parseScalarMapF32xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[int32, int64], parseScalarMapF32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[int32, uint32], parseScalarMapF32xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[int32, int32], parseScalarMapF32xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[int32, float32], parseScalarMapF32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[int32, uint64], parseScalarMapF32xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[int32, int64], parseScalarMapF32xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[int32, float64], parseScalarMapF32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[int32, bool], parseScalarMapF32xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[int32, protoreflect.EnumNumber], parseScalarMapF32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[int32], parseScalarMapF32xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[int32], parseScalarMapF32xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},
	protoreflect.Sfixed64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[int64, int32], parseScalarMapF64xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[int64, uint32], parseScalarMapF64xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[int64, int32], parseScalarMapF64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[int64, int64], parseScalarMapF64xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[int64, uint64], parseScalarMapF64xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[int64, int64], parseScalarMapF64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[int64, uint32], parseScalarMapF64xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[int64, int32], parseScalarMapF64xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[int64, float32], parseScalarMapF64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[int64, uint64], parseScalarMapF64xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[int64, int64], parseScalarMapF64xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[int64, float64], parseScalarMapF64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[int64, bool], parseScalarMapF64xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[int64, protoreflect.EnumNumber], parseScalarMapF64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[int64], parseScalarMapF64xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[int64], parseScalarMapF64xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},

	// Bool maps are implements as two bits and a [2]V.
	protoreflect.BoolKind: {
		protoreflect.Int32Kind: archetype{
			size:    2 * uint32(unsafe2.Int32Size),
			align:   uint32(unsafe2.Int32Align),
			bits:    2,
			getter:  getBoolScalarMap[int32],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapV32}},
		},
		protoreflect.Int64Kind: archetype{
			size:    2 * uint32(unsafe2.Int64Size),
			align:   uint32(unsafe2.Int64Align),
			bits:    2,
			getter:  getBoolScalarMap[int64],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapV64}},
		},
		protoreflect.Uint32Kind: archetype{
			size:    2 * uint32(unsafe2.Int32Size),
			align:   uint32(unsafe2.Int32Align),
			bits:    2,
			getter:  getBoolScalarMap[uint32],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapV32}},
		},
		protoreflect.Uint64Kind: archetype{
			size:    2 * uint32(unsafe2.Int64Size),
			align:   uint32(unsafe2.Int64Align),
			bits:    2,
			getter:  getBoolScalarMap[uint64],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapV64}},
		},
		protoreflect.Sint32Kind: archetype{
			size:    2 * uint32(unsafe2.Int32Size),
			align:   uint32(unsafe2.Int32Align),
			bits:    2,
			getter:  getBoolScalarMap[int32],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapZ32}},
		},
		protoreflect.Sint64Kind: archetype{
			size:    2 * uint32(unsafe2.Int64Size),
			align:   uint32(unsafe2.Int64Align),
			bits:    2,
			getter:  getBoolScalarMap[int64],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapZ64}},
		},

		protoreflect.Fixed32Kind: archetype{
			size:    2 * uint32(unsafe2.Int32Size),
			align:   uint32(unsafe2.Int32Align),
			bits:    2,
			getter:  getBoolScalarMap[uint32],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF32}},
		},
		protoreflect.Fixed64Kind: archetype{
			size:    2 * uint32(unsafe2.Int64Size),
			align:   uint32(unsafe2.Int64Align),
			bits:    2,
			getter:  getBoolScalarMap[uint64],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF64}},
		},
		protoreflect.Sfixed32Kind: archetype{
			size:    2 * uint32(unsafe2.Int32Size),
			align:   uint32(unsafe2.Int32Align),
			bits:    2,
			getter:  getBoolScalarMap[int32],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF32}},
		},
		protoreflect.Sfixed64Kind: archetype{
			size:    2 * uint32(unsafe2.Int64Size),
			align:   uint32(unsafe2.Int64Align),
			bits:    2,
			getter:  getBoolScalarMap[int64],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF64}},
		},
		protoreflect.FloatKind: archetype{
			size:    2 * uint32(unsafe2.Int32Size),
			align:   uint32(unsafe2.Int32Align),
			bits:    2,
			getter:  getBoolScalarMap[float32],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF32}},
		},
		protoreflect.DoubleKind: archetype{
			size:    2 * uint32(unsafe2.Int64Size),
			align:   uint32(unsafe2.Int64Align),
			bits:    2,
			getter:  getBoolScalarMap[float64],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapF64}},
		},

		protoreflect.BoolKind: archetype{
			align:   1,
			bits:    4,
			getter:  getBoolBoolMap,
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolBoolMap}},
		},
		protoreflect.EnumKind: archetype{
			size:    2 * uint32(unsafe2.Int32Size),
			align:   uint32(unsafe2.Int32Align),
			bits:    2,
			getter:  getBoolScalarMap[protoreflect.EnumNumber],
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapV32}},
		},

		protoreflect.StringKind: archetype{
			size:    2 * uint32(zcSize),
			align:   uint32(zcAlign),
			bits:    2,
			getter:  getBoolStringMap,
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapS}},
		},
		protoreflect.BytesKind: archetype{
			size:    2 * uint32(zcSize),
			align:   uint32(zcAlign),
			bits:    2,
			getter:  getBoolBytesMap,
			parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parseBoolScalarMapB}},
		},

		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},

	protoreflect.EnumKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getScalarMap[protoreflect.EnumNumber, int32], parseScalarMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getScalarMap[protoreflect.EnumNumber, uint32], parseScalarMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getScalarMap[protoreflect.EnumNumber, int32], parseScalarMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getScalarMap[protoreflect.EnumNumber, int64], parseScalarMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getScalarMap[protoreflect.EnumNumber, uint64], parseScalarMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getScalarMap[protoreflect.EnumNumber, int64], parseScalarMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getScalarMap[protoreflect.EnumNumber, uint32], parseScalarMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getScalarMap[protoreflect.EnumNumber, int32], parseScalarMapV32xF32),
		protoreflect.FloatKind:    mapArch(getScalarMap[protoreflect.EnumNumber, float32], parseScalarMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getScalarMap[protoreflect.EnumNumber, uint64], parseScalarMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getScalarMap[protoreflect.EnumNumber, int64], parseScalarMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getScalarMap[protoreflect.EnumNumber, float64], parseScalarMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getScalarMap[protoreflect.EnumNumber, bool], parseScalarMapV32xV1),
		protoreflect.EnumKind: mapArch(getScalarMap[protoreflect.EnumNumber, protoreflect.EnumNumber], parseScalarMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getScalarStringMap[protoreflect.EnumNumber], parseScalarMapV32xS),
		protoreflect.BytesKind:  mapArch(getScalarBytesMap[protoreflect.EnumNumber], parseScalarMapV32xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},

	protoreflect.StringKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getStringScalarMap[int32], parseScalarMapSxV32),
		protoreflect.Uint32Kind: mapArch(getStringScalarMap[uint32], parseScalarMapSxV32),
		protoreflect.Sint32Kind: mapArch(getStringScalarMap[int32], parseScalarMapSxZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getStringScalarMap[int64], parseScalarMapSxV64),
		protoreflect.Uint64Kind: mapArch(getStringScalarMap[uint64], parseScalarMapSxV64),
		protoreflect.Sint64Kind: mapArch(getStringScalarMap[int64], parseScalarMapSxZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getStringScalarMap[uint32], parseScalarMapSxF32),
		protoreflect.Sfixed32Kind: mapArch(getStringScalarMap[int32], parseScalarMapSxF32),
		protoreflect.FloatKind:    mapArch(getStringScalarMap[float32], parseScalarMapSxF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getStringScalarMap[uint64], parseScalarMapSxF64),
		protoreflect.Sfixed64Kind: mapArch(getStringScalarMap[int64], parseScalarMapSxF64),
		protoreflect.DoubleKind:   mapArch(getStringScalarMap[float64], parseScalarMapSxF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getStringScalarMap[bool], parseScalarMapSxV1),
		protoreflect.EnumKind: mapArch(getStringScalarMap[protoreflect.EnumNumber], parseScalarMapSxV32),

		// String types.
		protoreflect.StringKind: mapArch(getStringStringMap, parseScalarMapSxS),
		protoreflect.BytesKind:  mapArch(getStringBytesMap, parseScalarMapSxB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},
}

func mapArch(getter getterThunk, parser parserThunk) archetype {
	return archetype{
		size: uint32(unsafe2.PointerSize), align: uint32(unsafe2.PointerAlign),
		getter:  getter,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parser}},
	}
}

type emptyMap struct {
	unimplementedMap
}

func (emptyMap) IsValid() bool                                                  { return false }
func (emptyMap) Len() int                                                       { return 0 }
func (emptyMap) Has(mk protoreflect.MapKey) bool                                { return false }
func (emptyMap) Get(mk protoreflect.MapKey) protoreflect.Value                  { return protoreflect.ValueOf(nil) }
func (emptyMap) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {}

type scalarMap[K integer, V any] struct {
	unimplementedMap
	table *swiss.Table[K, V]
}

func (m scalarMap[K, V]) Len() int                        { return m.table.Len() }
func (m scalarMap[K, V]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m scalarMap[K, V]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := reflectValueScalar[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(*v)
}

func (m scalarMap[K, V]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v)) {
			return
		}
	}
}

func getScalarMap[K integer, V any](m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[K, V]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(scalarMap[K, V]{table: *v})
}

type scalarStringMap[K integer] struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[K, zc]
}

func (m scalarStringMap[K]) Len() int                        { return m.table.Len() }
func (m scalarStringMap[K]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m scalarStringMap[K]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := reflectValueScalar[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.utf8(m.src))
}

func (m scalarStringMap[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v.utf8(m.src))) {
			return
		}
	}
}

func getScalarStringMap[K integer](m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[K, zc]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(scalarStringMap[K]{src: m.context.src, table: *v})
}

type scalarBytesMap[K integer] struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[K, zc]
}

func (m scalarBytesMap[K]) Len() int                        { return m.table.Len() }
func (m scalarBytesMap[K]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m scalarBytesMap[K]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := reflectValueScalar[K](mk.Value())
	v := m.table.Lookup(k)
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.bytes(m.src))
}

func (m scalarBytesMap[K]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v.bytes(m.src))) {
			return
		}
	}
}

func getScalarBytesMap[K integer](m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[K, zc]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(scalarBytesMap[K]{src: m.context.src, table: *v})
}

type stringScalarMap[V any] struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[zc, V]
}

func (m stringScalarMap[V]) extract() func(zc) []byte {
	return func(zc zc) []byte { return zc.bytes(m.src) }
}

func (m stringScalarMap[V]) Len() int                        { return m.table.Len() }
func (m stringScalarMap[V]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m stringScalarMap[V]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(unsafe2.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(*v)
}

func (m stringScalarMap[V]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.utf8(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v)) {
			return
		}
	}
}

func getStringScalarMap[V any](m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[zc, V]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(stringScalarMap[V]{src: m.context.src, table: *v})
}

type stringStringMap struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[zc, zc]
}

func (m stringStringMap) extract() func(zc) []byte {
	return func(zc zc) []byte { return zc.bytes(m.src) }
}

func (m stringStringMap) Len() int                        { return m.table.Len() }
func (m stringStringMap) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m stringStringMap) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(unsafe2.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.utf8(m.src))
}

func (m stringStringMap) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.utf8(m.src)
		v := v.utf8(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v)) {
			return
		}
	}
}

func getStringStringMap(m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[zc, zc]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(stringStringMap{src: m.context.src, table: *v})
}

type stringBytesMap struct {
	unimplementedMap
	src   *byte
	table *swiss.Table[zc, zc]
}

func (m stringBytesMap) extract() func(zc) []byte {
	return func(zc zc) []byte { return zc.bytes(m.src) }
}

func (m stringBytesMap) Len() int                        { return m.table.Len() }
func (m stringBytesMap) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m stringBytesMap) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.String()
	v := m.table.LookupFunc(unsafe2.StringToSlice[[]byte](k), m.extract())
	if v == nil {
		return protoreflect.ValueOf(nil)
	}

	return protoreflect.ValueOf(v.bytes(m.src))
}

func (m stringBytesMap) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	for k, v := range m.table.All() {
		k := k.utf8(m.src)
		v := v.bytes(m.src)
		if !yield(protoreflect.MapKey(protoreflect.ValueOf(k)), protoreflect.ValueOf(v)) {
			return
		}
	}
}

func getStringBytesMap(m *message, _ Type, getter getter) protoreflect.Value {
	v := getField[*swiss.Table[zc, zc]](m, getter.offset)
	if v == nil || *v == nil {
		return protoreflect.ValueOf(emptyMap{})
	}

	return protoreflect.ValueOf(stringBytesMap{src: m.context.src, table: *v})
}

type boolScalarMap[V any] struct {
	unimplementedMap
	m *message
	f fieldOffset
}

func (m boolScalarMap[V]) Len() int {
	var n int
	if m.m.getBit(m.f.bit) {
		n++
	}
	if m.m.getBit(m.f.bit + 1) {
		n++
	}
	return n
}
func (m boolScalarMap[V]) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m boolScalarMap[V]) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	var idx uint32
	if k {
		idx = 1
	}

	p := getField[V](m.m, m.f)
	if !m.m.getBit(m.f.bit+idx) || p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := unsafe2.Add(p, idx)
	return protoreflect.ValueOf(*v)
}

func (m boolScalarMap[V]) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	v := getField[V](m.m, m.f)
	if v == nil {
		return
	}
	if m.m.getBit(m.f.bit) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(false)),
			protoreflect.ValueOf(*v)) {
		return
	}

	v = unsafe2.Add(v, 1)
	if m.m.getBit(m.f.bit+1) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(true)),
			protoreflect.ValueOf(*v)) {
		return
	}
}

func getBoolScalarMap[V any](m *message, _ Type, getter getter) protoreflect.Value {
	return protoreflect.ValueOf(boolScalarMap[V]{m: m, f: getter.offset})
}

type boolStringMap struct {
	unimplementedMap
	m *message
	f fieldOffset
}

func (m boolStringMap) Len() int {
	var n int
	if m.m.getBit(m.f.bit) {
		n++
	}
	if m.m.getBit(m.f.bit + 1) {
		n++
	}
	return n
}
func (m boolStringMap) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m boolStringMap) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	var idx uint32
	if k {
		idx = 1
	}

	p := getField[zc](m.m, m.f)
	if !m.m.getBit(m.f.bit+idx) || p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := unsafe2.Add(p, idx)
	return protoreflect.ValueOf(v.utf8(m.m.context.src))
}

func (m boolStringMap) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	v := getField[zc](m.m, m.f)
	if v == nil {
		return
	}
	if m.m.getBit(m.f.bit) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(false)),
			protoreflect.ValueOf(v.utf8(m.m.context.src))) {
		return
	}

	v = unsafe2.Add(v, 1)
	if m.m.getBit(m.f.bit+1) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(true)),
			protoreflect.ValueOf(v.utf8(m.m.context.src))) {
		return
	}
}

func getBoolStringMap(m *message, _ Type, getter getter) protoreflect.Value {
	return protoreflect.ValueOf(boolStringMap{m: m, f: getter.offset})
}

type boolBytesMap struct {
	unimplementedMap
	m *message
	f fieldOffset
}

func (m boolBytesMap) Len() int {
	var n int
	if m.m.getBit(m.f.bit) {
		n++
	}
	if m.m.getBit(m.f.bit + 1) {
		n++
	}
	return n
}
func (m boolBytesMap) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m boolBytesMap) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	var idx uint32
	if k {
		idx = 1
	}

	p := getField[zc](m.m, m.f)
	if !m.m.getBit(m.f.bit+idx) || p == nil {
		return protoreflect.ValueOf(nil)
	}

	v := unsafe2.Add(p, idx)
	return protoreflect.ValueOf(v.bytes(m.m.context.src))
}

func (m boolBytesMap) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	v := getField[zc](m.m, m.f)
	if v == nil {
		return
	}
	if m.m.getBit(m.f.bit) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(false)),
			protoreflect.ValueOf(v.bytes(m.m.context.src))) {
		return
	}

	v = unsafe2.Add(v, 1)
	if m.m.getBit(m.f.bit+1) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(true)),
			protoreflect.ValueOf(v.bytes(m.m.context.src))) {
		return
	}
}

func getBoolBytesMap(m *message, _ Type, getter getter) protoreflect.Value {
	return protoreflect.ValueOf(boolBytesMap{m: m, f: getter.offset})
}

type boolBoolMap struct {
	unimplementedMap
	m *message
	f fieldOffset
}

func (m boolBoolMap) Len() int {
	var n int
	if m.m.getBit(m.f.bit) {
		n++
	}
	if m.m.getBit(m.f.bit + 1) {
		n++
	}
	return n
}
func (m boolBoolMap) Has(mk protoreflect.MapKey) bool { return m.Get(mk).IsValid() }
func (m boolBoolMap) Get(mk protoreflect.MapKey) protoreflect.Value {
	k := mk.Bool()
	var idx uint32
	if k {
		idx = 1
	}

	if !m.m.getBit(m.f.bit + idx) {
		return protoreflect.ValueOf(nil)
	}
	return protoreflect.ValueOf(m.m.getBit(m.f.bit + idx + 2))
}

func (m boolBoolMap) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {
	if m.m.getBit(m.f.bit) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(false)),
			protoreflect.ValueOfBool(m.m.getBit(m.f.bit+2))) {
		return
	}

	if m.m.getBit(m.f.bit+1) &&
		!yield(protoreflect.MapKey(protoreflect.ValueOfBool(true)),
			protoreflect.ValueOf(m.m.getBit(m.f.bit+3))) {
		return
	}
}

func getBoolBoolMap(m *message, _ Type, getter getter) protoreflect.Value {
	return protoreflect.ValueOf(boolBoolMap{m: m, f: getter.offset})
}

// mapItem is a type usable in any of the map parsers.
type mapItem[V any] interface {
	kind() protowire.Type
	parse(parser1, parser2) (parser1, parser2, V)
	extract(parser1, parser2) func(V) []byte
}

type (
	varintItem[T uint32 | uint64] struct{}
	zigzagItem[T uint32 | uint64] struct{}
	boolItem                      struct{}
	fixed32Item                   struct{}
	fixed64Item                   struct{}
	stringItem                    struct{}
	bytesItem                     struct{}
)

func (varintItem[_]) kind() protowire.Type { return protowire.VarintType }
func (zigzagItem[_]) kind() protowire.Type { return protowire.VarintType }
func (boolItem) kind() protowire.Type      { return protowire.VarintType }
func (fixed32Item) kind() protowire.Type   { return protowire.Fixed32Type }
func (fixed64Item) kind() protowire.Type   { return protowire.Fixed64Type }
func (stringItem) kind() protowire.Type    { return protowire.BytesType }
func (bytesItem) kind() protowire.Type     { return protowire.BytesType }

func (varintItem[T]) parse(p1 parser1, p2 parser2) (parser1, parser2, T) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	return p1, p2, T(n)
}

func (zigzagItem[T]) parse(p1 parser1, p2 parser2) (parser1, parser2, T) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	return p1, p2, zigzag64[T](n)
}

func (boolItem) parse(p1 parser1, p2 parser2) (parser1, parser2, uint8) {
	var n uint64
	p1, p2, n = p1.varint(p2)
	if n != 0 {
		n = 1
	}
	return p1, p2, uint8(n)
}

func (fixed32Item) parse(p1 parser1, p2 parser2) (parser1, parser2, uint32) {
	return p1.fixed32(p2)
}

func (fixed64Item) parse(p1 parser1, p2 parser2) (parser1, parser2, uint64) {
	return p1.fixed64(p2)
}

func (stringItem) parse(p1 parser1, p2 parser2) (parser1, parser2, uint64) {
	var zc zc
	p1, p2, zc = p1.utf8(p2)
	return p1, p2, uint64(zc)
}

func (bytesItem) parse(p1 parser1, p2 parser2) (parser1, parser2, uint64) {
	var zc zc
	p1, p2, zc = p1.bytes(p2)
	return p1, p2, uint64(zc)
}

func (varintItem[T]) extract(parser1, parser2) func(T) []byte    { return nil }
func (zigzagItem[T]) extract(parser1, parser2) func(T) []byte    { return nil }
func (fixed32Item) extract(parser1, parser2) func(uint32) []byte { return nil }
func (fixed64Item) extract(parser1, parser2) func(uint64) []byte { return nil }
func (stringItem) extract(p1 parser1, _ parser2) func(uint64) []byte {
	src := p1.c().src
	return func(u uint64) []byte {
		return zc(u).bytes(src)
	}
}

//nolint:unused // Required for interface conformance.
func (boolItem) extract(parser1, parser2) func(uint8) []byte { panic(dbg.Unsupported()) }

//nolint:unused // Required for interface conformance.
func (bytesItem) extract(parser1, parser2) func(uint64) []byte { panic(dbg.Unsupported()) }

//fastpb:stencil parseScalarMapV32xV32 parseScalarMap[varintItem[uint32], varintItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseScalarMapV32xV64 parseScalarMap[varintItem[uint32], varintItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapV32xZ32 parseScalarMap[varintItem[uint32], zigzagItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseScalarMapV32xZ64 parseScalarMap[varintItem[uint32], zigzagItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapV32xF32 parseScalarMap[varintItem[uint32], fixed32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseScalarMapV32xF64 parseScalarMap[varintItem[uint32], fixed64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapV32xV1  parseScalarMap[varintItem[uint32], boolItem, uint32, uint8] Init -> swiss.InitU32xU8 Insert -> swiss.InsertU32xU8
//fastpb:stencil parseScalarMapV32xS   parseScalarMap[varintItem[uint32], stringItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapV32xB   parseScalarMap[varintItem[uint32], bytesItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64

//fastpb:stencil parseScalarMapV64xV32 parseScalarMap[varintItem[uint64], varintItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapV64xV64 parseScalarMap[varintItem[uint64], varintItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapV64xZ32 parseScalarMap[varintItem[uint64], zigzagItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapV64xZ64 parseScalarMap[varintItem[uint64], zigzagItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapV64xF32 parseScalarMap[varintItem[uint64], fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapV64xF64 parseScalarMap[varintItem[uint64], fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapV64xV1  parseScalarMap[varintItem[uint64], boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//fastpb:stencil parseScalarMapV64xS   parseScalarMap[varintItem[uint64], stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapV64xB   parseScalarMap[varintItem[uint64], bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//fastpb:stencil parseScalarMapZ32xV32 parseScalarMap[zigzagItem[uint32], varintItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseScalarMapZ32xV64 parseScalarMap[zigzagItem[uint32], varintItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapZ32xZ32 parseScalarMap[zigzagItem[uint32], zigzagItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseScalarMapZ32xZ64 parseScalarMap[zigzagItem[uint32], zigzagItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapZ32xF32 parseScalarMap[zigzagItem[uint32], fixed32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseScalarMapZ32xF64 parseScalarMap[zigzagItem[uint32], fixed64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapZ32xV1  parseScalarMap[zigzagItem[uint32], boolItem, uint32, uint8] Init -> swiss.InitU32xU8 Insert -> swiss.InsertU32xU8
//fastpb:stencil parseScalarMapZ32xS   parseScalarMap[zigzagItem[uint32], stringItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapZ32xB   parseScalarMap[zigzagItem[uint32], bytesItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64

//fastpb:stencil parseScalarMapZ64xV32 parseScalarMap[zigzagItem[uint64], varintItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapZ64xV64 parseScalarMap[zigzagItem[uint64], varintItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapZ64xZ32 parseScalarMap[zigzagItem[uint64], zigzagItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapZ64xZ64 parseScalarMap[zigzagItem[uint64], zigzagItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapZ64xF32 parseScalarMap[zigzagItem[uint64], fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapZ64xF64 parseScalarMap[zigzagItem[uint64], fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapZ64xV1  parseScalarMap[zigzagItem[uint64], boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//fastpb:stencil parseScalarMapZ64xS   parseScalarMap[zigzagItem[uint64], stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapZ64xB   parseScalarMap[zigzagItem[uint64], bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//fastpb:stencil parseScalarMapF32xV32 parseScalarMap[fixed32Item, varintItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseScalarMapF32xV64 parseScalarMap[fixed32Item, varintItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapF32xZ32 parseScalarMap[fixed32Item, zigzagItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseScalarMapF32xZ64 parseScalarMap[fixed32Item, zigzagItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapF32xF32 parseScalarMap[fixed32Item, fixed32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseScalarMapF32xF64 parseScalarMap[fixed32Item, fixed64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapF32xV1  parseScalarMap[fixed32Item, boolItem, uint32, uint8] Init -> swiss.InitU32xU8 Insert -> swiss.InsertU32xU8
//fastpb:stencil parseScalarMapF32xS   parseScalarMap[fixed32Item, stringItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseScalarMapF32xB   parseScalarMap[fixed32Item, bytesItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64

//fastpb:stencil parseScalarMapF64xV32 parseScalarMap[fixed64Item, varintItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapF64xV64 parseScalarMap[fixed64Item, varintItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapF64xZ32 parseScalarMap[fixed64Item, zigzagItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapF64xZ64 parseScalarMap[fixed64Item, zigzagItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapF64xF32 parseScalarMap[fixed64Item, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapF64xF64 parseScalarMap[fixed64Item, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapF64xV1  parseScalarMap[fixed64Item, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//fastpb:stencil parseScalarMapF64xS   parseScalarMap[fixed64Item, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapF64xB   parseScalarMap[fixed64Item, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//fastpb:stencil parseScalarMapSxV32 parseScalarMap[stringItem, varintItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapSxV64 parseScalarMap[stringItem, varintItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapSxZ32 parseScalarMap[stringItem, zigzagItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapSxZ64 parseScalarMap[stringItem, zigzagItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapSxF32 parseScalarMap[stringItem, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapSxF64 parseScalarMap[stringItem, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapSxV1  parseScalarMap[stringItem, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//fastpb:stencil parseScalarMapSxS   parseScalarMap[stringItem, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapSxB   parseScalarMap[stringItem, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

// parseScalarMap parses a map type whose value is a non-message type.
func parseScalarMap[
	KI mapItem[K], VI mapItem[V],
	K swiss.Key, V any,
](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.lengthPrefix(p2)

	p2.scratch = uint64(p1.e_)
	p1.e_ = p1.b_.Add(int(n))

	var ki KI
	var vi VI
	var k K
	var v V

	kTag := protowire.EncodeTag(1, ki.kind())
	vTag := protowire.EncodeTag(2, vi.kind())

	// Basically every map ever encodes its fields in order and does not
	// have duplicate fields, so this is a hot fast path.
	if p1.len() == 0 {
		goto insert
	}
	p1.log(p2, "first byte", "%#02x", *p1.b())
	if *p1.b() == byte(kTag) {
		p1.b_++
		p1, p2, k = ki.parse(p1, p2)
		if p1.len() == 0 {
			goto insert
		}
		p1.log(p2, "second byte", "%#02x", *p1.b())
		if *p1.b() == byte(vTag) {
			p1.b_++
			p1, p2, v = vi.parse(p1, p2)
			p1.log(p2, "map done?",
				"%v:%v, %v/%x: %v/%x",
				p1.b_, p1.e_,
				k, unsafe2.Bytes(&k),
				v, unsafe2.Bytes(&v))
			if p1.b_ == p1.e_ {
				goto insert
			}
		}
	}

	// Slow fallback. This code should almost never be executed so we can
	// afford to call varint() each time we parse a tag.
	for p1.b_ < p1.e_ {
		var tag uint64
		p1, p2, tag = p1.varint(p2)
		switch tag {
		case kTag:
			p1, p2, k = ki.parse(p1, p2)
		case vTag:
			p1, p2, v = vi.parse(p1, p2)
		default:
			n, t := protowire.DecodeTag(tag)
			m := protowire.ConsumeFieldValue(n, t, p1.buf())
			p1.b_ = p1.b_.Add(m)
		}
	}

insert:
	extract := ki.extract(p1, p2)
	var mp **swiss.Table[K, V]
	p1, p2, mp = getMutableField[*swiss.Table[K, V]](p1, p2)

	m := *mp
	if m == nil {
		size, _ := swiss.Layout[K, V](1)
		m = unsafe2.Cast[swiss.Table[K, V]](p1.arena().Alloc(size))
		unsafe2.StoreNoWB(mp, m)
		m.Init(1, nil, extract)
	}

	vp := m.Insert(k, extract)
	if vp == nil {
		size, _ := swiss.Layout[K, V](m.Len() + 1)
		m2 := unsafe2.Cast[swiss.Table[K, V]](p1.arena().Alloc(size))
		unsafe2.StoreNoWB(mp, m2)
		m2.Init(m.Len()+1, m, extract)
		vp = m2.Insert(k, extract)
	}

	*vp = v

	p1.e_ = unsafe2.Addr[byte](p2.scratch)
	return p1, p2
}

//fastpb:stencil parseBoolScalarMapV32 parseBoolScalarMap[varintItem[uint32], uint32]
//fastpb:stencil parseBoolScalarMapV64 parseBoolScalarMap[varintItem[uint64], uint64]
//fastpb:stencil parseBoolScalarMapZ32 parseBoolScalarMap[zigzagItem[uint32], uint32]
//fastpb:stencil parseBoolScalarMapZ64 parseBoolScalarMap[zigzagItem[uint64], uint64]
//fastpb:stencil parseBoolScalarMapF32 parseBoolScalarMap[fixed32Item, uint32]
//fastpb:stencil parseBoolScalarMapF64 parseBoolScalarMap[fixed64Item, uint64]
//fastpb:stencil parseBoolScalarMapV1  parseBoolScalarMap[boolItem, uint8]
//fastpb:stencil parseBoolScalarMapS   parseBoolScalarMap[stringItem, uint64]
//fastpb:stencil parseBoolScalarMapB   parseBoolScalarMap[bytesItem, uint64]

// parseBoolScalarMap parses a map type whose key is bool and whose value is a non-message type.
func parseBoolScalarMap[
	VI mapItem[V], V integer,
](p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.lengthPrefix(p2)

	p2.scratch = uint64(p1.e_)
	p1.e_ = p1.b_.Add(int(n))

	var p *V
	p1, p2, p = getMutableField[V](p1, p2)

	var k bool
	var vi VI
	var v V

	kTag := protowire.EncodeTag(1, protowire.VarintType)
	vTag := protowire.EncodeTag(2, vi.kind())

	// Basically every map ever encodes its fields in order and does not
	// have duplicate fields, so this is a hot fast path.
	p1.log(p2, "first byte", "%#02x", *p1.b())
	if *p1.b() == byte(kTag) {
		p1.b_++
		var n uint64
		p1, p2, n = p1.varint(p2)
		k = n != 0
		p1.log(p2, "second byte", "%#02x", *p1.b())
		if *p1.b() == byte(vTag) {
			p1.b_++
			p1, p2, v = vi.parse(p1, p2)
			p1.log(p2, "map done?",
				"%v:%v, %v/%x: %v/%x",
				p1.b_, p1.e_,
				k, unsafe2.Bytes(&k),
				v, unsafe2.Bytes(&v))
			if p1.b_ == p1.e_ {
				goto insert
			}
		}
	}

	// Slow fallback. This code should almost never be executed so we can
	// afford to call varint() each time we parse a tag.
	for p1.b_ < p1.e_ {
		var tag uint64
		p1, p2, tag = p1.varint(p2)
		switch tag {
		case kTag:
			var n uint64
			p1, p2, n = p1.varint(p2)
			k = n != 0
		case vTag:
			p1, p2, v = vi.parse(p1, p2)
		default:
			n, t := protowire.DecodeTag(tag)
			m := protowire.ConsumeFieldValue(n, t, p1.buf())
			p1.b_ = p1.b_.Add(m)
		}
	}

insert:
	var idx uint32
	if k {
		idx = 1
	}

	*unsafe2.Add(p, idx) = v
	p2.m().setBit(p2.f().offset.bit+idx, true)

	p1.e_ = unsafe2.Addr[byte](p2.scratch)
	return p1, p2
}

// parseBoolBoolMap parses a map<bool, bool>.
func parseBoolBoolMap(p1 parser1, p2 parser2) (parser1, parser2) {
	var n uint32
	p1, p2, n = p1.lengthPrefix(p2)

	p2.scratch = uint64(p1.e_)
	p1.e_ = p1.b_.Add(int(n))

	var k, v bool

	kTag := protowire.EncodeTag(1, protowire.VarintType)
	vTag := protowire.EncodeTag(2, protowire.VarintType)

	// Basically every map ever encodes its fields in order and does not
	// have duplicate fields, so this is a hot fast path.
	if p1.len() == 0 {
		goto insert
	}
	p1.log(p2, "first byte", "%#02x", *p1.b())
	if *p1.b() == byte(kTag) {
		p1.b_++
		var n uint64
		p1, p2, n = p1.varint(p2)
		k = n != 0
		if p1.len() == 0 {
			goto insert
		}
		p1.log(p2, "second byte", "%#02x", *p1.b())
		if *p1.b() == byte(vTag) {
			p1.b_++
			var n uint64
			p1, p2, n = p1.varint(p2)
			v = n != 0
			p1.log(p2, "map done?",
				"%v:%v, %v/%x: %v/%x",
				p1.b_, p1.e_,
				k, unsafe2.Bytes(&k),
				v, unsafe2.Bytes(&v))
			if p1.b_ == p1.e_ {
				goto insert
			}
		}
	}

	// Slow fallback. This code should almost never be executed so we can
	// afford to call varint() each time we parse a tag.
	for p1.b_ < p1.e_ {
		var tag uint64
		p1, p2, tag = p1.varint(p2)
		switch tag {
		case kTag:
			var n uint64
			p1, p2, n = p1.varint(p2)
			k = n != 0
		case vTag:
			var n uint64
			p1, p2, n = p1.varint(p2)
			v = n != 0
		default:
			n, t := protowire.DecodeTag(tag)
			m := protowire.ConsumeFieldValue(n, t, p1.buf())
			p1.b_ = p1.b_.Add(m)
		}
	}

insert:
	var idx uint32
	if k {
		idx = 1
	}

	p2.m().setBit(p2.f().offset.bit+idx, true)
	p2.m().setBit(p2.f().offset.bit+idx+2, v)

	p1.e_ = unsafe2.Addr[byte](p2.scratch)
	return p1, p2
}

type unimplementedMap struct{}

var _ protoreflect.Map = unimplementedMap{}

func (unimplementedMap) IsValid() bool                                  { return true }
func (unimplementedMap) Clear(protoreflect.MapKey)                      { panic(dbg.Unsupported()) }
func (unimplementedMap) Get(protoreflect.MapKey) protoreflect.Value     { panic(dbg.Unsupported()) }
func (unimplementedMap) Has(protoreflect.MapKey) bool                   { panic(dbg.Unsupported()) }
func (unimplementedMap) Len() int                                       { panic(dbg.Unsupported()) }
func (unimplementedMap) Mutable(protoreflect.MapKey) protoreflect.Value { panic(dbg.Unsupported()) }
func (unimplementedMap) NewValue() protoreflect.Value                   { panic(dbg.Unsupported()) }
func (unimplementedMap) Set(protoreflect.MapKey, protoreflect.Value)    { panic(dbg.Unsupported()) }
func (unimplementedMap) Range(f func(protoreflect.MapKey, protoreflect.Value) bool) {
	panic(dbg.Unsupported())
}
