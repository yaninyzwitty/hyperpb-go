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

package swiss

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCtrl(t *testing.T) {
	t.Parallel()

	a := ctrl{x0: 0x0123456789abcdef}
	b := broadcast(0x67)
	c := a.matches(b)
	t.Log(a, b, c)

	for i := range 8 {
		var set bool
		c, set = c.next()
		assert.Equal(t, i == 4, set)
	}
}
