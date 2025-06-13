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
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/debug"
	"github.com/bufbuild/fastpb/internal/swiss"
	"github.com/bufbuild/fastpb/internal/tdp/compiler"
	"github.com/bufbuild/fastpb/internal/tdp/dynamic"
	"github.com/bufbuild/fastpb/internal/tdp/vm"
	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
	"github.com/bufbuild/fastpb/internal/zc"
)

// map<K, V>, for K an integer type, is implemented as a swiss.Table of that
// type, while map<string, V> and map<bytes, V> are both implemented as a
// swiss.Table[zc, _], requiring the original buffer's source to perform
// lookups.

// mapFields consists of archetypes for map fields. The first index is the key,
// the second is the value.
var mapFields = map[protoreflect.Kind]map[protoreflect.Kind]*compiler.Archetype{
	protoreflect.Int32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int32, int32], parseMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int32, uint32], parseMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int32, int32], parseMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int32, int64], parseMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int32, uint64], parseMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int32, int64], parseMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int32, uint32], parseMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int32, int32], parseMapV32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int32, float32], parseMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int32, uint64], parseMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int32, int64], parseMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int32, float64], parseMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int32, bool], parseMapV32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int32, protoreflect.EnumNumber], parseMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int32], parseMapV32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int32], parseMapV32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int32], parseMapV32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Int64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int64, int32], parseMapV64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int64, uint32], parseMapV64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int64, int32], parseMapV64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int64, int64], parseMapV64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int64, uint64], parseMapV64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int64, int64], parseMapV64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int64, uint32], parseMapV64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int64, int32], parseMapV64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int64, float32], parseMapV64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int64, uint64], parseMapV64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int64, int64], parseMapV64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int64, float64], parseMapV64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int64, bool], parseMapV64x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int64, protoreflect.EnumNumber], parseMapV64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int64], parseMapV64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int64], parseMapV64xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int64], parseMapV64xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Uint32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint32, int32], parseMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint32, uint32], parseMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint32, int32], parseMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint32, int64], parseMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint32, uint64], parseMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint32, int64], parseMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint32, uint32], parseMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint32, int32], parseMapV32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint32, float32], parseMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint32, uint64], parseMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint32, int64], parseMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint32, float64], parseMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint32, bool], parseMapV32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[uint32, protoreflect.EnumNumber], parseMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint32], parseMapV32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint32], parseMapV32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[uint32], parseMapV32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Uint64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint64, int32], parseMapV64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint64, uint32], parseMapV64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint64, int32], parseMapV64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint64, int64], parseMapV64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint64, uint64], parseMapV64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint64, int64], parseMapV64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint64, uint32], parseMapV64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint64, int32], parseMapV64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint64, float32], parseMapV64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint64, uint64], parseMapV64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint64, int64], parseMapV64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint64, float64], parseMapV64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint64, bool], parseMapV64x2),
		protoreflect.EnumKind: mapArch(getMapIxI[uint64, protoreflect.EnumNumber], parseMapV64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint64], parseMapV64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint64], parseMapV64xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[uint64], parseMapV64xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Sint32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int32, int32], parseMapZ32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int32, uint32], parseMapZ32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int32, int32], parseMapZ32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int32, int64], parseMapZ32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int32, uint64], parseMapZ32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int32, int64], parseMapZ32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int32, uint32], parseMapZ32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int32, int32], parseMapZ32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int32, float32], parseMapZ32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int32, uint64], parseMapZ32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int32, int64], parseMapZ32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int32, float64], parseMapZ32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int32, bool], parseMapZ32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int32, protoreflect.EnumNumber], parseMapZ32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int32], parseMapZ32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int32], parseMapZ32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int32], parseMapZ32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Sint64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int64, int32], parseMapZ64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int64, uint32], parseMapZ64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int64, int32], parseMapZ64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int64, int64], parseMapZ64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int64, uint64], parseMapZ64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int64, int64], parseMapZ64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int64, uint32], parseMapZ64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int64, int32], parseMapZ64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int64, float32], parseMapZ64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int64, uint64], parseMapZ64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int64, int64], parseMapZ64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int64, float64], parseMapZ64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int64, bool], parseMapZ64x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int64, protoreflect.EnumNumber], parseMapZ64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int64], parseMapZ64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int64], parseMapZ64xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int64], parseMapZ64xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},

	protoreflect.Fixed32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint32, int32], parseMapF32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint32, uint32], parseMapF32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint32, int32], parseMapF32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint32, int64], parseMapF32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint32, uint64], parseMapF32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint32, int64], parseMapF32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint32, uint32], parseMapF32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint32, int32], parseMapF32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint32, float32], parseMapF32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint32, uint64], parseMapF32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint32, int64], parseMapF32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint32, float64], parseMapF32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint32, bool], parseMapF32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[uint32, protoreflect.EnumNumber], parseMapF32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint32], parseMapF32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint32], parseMapF32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[uint32], parseMapF32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Fixed64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint64, int32], parseMapF64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint64, uint32], parseMapF64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint64, int32], parseMapF64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint64, int64], parseMapF64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint64, uint64], parseMapF64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint64, int64], parseMapF64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint64, uint32], parseMapF64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint64, int32], parseMapF64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint64, float32], parseMapF64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint64, uint64], parseMapF64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint64, int64], parseMapF64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint64, float64], parseMapF64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint64, bool], parseMapF64x2),
		protoreflect.EnumKind: mapArch(getMapIxI[uint64, protoreflect.EnumNumber], parseMapF64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint64], parseMapF64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint64], parseMapF64xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[uint64], parseMapF64xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Sfixed32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int32, int32], parseMapF32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int32, uint32], parseMapF32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int32, int32], parseMapF32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int32, int64], parseMapF32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int32, uint64], parseMapF32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int32, int64], parseMapF32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int32, uint32], parseMapF32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int32, int32], parseMapF32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int32, float32], parseMapF32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int32, uint64], parseMapF32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int32, int64], parseMapF32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int32, float64], parseMapF32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int32, bool], parseMapF32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int32, protoreflect.EnumNumber], parseMapF32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int32], parseMapF32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int32], parseMapF32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int32], parseMapF32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
	protoreflect.Sfixed64Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int64, int32], parseMapF64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int64, uint32], parseMapF64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int64, int32], parseMapF64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int64, int64], parseMapF64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int64, uint64], parseMapF64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int64, int64], parseMapF64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int64, uint32], parseMapF64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int64, int32], parseMapF64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int64, float32], parseMapF64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int64, uint64], parseMapF64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int64, int64], parseMapF64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int64, float64], parseMapF64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int64, bool], parseMapF64x2),
		protoreflect.EnumKind: mapArch(getMapIxI[int64, protoreflect.EnumNumber], parseMapF64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int64], parseMapF64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int64], parseMapF64xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[int64], parseMapF64xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},

	protoreflect.BoolKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMap2xI[int32], parseMap2xV32),
		protoreflect.Uint32Kind: mapArch(getMap2xI[uint32], parseMap2xV32),
		protoreflect.Sint32Kind: mapArch(getMap2xI[int32], parseMap2xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMap2xI[int64], parseMap2xV64),
		protoreflect.Uint64Kind: mapArch(getMap2xI[uint64], parseMap2xV64),
		protoreflect.Sint64Kind: mapArch(getMap2xI[int64], parseMap2xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMap2xI[uint32], parseMap2xF32),
		protoreflect.Sfixed32Kind: mapArch(getMap2xI[int32], parseMap2xF32),
		protoreflect.FloatKind:    mapArch(getMap2xI[float32], parseMap2xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMap2xI[uint64], parseMap2xF64),
		protoreflect.Sfixed64Kind: mapArch(getMap2xI[int64], parseMap2xF64),
		protoreflect.DoubleKind:   mapArch(getMap2xI[float64], parseMap2xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMap2x2, parseMap2x2),
		protoreflect.EnumKind: mapArch(getMap2xI[protoreflect.EnumNumber], parseMap2xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMap2xS, parseMap2xS),
		protoreflect.BytesKind:  mapArch(getMap2xB, parseMap2xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMap2xM, parseMap2xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},

	protoreflect.EnumKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, int32], parseMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[protoreflect.EnumNumber, uint32], parseMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int32], parseMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, int64], parseMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[protoreflect.EnumNumber, uint64], parseMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int64], parseMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, uint32], parseMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int32], parseMapV32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[protoreflect.EnumNumber, float32], parseMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, uint64], parseMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int64], parseMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[protoreflect.EnumNumber, float64], parseMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[protoreflect.EnumNumber, bool], parseMapV32x2),
		protoreflect.EnumKind: mapArch(getMapIxI[protoreflect.EnumNumber, protoreflect.EnumNumber], parseMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[protoreflect.EnumNumber], parseMapV32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[protoreflect.EnumNumber], parseMapV32xB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapIxM[protoreflect.EnumNumber], parseMapV32xM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},

	protoreflect.StringKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapSxI[int32], parseMapSxV32),
		protoreflect.Uint32Kind: mapArch(getMapSxI[uint32], parseMapSxV32),
		protoreflect.Sint32Kind: mapArch(getMapSxI[int32], parseMapSxZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapSxI[int64], parseMapSxV64),
		protoreflect.Uint64Kind: mapArch(getMapSxI[uint64], parseMapSxV64),
		protoreflect.Sint64Kind: mapArch(getMapSxI[int64], parseMapSxZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapSxI[uint32], parseMapSxF32),
		protoreflect.Sfixed32Kind: mapArch(getMapSxI[int32], parseMapSxF32),
		protoreflect.FloatKind:    mapArch(getMapSxI[float32], parseMapSxF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapSxI[uint64], parseMapSxF64),
		protoreflect.Sfixed64Kind: mapArch(getMapSxI[int64], parseMapSxF64),
		protoreflect.DoubleKind:   mapArch(getMapSxI[float64], parseMapSxF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapSxI[bool], parseMapSx2),
		protoreflect.EnumKind: mapArch(getMapSxI[protoreflect.EnumNumber], parseMapSxV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapSxS, parseMapSxS),
		protoreflect.BytesKind:  mapArch(getMapSxB, parseMapSxB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapSxM, parseMapSxM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},

	proto2StringKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapSxI[int32], parseMapBxV32),
		protoreflect.Uint32Kind: mapArch(getMapSxI[uint32], parseMapBxV32),
		protoreflect.Sint32Kind: mapArch(getMapSxI[int32], parseMapBxZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapSxI[int64], parseMapBxV64),
		protoreflect.Uint64Kind: mapArch(getMapSxI[uint64], parseMapBxV64),
		protoreflect.Sint64Kind: mapArch(getMapSxI[int64], parseMapBxZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapSxI[uint32], parseMapBxF32),
		protoreflect.Sfixed32Kind: mapArch(getMapSxI[int32], parseMapBxF32),
		protoreflect.FloatKind:    mapArch(getMapSxI[float32], parseMapBxF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapSxI[uint64], parseMapBxF64),
		protoreflect.Sfixed64Kind: mapArch(getMapSxI[int64], parseMapBxF64),
		protoreflect.DoubleKind:   mapArch(getMapSxI[float64], parseMapBxF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapSxI[bool], parseMapBx2),
		protoreflect.EnumKind: mapArch(getMapSxI[protoreflect.EnumNumber], parseMapBxV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapSxS, parseMapBxS),
		protoreflect.BytesKind:  mapArch(getMapSxB, parseMapBxB),

		// Message types.
		protoreflect.MessageKind: mapArch(getMapSxM, parseMapBxM),
		protoreflect.GroupKind:   {
			// Not implemented.
		},
	},
}

func init() {
	// Generate each of the entries for proto2StringKind by making copies of
	// the string archetype and using the bytes archetype's parser.
	for _, archs := range mapFields {
		arch := *archs[protoreflect.StringKind]
		arch.Parsers = archs[protoreflect.BytesKind].Parsers
		archs[proto2StringKind] = &arch
	}
}

// mapArch is a helper for constructing map<K, V> archetypes, where K is not
// bool.
func mapArch(getter compiler.Getter, parser vm.Thunk) *compiler.Archetype {
	return &compiler.Archetype{
		Layout:  layout.Of[*swiss.Table[int32, int32]](),
		Getter:  getter,
		Parsers: []compiler.Parser{{Kind: protowire.BytesType, Retry: true, Thunk: parser}},
	}
}

// mapItem is a type usable in any of the map parsers. This is essentially a
// shim for pushing slight custom behavior modifications to each of the stencils
// of e.g. [parseMapKxV].
type mapItem[V any] interface {
	// The wire type for this item.
	kind() protowire.Type

	// Parses a value of this item type and returns it.
	parse(vm.P1, vm.P2) (vm.P1, vm.P2, V)

	// Returns the key extraction function used with swiss.Table.Insert.
	extract(vm.P1, vm.P2) func(V) []byte
}

type (
	varintItem[T uint32 | uint64] struct{}
	zigzagItem[T uint32 | uint64] struct{}

	boolItem    struct{}
	fixed32Item struct{}
	fixed64Item struct{}
	stringItem  struct{}
	bytesItem   struct{}
)

var (
	_ mapItem[uint8]  = boolItem{}
	_ mapItem[uint32] = fixed32Item{}
	_ mapItem[uint64] = fixed64Item{}
	_ mapItem[uint64] = stringItem{}
	_ mapItem[uint64] = bytesItem{}
)

func (varintItem[_]) kind() protowire.Type { return protowire.VarintType }
func (zigzagItem[_]) kind() protowire.Type { return protowire.VarintType }
func (boolItem) kind() protowire.Type      { return protowire.VarintType }
func (fixed32Item) kind() protowire.Type   { return protowire.Fixed32Type }
func (fixed64Item) kind() protowire.Type   { return protowire.Fixed64Type }
func (stringItem) kind() protowire.Type    { return protowire.BytesType }
func (bytesItem) kind() protowire.Type     { return protowire.BytesType }

func (varintItem[T]) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, T) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	return p1, p2, T(n)
}

func (zigzagItem[T]) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, T) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	return p1, p2, zigzag64[T](n)
}

func (boolItem) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint8) {
	var n uint64
	p1, p2, n = p1.Varint(p2)
	if n != 0 {
		n = 1
	}
	return p1, p2, uint8(n)
}

func (fixed32Item) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint32) {
	return p1.Fixed32(p2)
}

func (fixed64Item) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint64) {
	return p1.Fixed64(p2)
}

func (stringItem) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint64) {
	var r zc.Range
	p1, p2, r = p1.UTF8(p2)
	return p1, p2, uint64(r)
}

func (bytesItem) parse(p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2, uint64) {
	var r zc.Range
	p1, p2, r = p1.Bytes(p2)
	return p1, p2, uint64(r)
}

func (varintItem[T]) extract(vm.P1, vm.P2) func(T) []byte    { return nil }
func (zigzagItem[T]) extract(vm.P1, vm.P2) func(T) []byte    { return nil }
func (fixed32Item) extract(vm.P1, vm.P2) func(uint32) []byte { return nil }
func (fixed64Item) extract(vm.P1, vm.P2) func(uint64) []byte { return nil }
func (boolItem) extract(vm.P1, vm.P2) func(uint8) []byte     { return nil }
func (stringItem) extract(p1 vm.P1, _ vm.P2) func(uint64) []byte {
	src := p1.Src()
	return func(u uint64) []byte {
		return zc.Range(u).Bytes(src)
	}
}

func (bytesItem) extract(p1 vm.P1, _ vm.P2) func(uint64) []byte {
	src := p1.Src()
	return func(u uint64) []byte {
		return zc.Range(u).Bytes(src)
	}
}

//fastpb:stencil parseMapV32xV32 parseMapKxV[varintItem[uint32], varintItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseMapV32xV64 parseMapKxV[varintItem[uint32], varintItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapV32xZ32 parseMapKxV[varintItem[uint32], zigzagItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseMapV32xZ64 parseMapKxV[varintItem[uint32], zigzagItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapV32xF32 parseMapKxV[varintItem[uint32], fixed32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseMapV32xF64 parseMapKxV[varintItem[uint32], fixed64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapV32x2   parseMapKxV[varintItem[uint32], boolItem, uint32, uint8] Init -> swiss.InitU32xU8 Insert -> swiss.InsertU32xU8
//fastpb:stencil parseMapV32xS   parseMapKxV[varintItem[uint32], stringItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapV32xB   parseMapKxV[varintItem[uint32], bytesItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64

//fastpb:stencil parseMapV64xV32 parseMapKxV[varintItem[uint64], varintItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapV64xV64 parseMapKxV[varintItem[uint64], varintItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapV64xZ32 parseMapKxV[varintItem[uint64], zigzagItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapV64xZ64 parseMapKxV[varintItem[uint64], zigzagItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapV64xF32 parseMapKxV[varintItem[uint64], fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapV64xF64 parseMapKxV[varintItem[uint64], fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapV64x2   parseMapKxV[varintItem[uint64], boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//fastpb:stencil parseMapV64xS   parseMapKxV[varintItem[uint64], stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapV64xB   parseMapKxV[varintItem[uint64], bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//fastpb:stencil parseMapZ32xV32 parseMapKxV[zigzagItem[uint32], varintItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseMapZ32xV64 parseMapKxV[zigzagItem[uint32], varintItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapZ32xZ32 parseMapKxV[zigzagItem[uint32], zigzagItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseMapZ32xZ64 parseMapKxV[zigzagItem[uint32], zigzagItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapZ32xF32 parseMapKxV[zigzagItem[uint32], fixed32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseMapZ32xF64 parseMapKxV[zigzagItem[uint32], fixed64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapZ32x2   parseMapKxV[zigzagItem[uint32], boolItem, uint32, uint8] Init -> swiss.InitU32xU8 Insert -> swiss.InsertU32xU8
//fastpb:stencil parseMapZ32xS   parseMapKxV[zigzagItem[uint32], stringItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapZ32xB   parseMapKxV[zigzagItem[uint32], bytesItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64

//fastpb:stencil parseMapZ64xV32 parseMapKxV[zigzagItem[uint64], varintItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapZ64xV64 parseMapKxV[zigzagItem[uint64], varintItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapZ64xZ32 parseMapKxV[zigzagItem[uint64], zigzagItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapZ64xZ64 parseMapKxV[zigzagItem[uint64], zigzagItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapZ64xF32 parseMapKxV[zigzagItem[uint64], fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapZ64xF64 parseMapKxV[zigzagItem[uint64], fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapZ64x2   parseMapKxV[zigzagItem[uint64], boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//fastpb:stencil parseMapZ64xS   parseMapKxV[zigzagItem[uint64], stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapZ64xB   parseMapKxV[zigzagItem[uint64], bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//fastpb:stencil parseMapF32xV32 parseMapKxV[fixed32Item, varintItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseMapF32xV64 parseMapKxV[fixed32Item, varintItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapF32xZ32 parseMapKxV[fixed32Item, zigzagItem[uint32], uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseMapF32xZ64 parseMapKxV[fixed32Item, zigzagItem[uint64], uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapF32xF32 parseMapKxV[fixed32Item, fixed32Item, uint32, uint32] Init -> swiss.InitU32xU32 Insert -> swiss.InsertU32xU32
//fastpb:stencil parseMapF32xF64 parseMapKxV[fixed32Item, fixed64Item, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapF32x2   parseMapKxV[fixed32Item, boolItem, uint32, uint8] Init -> swiss.InitU32xU8 Insert -> swiss.InsertU32xU8
//fastpb:stencil parseMapF32xS   parseMapKxV[fixed32Item, stringItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64
//fastpb:stencil parseMapF32xB   parseMapKxV[fixed32Item, bytesItem, uint32, uint64] Init -> swiss.InitU32xU64 Insert -> swiss.InsertU32xU64

//fastpb:stencil parseMapF64xV32 parseMapKxV[fixed64Item, varintItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapF64xV64 parseMapKxV[fixed64Item, varintItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapF64xZ32 parseMapKxV[fixed64Item, zigzagItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapF64xZ64 parseMapKxV[fixed64Item, zigzagItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapF64xF32 parseMapKxV[fixed64Item, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapF64xF64 parseMapKxV[fixed64Item, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapF64x2   parseMapKxV[fixed64Item, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//fastpb:stencil parseMapF64xS   parseMapKxV[fixed64Item, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapF64xB   parseMapKxV[fixed64Item, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//fastpb:stencil parseMapSxV32 parseMapKxV[stringItem, varintItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapSxV64 parseMapKxV[stringItem, varintItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapSxZ32 parseMapKxV[stringItem, zigzagItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapSxZ64 parseMapKxV[stringItem, zigzagItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapSxF32 parseMapKxV[stringItem, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapSxF64 parseMapKxV[stringItem, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapSx2   parseMapKxV[stringItem, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//fastpb:stencil parseMapSxS   parseMapKxV[stringItem, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapSxB   parseMapKxV[stringItem, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//fastpb:stencil parseMapBxV32 parseMapKxV[bytesItem, varintItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapBxV64 parseMapKxV[bytesItem, varintItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapBxZ32 parseMapKxV[bytesItem, zigzagItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapBxZ64 parseMapKxV[bytesItem, zigzagItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapBxF32 parseMapKxV[bytesItem, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseMapBxF64 parseMapKxV[bytesItem, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapBx2   parseMapKxV[bytesItem, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//fastpb:stencil parseMapBxS   parseMapKxV[bytesItem, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseMapBxB   parseMapKxV[bytesItem, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

//fastpb:stencil parseMap2xV32 parseMapKxV[boolItem, varintItem[uint32], uint8, uint32] Init -> swiss.InitU8xU32 Insert -> swiss.InsertU8xU32
//fastpb:stencil parseMap2xV64 parseMapKxV[boolItem, varintItem[uint64], uint8, uint64] Init -> swiss.InitU8xU64 Insert -> swiss.InsertU8xU64
//fastpb:stencil parseMap2xZ32 parseMapKxV[boolItem, zigzagItem[uint32], uint8, uint32] Init -> swiss.InitU8xU32 Insert -> swiss.InsertU8xU32
//fastpb:stencil parseMap2xZ64 parseMapKxV[boolItem, zigzagItem[uint64], uint8, uint64] Init -> swiss.InitU8xU64 Insert -> swiss.InsertU8xU64
//fastpb:stencil parseMap2xF32 parseMapKxV[boolItem, fixed32Item, uint8, uint32] Init -> swiss.InitU8xU32 Insert -> swiss.InsertU8xU32
//fastpb:stencil parseMap2xF64 parseMapKxV[boolItem, fixed64Item, uint8, uint64] Init -> swiss.InitU8xU64 Insert -> swiss.InsertU8xU64
//fastpb:stencil parseMap2x2   parseMapKxV[boolItem, boolItem, uint8, uint8] Init -> swiss.InitU8xU8 Insert -> swiss.InsertU8xU8
//fastpb:stencil parseMap2xS   parseMapKxV[boolItem, stringItem, uint8, uint64] Init -> swiss.InitU8xU64 Insert -> swiss.InsertU8xU64
//fastpb:stencil parseMap2xB   parseMapKxV[boolItem, bytesItem, uint8, uint64] Init -> swiss.InitU8xU64 Insert -> swiss.InsertU8xU64

// parseMapKxV parses a map type whose value is a non-message type.
func parseMapKxV[
	KI mapItem[K], VI mapItem[V],
	K swiss.Key, V any,
](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n int
	p1, p2, n = p1.LengthPrefix(p2)

	p2.Scratch = uint64(p1.EndAddr)
	p1.EndAddr = p1.PtrAddr.Add(n)

	var ki KI
	var vi VI
	var k K
	var v V

	kTag := protowire.EncodeTag(1, ki.kind())
	vTag := protowire.EncodeTag(2, vi.kind())

	// Basically every map ever encodes its fields in order and does not
	// have duplicate fields, so this is a hot fast path.
	if p1.Len() == 0 {
		goto insert
	}
	p1.Log(p2, "first byte", "%#02x", *p1.Ptr())
	if *p1.Ptr() == byte(kTag) {
		p1.PtrAddr++
		p1, p2, k = ki.parse(p1, p2)
		if p1.Len() == 0 {
			goto insert
		}
		p1.Log(p2, "second byte", "%#02x", *p1.Ptr())
		if *p1.Ptr() == byte(vTag) {
			p1.PtrAddr++
			p1, p2, v = vi.parse(p1, p2)
			p1.Log(p2, "map done?",
				"%v:%v, %v/%x: %v/%x",
				p1.PtrAddr, p1.EndAddr,
				k, unsafe2.Bytes(&k),
				v, unsafe2.Bytes(&v))
			if p1.PtrAddr == p1.EndAddr {
				goto insert
			}
		}
	}

	// Slow fallback. This code should almost never be executed so we can
	// afford to call varint() each time we parse a tag.
	for p1.PtrAddr < p1.EndAddr {
		var tag uint64
		p1, p2, tag = p1.Varint(p2)
		switch tag {
		case kTag:
			p1, p2, k = ki.parse(p1, p2)
		case vTag:
			p1, p2, v = vi.parse(p1, p2)
		default:
			n, t := protowire.DecodeTag(tag)
			m := protowire.ConsumeFieldValue(n, t, p1.Buf())
			if m < 0 {
				p1.Fail(p2, -vm.ErrorCode(m))
			}
			p1.PtrAddr = p1.PtrAddr.Add(m)
		}
	}

insert:
	extract := ki.extract(p1, p2)
	var mp **swiss.Table[K, V]
	p1, p2, mp = vm.GetMutableField[*swiss.Table[K, V]](p1, p2)

	m := *mp
	if m == nil {
		size, _ := swiss.Layout[K, V](1)
		m = unsafe2.Cast[swiss.Table[K, V]](p1.Arena().Alloc(size))
		unsafe2.StoreNoWB(mp, m)
		m.Init(1, nil, extract)
	}

	vp := m.Insert(k, extract)
	if vp == nil {
		size, _ := swiss.Layout[K, V](m.Len() + 1)
		m2 := unsafe2.Cast[swiss.Table[K, V]](p1.Arena().Alloc(size))
		unsafe2.StoreNoWB(mp, m2)
		m2.Init(m.Len()+1, m, extract)
		vp = m2.Insert(k, extract)
	}

	*vp = v

	p1.EndAddr = unsafe2.Addr[byte](p2.Scratch)
	return p1, p2
}

//fastpb:stencil parseMapV32xM parseMapKxM[varintItem[uint32], uint32] Init -> swiss.InitU32xP Insert -> swiss.InsertU32xP
//fastpb:stencil parseMapV64xM parseMapKxM[varintItem[uint64], uint64] Init -> swiss.InitU64xP Insert -> swiss.InsertU64xP
//fastpb:stencil parseMapZ32xM parseMapKxM[zigzagItem[uint32], uint32] Init -> swiss.InitU32xP Insert -> swiss.InsertU32xP
//fastpb:stencil parseMapZ64xM parseMapKxM[zigzagItem[uint64], uint64] Init -> swiss.InitU64xP Insert -> swiss.InsertU64xP
//fastpb:stencil parseMapF32xM parseMapKxM[fixed32Item, uint32] Init -> swiss.InitU32xP Insert -> swiss.InsertU32xP
//fastpb:stencil parseMapF64xM parseMapKxM[fixed64Item, uint64] Init -> swiss.InitU64xP Insert -> swiss.InsertU64xP
//fastpb:stencil parseMapSxM   parseMapKxM[stringItem, uint64] Init -> swiss.InitU64xP Insert -> swiss.InsertU64xP
//fastpb:stencil parseMapBxM   parseMapKxM[bytesItem, uint64] Init -> swiss.InitU64xP Insert -> swiss.InsertU64xP
//fastpb:stencil parseMap2xM   parseMapKxM[boolItem, uint8] Init -> swiss.InitU8xP Insert -> swiss.InsertU8xP

// parseMapKxM parses a map type whose value is a message type.
func parseMapKxM[KI mapItem[K], K swiss.Key](p1 vm.P1, p2 vm.P2) (vm.P1, vm.P2) {
	var n int
	p1, p2, n = p1.LengthPrefix(p2)

	p2.Scratch = uint64(p1.EndAddr)
	p1.EndAddr = p1.PtrAddr.Add(n)

	var ki KI
	var k K
	var fast bool

	kTag := protowire.EncodeTag(1, ki.kind())
	vTag := protowire.EncodeTag(1, protowire.BytesType)

	// Basically every map ever encodes its fields in order and does not
	// have duplicate fields, so this is a hot fast path.
	if p1.Len() == 0 {
		fast = true
		goto insert
	}
	p1.Log(p2, "first byte", "%#02x", *p1.Ptr())
	if *p1.Ptr() == byte(kTag) {
		p1.PtrAddr++
		p1, p2, k = ki.parse(p1, p2)
		if p1.Len() == 0 {
			fast = true
			goto insert
		}
		p1.Log(p2, "second byte", "%#02x", *p1.Ptr())
		if *p1.Ptr() == byte(vTag) {
			p1.PtrAddr++
			// Need to parse a length prefix and check if it reaches all the
			// way to the end of the message.
			p1, p2, n = p1.LengthPrefix(p2)
			if p1.EndAddr > p1.PtrAddr.Add(n) {
				fast = true
				goto insert
			}
		}
	}

	// Slow fallback. This code should almost never be executed so we can
	// afford to call varint() each time we parse a tag.
	for p1.PtrAddr < p1.EndAddr {
		var tag uint64
		p1, p2, tag = p1.Varint(p2)
		switch tag {
		case kTag:
			p1, p2, k = ki.parse(p1, p2)
		default:
			n, t := protowire.DecodeTag(tag)
			m := protowire.ConsumeFieldValue(n, t, p1.Buf())
			if m < 0 {
				p1.Fail(p2, -vm.ErrorCode(m))
			}
			p1.PtrAddr = p1.PtrAddr.Add(m)
		}
	}

	// Now we need to rewind back to the beginning.
	p1.PtrAddr = p1.EndAddr.Add(-n)

insert:
	type V = unsafe.Pointer

	extract := ki.extract(p1, p2)
	var mp **swiss.Table[K, V]
	p1, p2, mp = vm.GetMutableField[*swiss.Table[K, V]](p1, p2)

	m := *mp
	if m == nil {
		size, _ := swiss.Layout[K, V](1)
		m = unsafe2.Cast[swiss.Table[K, V]](p1.Arena().Alloc(size))
		unsafe2.StoreNoWB(mp, m)
		m.Init(1, nil, extract)
	}

	vp := m.Insert(k, extract)
	if vp == nil {
		size, _ := swiss.Layout[K, V](m.Len() + 1)
		m2 := unsafe2.Cast[swiss.Table[K, V]](p1.Arena().Alloc(size))
		unsafe2.StoreNoWB(mp, m2)
		m2.Init(m.Len()+1, m, extract)
		vp = m2.Insert(k, extract)
	}

	var v *dynamic.Message
	// Allocate unconditionally to match Go protobuf's behavior.
	// TODO: This could instead clear, but that optimization will almost never
	// be relevant, because no serializer will ever emit the same key twice.
	p1, p2, v = vm.AllocMessage(p1, p2)
	unsafe2.StoreNoWBUntyped(vp, unsafe.Pointer(v))

	// Unspill the old end pointer.
	p1.EndAddr = unsafe2.Addr[byte](p2.Scratch)

	// Schedule a message parse.
	if fast {
		p1.Log(p2, "fast map entry", "%d", n)
		return p1.PushMessage(p2, n, v)
	}

	p1.Log(p2, "slow map entry", "%d", n)
	return p1.PushMapEntry(p2, n, v)
}

// emptyMap is a map with no elements.
type emptyMap struct {
	unimplementedMap
}

func (emptyMap) IsValid() bool                                                  { return false }
func (emptyMap) Len() int                                                       { return 0 }
func (emptyMap) Has(mk protoreflect.MapKey) bool                                { return false }
func (emptyMap) Get(mk protoreflect.MapKey) protoreflect.Value                  { return protoreflect.ValueOf(nil) }
func (emptyMap) Range(yield func(protoreflect.MapKey, protoreflect.Value) bool) {}

// unimplementedMap is a map whose functions all panic, except IsValid.
type unimplementedMap struct{}

func (unimplementedMap) IsValid() bool                                  { return true }
func (unimplementedMap) Clear(protoreflect.MapKey)                      { panic(debug.Unsupported()) }
func (unimplementedMap) Get(protoreflect.MapKey) protoreflect.Value     { panic(debug.Unsupported()) }
func (unimplementedMap) Has(protoreflect.MapKey) bool                   { panic(debug.Unsupported()) }
func (unimplementedMap) Len() int                                       { panic(debug.Unsupported()) }
func (unimplementedMap) Mutable(protoreflect.MapKey) protoreflect.Value { panic(debug.Unsupported()) }
func (unimplementedMap) NewValue() protoreflect.Value                   { panic(debug.Unsupported()) }
func (unimplementedMap) Set(protoreflect.MapKey, protoreflect.Value)    { panic(debug.Unsupported()) }
func (unimplementedMap) Range(f func(protoreflect.MapKey, protoreflect.Value) bool) {
	panic(debug.Unsupported())
}
