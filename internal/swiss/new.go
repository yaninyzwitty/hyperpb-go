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

package swiss

import (
	"unsafe"

	"github.com/bufbuild/fastpb/internal/unsafe2"
)

// Entry is an entry for building a table with [New].
type Entry[K, V any] struct {
	Key   K
	Value V
}

// KV constructs a new entry.
//
// This exists for type inference.
func KV[K, V any](k K, v V) Entry[K, V] { return Entry[K, V]{k, v} }

// New builds a table for the given entries, which will be appended to out.
//
// V must not contain pointers.
func New[K Key, V any](out []byte, extract func(K) []byte, entries ...Entry[K, V]) ([]byte, *Table[K, V]) {
	size, align := Layout[K, V](len(entries))
	_, padding := unsafe2.Misalign(unsafe.SliceData(out), align)

	skip := len(out)
	out = append(out, make([]byte, padding+size)...)
	p := unsafe2.Add(unsafe.SliceData(out), skip)
	t := unsafe2.Cast[Table[K, V]](p)
	t.Init(len(entries), nil, nil)

	// Load up the entries in the map.
	for _, e := range entries {
		*t.Insert(e.Key, extract) = e.Value
	}

	// if dbg.Enabled && reflect.TypeOf(*new(V)).Comparable() {
	// 	// Perform a self-test to make sure that everything is ok.
	// 	var failed bool
	// 	for _, e := range entries {
	// 		var v *V
	// 		if extract == nil {
	// 			v = t.Lookup(e.Key)
	// 		} else {
	// 			v = t.LookupFunc(extract(e.Key), extract)
	// 		}

	// 		t.log("self test", "%v: %v == %v", e.Key, e.Value, v)
	// 		if v == nil || any(v) != any(e.Value) {
	// 			t.log("self test failed", "key: %v", e.Key)
	// 		}
	// 	}

	// 	if failed {
	// 		panic("self-test failed")
	// 	}
	// }

	t.log("new", "%v", t.Dump())

	return out, t
}
