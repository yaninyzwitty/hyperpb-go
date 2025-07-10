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

package zigzag

import (
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"

	"buf.build/go/hyperpb/internal/tdp"
)

// Decode decodes a zigzag-encoded value of any type.
//
// Calling DecodeZigZag does not work correctly when sign extension is involved.
func Decode[T tdp.Number](raw T) T {
	n := uint64(raw)
	n &= (1 << (unsafe.Sizeof(raw) * 8)) - 1

	return T(protowire.DecodeZigZag(n))
}

// Decode64 is a helper for calling zigzag with a raw 64-bit input.
func Decode64[T tdp.Number](raw uint64) T {
	return Decode(T(raw))
}
