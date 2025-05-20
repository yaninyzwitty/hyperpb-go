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

// map<K, V>, for K an integer type, is implemented as a swiss.Table of that
// type, while map<string, V> and map<bytes, V> are both implemented as a
// swiss.Table[zc, _], requiring the original buffer's source to perform
// lookups.

// mapFields consists of archetypes for map fields. The first index is the key,
// the second is the value.
var mapFields = map[protoreflect.Kind]map[protoreflect.Kind]*archetype{
	protoreflect.Int32Kind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[int32, int32], parseScalarMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int32, uint32], parseScalarMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int32, int32], parseScalarMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int32, int64], parseScalarMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int32, uint64], parseScalarMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int32, int64], parseScalarMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int32, uint32], parseScalarMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int32, int32], parseScalarMapV32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int32, float32], parseScalarMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int32, uint64], parseScalarMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int32, int64], parseScalarMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int32, float64], parseScalarMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int32, bool], parseScalarMapV32xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[int32, protoreflect.EnumNumber], parseScalarMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int32], parseScalarMapV32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int32], parseScalarMapV32xB),

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
		protoreflect.Int32Kind:  mapArch(getMapIxI[int64, int32], parseScalarMapV64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int64, uint32], parseScalarMapV64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int64, int32], parseScalarMapV64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int64, int64], parseScalarMapV64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int64, uint64], parseScalarMapV64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int64, int64], parseScalarMapV64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int64, uint32], parseScalarMapV64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int64, int32], parseScalarMapV64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int64, float32], parseScalarMapV64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int64, uint64], parseScalarMapV64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int64, int64], parseScalarMapV64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int64, float64], parseScalarMapV64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int64, bool], parseScalarMapV64xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[int64, protoreflect.EnumNumber], parseScalarMapV64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int64], parseScalarMapV64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int64], parseScalarMapV64xB),

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
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint32, int32], parseScalarMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint32, uint32], parseScalarMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint32, int32], parseScalarMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint32, int64], parseScalarMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint32, uint64], parseScalarMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint32, int64], parseScalarMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint32, uint32], parseScalarMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint32, int32], parseScalarMapV32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint32, float32], parseScalarMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint32, uint64], parseScalarMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint32, int64], parseScalarMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint32, float64], parseScalarMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint32, bool], parseScalarMapV32xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[uint32, protoreflect.EnumNumber], parseScalarMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint32], parseScalarMapV32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint32], parseScalarMapV32xB),

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
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint64, int32], parseScalarMapV64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint64, uint32], parseScalarMapV64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint64, int32], parseScalarMapV64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint64, int64], parseScalarMapV64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint64, uint64], parseScalarMapV64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint64, int64], parseScalarMapV64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint64, uint32], parseScalarMapV64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint64, int32], parseScalarMapV64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint64, float32], parseScalarMapV64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint64, uint64], parseScalarMapV64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint64, int64], parseScalarMapV64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint64, float64], parseScalarMapV64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint64, bool], parseScalarMapV64xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[uint64, protoreflect.EnumNumber], parseScalarMapV64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint64], parseScalarMapV64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint64], parseScalarMapV64xB),

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
		protoreflect.Int32Kind:  mapArch(getMapIxI[int32, int32], parseScalarMapZ32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int32, uint32], parseScalarMapZ32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int32, int32], parseScalarMapZ32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int32, int64], parseScalarMapZ32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int32, uint64], parseScalarMapZ32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int32, int64], parseScalarMapZ32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int32, uint32], parseScalarMapZ32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int32, int32], parseScalarMapZ32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int32, float32], parseScalarMapZ32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int32, uint64], parseScalarMapZ32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int32, int64], parseScalarMapZ32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int32, float64], parseScalarMapZ32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int32, bool], parseScalarMapZ32xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[int32, protoreflect.EnumNumber], parseScalarMapZ32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int32], parseScalarMapZ32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int32], parseScalarMapZ32xB),

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
		protoreflect.Int32Kind:  mapArch(getMapIxI[int64, int32], parseScalarMapZ64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int64, uint32], parseScalarMapZ64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int64, int32], parseScalarMapZ64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int64, int64], parseScalarMapZ64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int64, uint64], parseScalarMapZ64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int64, int64], parseScalarMapZ64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int64, uint32], parseScalarMapZ64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int64, int32], parseScalarMapZ64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int64, float32], parseScalarMapZ64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int64, uint64], parseScalarMapZ64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int64, int64], parseScalarMapZ64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int64, float64], parseScalarMapZ64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int64, bool], parseScalarMapZ64xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[int64, protoreflect.EnumNumber], parseScalarMapZ64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int64], parseScalarMapZ64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int64], parseScalarMapZ64xB),

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
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint32, int32], parseScalarMapF32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint32, uint32], parseScalarMapF32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint32, int32], parseScalarMapF32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint32, int64], parseScalarMapF32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint32, uint64], parseScalarMapF32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint32, int64], parseScalarMapF32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint32, uint32], parseScalarMapF32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint32, int32], parseScalarMapF32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint32, float32], parseScalarMapF32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint32, uint64], parseScalarMapF32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint32, int64], parseScalarMapF32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint32, float64], parseScalarMapF32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint32, bool], parseScalarMapF32xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[uint32, protoreflect.EnumNumber], parseScalarMapF32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint32], parseScalarMapF32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint32], parseScalarMapF32xB),

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
		protoreflect.Int32Kind:  mapArch(getMapIxI[uint64, int32], parseScalarMapF64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[uint64, uint32], parseScalarMapF64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[uint64, int32], parseScalarMapF64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[uint64, int64], parseScalarMapF64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[uint64, uint64], parseScalarMapF64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[uint64, int64], parseScalarMapF64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[uint64, uint32], parseScalarMapF64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[uint64, int32], parseScalarMapF64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[uint64, float32], parseScalarMapF64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[uint64, uint64], parseScalarMapF64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[uint64, int64], parseScalarMapF64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[uint64, float64], parseScalarMapF64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[uint64, bool], parseScalarMapF64xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[uint64, protoreflect.EnumNumber], parseScalarMapF64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[uint64], parseScalarMapF64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[uint64], parseScalarMapF64xB),

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
		protoreflect.Int32Kind:  mapArch(getMapIxI[int32, int32], parseScalarMapF32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int32, uint32], parseScalarMapF32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int32, int32], parseScalarMapF32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int32, int64], parseScalarMapF32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int32, uint64], parseScalarMapF32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int32, int64], parseScalarMapF32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int32, uint32], parseScalarMapF32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int32, int32], parseScalarMapF32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int32, float32], parseScalarMapF32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int32, uint64], parseScalarMapF32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int32, int64], parseScalarMapF32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int32, float64], parseScalarMapF32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int32, bool], parseScalarMapF32xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[int32, protoreflect.EnumNumber], parseScalarMapF32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int32], parseScalarMapF32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int32], parseScalarMapF32xB),

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
		protoreflect.Int32Kind:  mapArch(getMapIxI[int64, int32], parseScalarMapF64xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[int64, uint32], parseScalarMapF64xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[int64, int32], parseScalarMapF64xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[int64, int64], parseScalarMapF64xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[int64, uint64], parseScalarMapF64xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[int64, int64], parseScalarMapF64xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[int64, uint32], parseScalarMapF64xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[int64, int32], parseScalarMapF64xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[int64, float32], parseScalarMapF64xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[int64, uint64], parseScalarMapF64xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[int64, int64], parseScalarMapF64xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[int64, float64], parseScalarMapF64xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[int64, bool], parseScalarMapF64xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[int64, protoreflect.EnumNumber], parseScalarMapF64xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[int64], parseScalarMapF64xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[int64], parseScalarMapF64xB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},

	protoreflect.BoolKind: boolMapFields,

	protoreflect.EnumKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, int32], parseScalarMapV32xV32),
		protoreflect.Uint32Kind: mapArch(getMapIxI[protoreflect.EnumNumber, uint32], parseScalarMapV32xV32),
		protoreflect.Sint32Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int32], parseScalarMapV32xZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, int64], parseScalarMapV32xV64),
		protoreflect.Uint64Kind: mapArch(getMapIxI[protoreflect.EnumNumber, uint64], parseScalarMapV32xV64),
		protoreflect.Sint64Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int64], parseScalarMapV32xZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, uint32], parseScalarMapV32xF32),
		protoreflect.Sfixed32Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int32], parseScalarMapV32xF32),
		protoreflect.FloatKind:    mapArch(getMapIxI[protoreflect.EnumNumber, float32], parseScalarMapV32xF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapIxI[protoreflect.EnumNumber, uint64], parseScalarMapV32xF64),
		protoreflect.Sfixed64Kind: mapArch(getMapIxI[protoreflect.EnumNumber, int64], parseScalarMapV32xF64),
		protoreflect.DoubleKind:   mapArch(getMapIxI[protoreflect.EnumNumber, float64], parseScalarMapV32xF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapIxI[protoreflect.EnumNumber, bool], parseScalarMapV32xV1),
		protoreflect.EnumKind: mapArch(getMapIxI[protoreflect.EnumNumber, protoreflect.EnumNumber], parseScalarMapV32xV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapIxS[protoreflect.EnumNumber], parseScalarMapV32xS),
		protoreflect.BytesKind:  mapArch(getMapIxB[protoreflect.EnumNumber], parseScalarMapV32xB),

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
		protoreflect.Int32Kind:  mapArch(getMapSxI[int32], parseScalarMapSxV32),
		protoreflect.Uint32Kind: mapArch(getMapSxI[uint32], parseScalarMapSxV32),
		protoreflect.Sint32Kind: mapArch(getMapSxI[int32], parseScalarMapSxZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapSxI[int64], parseScalarMapSxV64),
		protoreflect.Uint64Kind: mapArch(getMapSxI[uint64], parseScalarMapSxV64),
		protoreflect.Sint64Kind: mapArch(getMapSxI[int64], parseScalarMapSxZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapSxI[uint32], parseScalarMapSxF32),
		protoreflect.Sfixed32Kind: mapArch(getMapSxI[int32], parseScalarMapSxF32),
		protoreflect.FloatKind:    mapArch(getMapSxI[float32], parseScalarMapSxF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapSxI[uint64], parseScalarMapSxF64),
		protoreflect.Sfixed64Kind: mapArch(getMapSxI[int64], parseScalarMapSxF64),
		protoreflect.DoubleKind:   mapArch(getMapSxI[float64], parseScalarMapSxF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapSxI[bool], parseScalarMapSxV1),
		protoreflect.EnumKind: mapArch(getMapSxI[protoreflect.EnumNumber], parseScalarMapSxV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapSxS, parseScalarMapSxS),
		protoreflect.BytesKind:  mapArch(getMapSxB, parseScalarMapSxB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},

	proto2StringKind: {
		// 32-bit varint types.
		protoreflect.Int32Kind:  mapArch(getMapSxI[int32], parseScalarMapBxV32),
		protoreflect.Uint32Kind: mapArch(getMapSxI[uint32], parseScalarMapBxV32),
		protoreflect.Sint32Kind: mapArch(getMapSxI[int32], parseScalarMapBxZ32),

		// 64-bit varint types.
		protoreflect.Int64Kind:  mapArch(getMapSxI[int64], parseScalarMapBxV64),
		protoreflect.Uint64Kind: mapArch(getMapSxI[uint64], parseScalarMapBxV64),
		protoreflect.Sint64Kind: mapArch(getMapSxI[int64], parseScalarMapBxZ64),

		// 32-bit fixed types.
		protoreflect.Fixed32Kind:  mapArch(getMapSxI[uint32], parseScalarMapBxF32),
		protoreflect.Sfixed32Kind: mapArch(getMapSxI[int32], parseScalarMapBxF32),
		protoreflect.FloatKind:    mapArch(getMapSxI[float32], parseScalarMapBxF32),

		// 64-bit fixed types.
		protoreflect.Fixed64Kind:  mapArch(getMapSxI[uint64], parseScalarMapBxF64),
		protoreflect.Sfixed64Kind: mapArch(getMapSxI[int64], parseScalarMapBxF64),
		protoreflect.DoubleKind:   mapArch(getMapSxI[float64], parseScalarMapBxF64),

		// Special scalar types.
		protoreflect.BoolKind: mapArch(getMapSxI[bool], parseScalarMapBxV1),
		protoreflect.EnumKind: mapArch(getMapSxI[protoreflect.EnumNumber], parseScalarMapBxV32),

		// String types.
		protoreflect.StringKind: mapArch(getMapSxS, parseScalarMapBxS),
		protoreflect.BytesKind:  mapArch(getMapSxB, parseScalarMapBxB),

		// Message types.
		protoreflect.MessageKind: {
			// Not implemented.
		},
		protoreflect.GroupKind: {
			// Not implemented.
		},
	},
}

func init() {
	// Generate each of the entries for proto2StringKind by making copies of
	// the string archetype and using the bytes archetype's parser.
	for _, archs := range mapFields {
		arch := *archs[protoreflect.StringKind]
		arch.parsers = archs[protoreflect.BytesKind].parsers
		archs[proto2StringKind] = &arch
	}
}

// mapArch is a helper for constructing map<K, V> archetypes, where K is not
// bool.
func mapArch(getter getterThunk, parser parserThunk) *archetype {
	return &archetype{
		size: uint32(unsafe2.PointerSize), align: uint32(unsafe2.PointerAlign),
		getter:  getter,
		parsers: []parseKind{{kind: protowire.BytesType, retry: true, parser: parser}},
	}
}

// mapItem is a type usable in any of the map parsers. This is essentially a
// shim for pushing slight custom behavior modifications to each of the stencils
// of e.g. [parseScalarMap].
type mapItem[V any] interface {
	// The wire type for this item.
	kind() protowire.Type

	// Parses a value of this item type and returns it.
	parse(parser1, parser2) (parser1, parser2, V)

	// Returns the key extraction function used with swiss.Table.Insert.
	extract(parser1, parser2) func(V) []byte
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
func (boolItem) extract(parser1, parser2) func(uint8) []byte     { return nil }
func (stringItem) extract(p1 parser1, _ parser2) func(uint64) []byte {
	src := p1.c().src
	return func(u uint64) []byte {
		return zc(u).bytes(src)
	}
}

func (bytesItem) extract(p1 parser1, _ parser2) func(uint64) []byte {
	src := p1.c().src
	return func(u uint64) []byte {
		return zc(u).bytes(src)
	}
}

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

//fastpb:stencil parseScalarMapBxV32 parseScalarMap[bytesItem, varintItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapBxV64 parseScalarMap[bytesItem, varintItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapBxZ32 parseScalarMap[bytesItem, zigzagItem[uint32], uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapBxZ64 parseScalarMap[bytesItem, zigzagItem[uint64], uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapBxF32 parseScalarMap[bytesItem, fixed32Item, uint64, uint32] Init -> swiss.InitU64xU32 Insert -> swiss.InsertU64xU32
//fastpb:stencil parseScalarMapBxF64 parseScalarMap[bytesItem, fixed64Item, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapBxV1  parseScalarMap[bytesItem, boolItem, uint64, uint8] Init -> swiss.InitU64xU8 Insert -> swiss.InsertU64xU8
//fastpb:stencil parseScalarMapBxS   parseScalarMap[bytesItem, stringItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64
//fastpb:stencil parseScalarMapBxB   parseScalarMap[bytesItem, bytesItem, uint64, uint64] Init -> swiss.InitU64xU64 Insert -> swiss.InsertU64xU64

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
			if m < 0 {
				p1.fail(p2, -errCode(m))
			}
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
