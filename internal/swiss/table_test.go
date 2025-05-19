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

package swiss_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/swiss"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

//nolint:tparallel // The tests are intentionally serialized.
func TestTable(t *testing.T) {
	t.Parallel()
	arena := new(arena.Arena)

	size, _ := swiss.Layout[int32, int32](0)
	m := unsafe2.Cast[swiss.Table[int32, int32]](arena.Alloc(size))
	m.Init(0, nil, nil)
	for k := range int32(1000) {
		defer dbg.WithTesting(t)()

		t.Log(m.Dump())
		v := m.Insert(k, nil)
		if v == nil {
			size, _ := swiss.Layout[int32, int32](m.Len() + 1)
			m2 := unsafe2.Cast[swiss.Table[int32, int32]](arena.Alloc(size))
			m2.Init(m.Len()+1, m, nil)
			m = m2
			v = m.Insert(k, nil)
		}
		*v = -k

		ok := t.Run(strconv.Itoa(int(k)), func(t *testing.T) {
			defer dbg.WithTesting(t)()

			for k := range k + 1 {
				v := -k
				require.Equal(t, m.Lookup(k), &v, "%v: %v", k, v)
			}
		})

		if !ok {
			t.FailNow()
		}
	}
}
