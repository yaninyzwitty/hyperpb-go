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

package zigzag_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/encoding/protowire"

	"buf.build/go/hyperpb/internal/zigzag"
)

func TestZigzag(t *testing.T) {
	t.Parallel()

	tests32 := []int32{
		0, 1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 11, 12, 13, 14, 15,
		0x7fffffff,
		-0x80000000,
		-1, -2, -3, -4, -5, -6, -7, -8,
	}
	tests64 := []int64{
		0, 1, 2, 3, 4, 5, 6, 7,
		8, 9, 10, 11, 12, 13, 14, 15,
		0x7fffffffffffffff,
		-0x8000000000000000,
		-1, -2, -3, -4, -5, -6, -7, -8,
	}

	for _, tt := range tests32 {
		t.Run(fmt.Sprintf("32/%#x", tt), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, int32(protowire.DecodeZigZag(uint64(uint32(tt)))), zigzag.Decode(tt))
		})
	}

	for _, tt := range tests64 {
		t.Run(fmt.Sprintf("64/%#x", tt), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, protowire.DecodeZigZag(uint64(tt)), zigzag.Decode(tt))
		})
	}
}
