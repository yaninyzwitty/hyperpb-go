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

//nolint:tparallel // The tests are intentionally serialized.
package swiss_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bufbuild/hyperpb/internal/arena"
	"github.com/bufbuild/hyperpb/internal/debug"
	"github.com/bufbuild/hyperpb/internal/swiss"
	"github.com/bufbuild/hyperpb/internal/xunsafe"
)

type value struct {
	x int32
}

func TestIntTable(t *testing.T) {
	t.Parallel()
	defer debug.WithTesting(t)()
	arena := new(arena.Arena)

	size, _ := swiss.Layout[int32, value](0)
	m := xunsafe.Cast[swiss.Table[int32, value]](arena.Alloc(size))
	m.Init(0, nil, nil)
	for k := range int32(1000) {
		t.Log(m.Dump())
		v := m.Insert(k, nil)
		if v == nil {
			size, _ := swiss.Layout[int32, value](m.Len() + 1)
			m2 := xunsafe.Cast[swiss.Table[int32, value]](arena.Alloc(size))
			m2.Init(m.Len()+1, m, nil)
			m = m2
			v = m.Insert(k, nil)
		}
		*v = value{-k}

		ok := t.Run(strconv.Itoa(int(k)), func(t *testing.T) {
			defer debug.WithTesting(t)()

			for k := range k + 1 {
				p := m.Lookup(k)
				v := value{-k}
				require.Equal(t, p, &v, "%v: %v; got %v", k, v, p)
			}
		})

		if !ok {
			t.FailNow()
		}
	}
}

func TestStringTable(t *testing.T) {
	t.Parallel()
	defer debug.WithTesting(t)()
	arena := new(arena.Arena)
	extract := func(n uint32) []byte {
		return xunsafe.StringToSlice[[]byte](urlSlice[n])
	}

	size, _ := swiss.Layout[uint32, value](0)
	m := xunsafe.Cast[swiss.Table[uint32, value]](arena.Alloc(size))
	m.Init(0, nil, nil)
	for k := range uint32(1000) {
		t.Log(m.Dump())
		v := m.Insert(k, extract)
		if v == nil {
			size, _ := swiss.Layout[uint32, value](m.Len() + 1)
			m2 := xunsafe.Cast[swiss.Table[uint32, value]](arena.Alloc(size))
			m2.Init(m.Len()+1, m, extract)
			m = m2
			v = m.Insert(k, extract)
		}
		*v = value{-int32(k)}

		ok := t.Run(strconv.Itoa(int(k)), func(t *testing.T) {
			defer debug.WithTesting(t)()

			for k := range k + 1 {
				p := m.LookupFunc(extract(k), extract)
				v := value{-int32(k)}
				require.Equal(t, p, &v, "%q: %v; got %v", extract(k), v, p)
			}
		})

		if !ok {
			t.FailNow()
		}
	}
}
