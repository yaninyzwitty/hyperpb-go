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

package layout_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

func TestAlign(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 8, layout.RoundUp(8, 8))
	assert.Equal(t, 16, layout.RoundUp(9, 8))
	assert.Equal(t, 16, layout.RoundUp(10, 8))
	assert.Equal(t, 16, layout.RoundUp(11, 8))
	assert.Equal(t, 16, layout.RoundUp(12, 8))
	assert.Equal(t, 16, layout.RoundUp(13, 8))
	assert.Equal(t, 16, layout.RoundUp(14, 8))
	assert.Equal(t, 16, layout.RoundUp(15, 8))
	assert.Equal(t, 16, layout.RoundUp(16, 8))

	assert.Equal(t, 0, layout.Padding(8, 8))
	assert.Equal(t, 7, layout.Padding(9, 8))
	assert.Equal(t, 6, layout.Padding(10, 8))
	assert.Equal(t, 5, layout.Padding(11, 8))
	assert.Equal(t, 4, layout.Padding(12, 8))
	assert.Equal(t, 3, layout.Padding(13, 8))
	assert.Equal(t, 2, layout.Padding(14, 8))
	assert.Equal(t, 1, layout.Padding(15, 8))
	assert.Equal(t, 0, layout.Padding(16, 8))
}
