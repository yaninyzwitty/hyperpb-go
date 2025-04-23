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

package unsafe2_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bufbuild/fastpb/internal/unsafe2"
)

func TestMisalign(t *testing.T) {
	t.Parallel()

	type A = unsafe2.Addr[byte]

	prev, next := A(0).Misalign(8)
	assert.Equal(t, 0, prev)
	assert.Equal(t, 0, next)

	prev, next = A(1).Misalign(8)
	assert.Equal(t, 1, prev)
	assert.Equal(t, 7, next)
	prev, next = A(3).Misalign(8)
	assert.Equal(t, 3, prev)
	assert.Equal(t, 5, next)
	prev, next = A(4).Misalign(8)
	assert.Equal(t, 4, prev)
	assert.Equal(t, 4, next)
	prev, next = A(7).Misalign(8)
	assert.Equal(t, 7, prev)
	assert.Equal(t, 1, next)
	prev, next = A(8).Misalign(8)
	assert.Equal(t, 0, prev)
	assert.Equal(t, 0, next)
}

func TestPC(t *testing.T) {
	t.Parallel()

	f := func() int { return 42 }
	pc := unsafe2.NewPC(f)

	fmt.Printf("%#x\n", pc)
	assert.Equal(t, 42, pc.Get()())
}
