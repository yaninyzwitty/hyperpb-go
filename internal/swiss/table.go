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

// Package swiss provides arena-friendly swisstable implementations.
package swiss

import (
	"bytes"
	"fmt"
	"iter"
	"math"
	"math/bits"
	"math/rand/v2"
	"strings"
	"unsafe"

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

//go:generate go run ../stencil

const maxEntries = math.MaxInt32 / 8

// Key is one of the allowed keys for [Table].
type Key interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int |
		~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint |
		~uintptr
}

// Table is a swisstable.
type Table[K Key, V any] struct {
	// There is a soft and a hard cap. The soft cap is how many elements can be
	// inserted before the table needs to be rehashed, while hard is the actual
	// allocated limit.
	len, soft, hard uint32

	// We can't use the address of a table as the seed, because the compiler
	// wants to be able to copy tables byte-wise in memory.
	seed fxhash

	// ctrl   [cap/8]ctrl
	// keys   [cap]K
	// values [cap]V
}

// Layout calculates the layout for a table with the given types.
//
// len is the desired number of elements.
func Layout[K Key, V any](len int) (size, align int) {
	if len > maxEntries {
		panic(fmt.Sprintf("tdp/internal/table: cannot create table of length %d; max is %d", len, maxEntries))
	}

	var t Table[K, V]
	var k K
	var v V

	_, cap := loadFactor(len)

	size = int(unsafe.Sizeof(t))
	size += int(cap) // Control bytes.

	if diff := unsafe.Alignof(ctrl(0)) - unsafe.Alignof(k); diff > 0 {
		size += int(diff)
	}
	size += int(unsafe.Sizeof(k)) * int(cap)

	if diff := unsafe.Alignof(v) - unsafe.Alignof(k); diff > 0 {
		size += int(diff)
	}
	size += int(unsafe.Sizeof(v)) * int(cap)

	size = (size + 7) &^ 7 // Round up to a multiple of 8.
	return size, 8
}

// Init initializes data allocated per [Layout]'s requirements, optionally
// copying values out of the given table. This function assumes that from is
// smaller. extract is an optional function for extracting variable-length key
// material from keys in the table.
//
// data is assumed to point zeroed memory.
func (t *Table[K, V]) Init(len int, from *Table[K, V], extract func(K) []byte) *Table[K, V] {
	t.soft, t.hard = loadFactor(len)
	t.seed = fxhash(rand.Uint64())
	// empty is chosen to be zero so that we do not need to initialize the
	// control bytes.

	if from == nil || from.len == 0 {
		return t
	}

	ctrl1 := from.ctrl()
	keys1 := from.keys()
	vals1 := from.values()

	ctrl2 := unsafe2.Cast[unsafe2.VLA[byte]](t.ctrl())
	keys2 := t.keys()
	vals2 := t.values()

	// The if is hoisted to avoid an extra branch comparison inside of the hot
	// loop.
	if extract == nil {
		for i := 0; ; i++ {
			dbg.Assert(i < int(t.hard/8), "infinite loop during copy")

			ctrl := *ctrl1.Get(i)
			for j := range 8 {
				if ctrl&0xff == empty {
					continue
				}
				ctrl >>= 8

				n := i*8 + j
				k := *keys1.Get(n)
				h := t.seed.u64(zext(k))
				idx, occupied := t.search(h, k)
				dbg.Assert(!occupied, "fwo keys mapped to one slot")

				*ctrl2.Get(idx) = h.h2()
				*keys2.Get(idx) = *keys1.Get(n)
				*vals2.Get(idx) = *vals1.Get(n)
				t.len++

				if t.len == from.len {
					return t
				}
			}
		}
	} else {
		for i := 0; ; i++ {
			dbg.Assert(i < int(t.hard/8), "infinite loop during copy")

			ctrl := *ctrl1.Get(i)
			for j := range 8 {
				if ctrl&0xff == empty {
					continue
				}
				ctrl >>= 8

				n := i*8 + j
				k := extract(*keys1.Get(n))
				h := t.seed.bytes(k)
				idx, occupied := t.searchFunc(h, k, extract)
				dbg.Assert(!occupied, "fwo keys mapped to one slot")

				*ctrl2.Get(idx) = h.h2()
				*keys2.Get(idx) = *keys1.Get(n)
				*vals2.Get(idx) = *vals1.Get(n)
				t.len++

				if t.len == from.len {
					return t
				}
			}
		}
	}
}

// Len returns this table's length.
func (t *Table[K, V]) Len() int {
	return int(t.len)
}

// Lookup looks up the given key and returns it, or nil if no such key.
func (t *Table[K, V]) Lookup(k K) *V {
	h := t.seed.u64(zext(k))
	idx, occupied := t.search(h, k)
	if !occupied {
		return nil
	}
	return t.values().Get(idx)
}

// LookupFunc us like Lookup, but it takes an optional function for extracting
// variable-length key material from keys in the table.
func (t *Table[K, V]) LookupFunc(k []byte, extract func(K) []byte) *V {
	h := t.seed.bytes(k)
	idx, occupied := t.searchFunc(h, k, extract)
	if !occupied {
		return nil
	}
	return t.values().Get(idx)
}

// insert returns a pointer to the slot for the given key. extract is an
// optional function for extracting variable-length key material from keys
// in the table.
//
// Returns nil if the table would grow too large.
func (t *Table[K, V]) Insert(k K, extract func(K) []byte) *V {
	if t.len == t.soft {
		return nil // Tell the caller to reallocate.
	}

	var idx int
	var occupied bool
	var h fxhash
	if extract == nil {
		h = t.seed.u64(zext(k))
		idx, occupied = t.search(h, k)
	} else {
		k := extract(k)
		h = t.seed.bytes(k)
		idx, occupied = t.searchFunc(h, k, extract)
	}
	if !occupied {
		ctrl := unsafe2.Cast[unsafe2.VLA[byte]](t.ctrl())
		*ctrl.Get(idx) = h.h2()
		*t.keys().Get(idx) = k
		t.len++
	}
	return t.values().Get(idx)
}

// All ranges over a table.
func (t *Table[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		if t.len == 0 {
			return
		}

		ctrl := t.ctrl()
		keys := t.keys()
		vals := t.values()

		len := t.len
		for i := range int(t.hard) / 8 {
			ctrl := *ctrl.Get(i)
			for j := range 8 {
				if ctrl&0xff == empty {
					continue
				}
				ctrl >>= 8

				k := i*8 + j
				len--
				if !yield(*keys.Get(k), *vals.Get(k)) || len == 0 {
					return
				}
			}
		}
	}
}

// search searches for a key's bucket: either an occupied slot, or an empty
// slot where it could be inserted at.
//
// Returns the index of the bucket and whether it is already occupied.
func (t *Table[K, V]) search(h fxhash, k K) (idx int, occupied bool) {
	t.log("search", "h: %v, k: %v", h, k)

	h2 := broadcast(h.h2())
	empty := broadcast(empty)

	p := t.probe(h)
	keys := t.keys()
	for {
		// Guaranteed to terminate because there's always going to be an open
		// spot to insert. We include a debug assert for catching when this
		// fails to happen.
		dbg.Assert(p.i <= p.mask, "full table")

		var i int
		var ctrl ctrl
		p, i, ctrl = p.next()

		// First, check for any hits.
		mask := ctrl.matches(h2)
		t.log("matching", "i: %v, ctrl: %v, h2: %v, mask: %v", i, ctrl, h2, mask)
		if mask != 0 {
			n := i * 8
			for j := range 8 {
				var eq bool
				mask, eq = mask.next()
				if eq {
					k2 := *keys.Get(n)
					t.log("checking", "%v == %v", k, k2)
					if k == k2 {
						t.log("found occupied", "%v,%v = %v", i, j, n)
						return n, true
					}
				}
				n++
			}
		}

		// Otherwise, check for empties.
		j := ctrl.first(empty)
		if j < 8 {
			n := i*8 + j
			t.log("found vacant", "%v,%v = %v", i, j, n)
			return n, false
		}
	}
}

// searchFunc is like search, but takes a function for extracting a variable-length key.
func (t *Table[K, V]) searchFunc(h fxhash, k []byte, extract func(K) []byte) (idx int, occupied bool) {
	h2 := broadcast(h.h2())
	empty := broadcast(empty)

	p := t.probe(h)
	keys := t.keys()
	for {
		// Guaranteed to terminate because there's always going to be an open
		// spot to insert. We include a debug assert for catching when this
		// fails to happen.
		dbg.Assert(p.i <= p.mask, "full table")

		var i int
		var ctrl ctrl
		p, i, ctrl = p.next()

		// First, check for any hits.
		mask := ctrl.matches(h2)
		if mask != 0 {
			n := i * 8
			for j := range 8 {
				var eq bool
				mask, eq = mask.next()
				if eq {
					k2 := extract(*keys.Get(n))
					t.log("checking", "%x == %x", k, k2)
					if bytes.Equal(k, k2) {
						t.log("found occupied", "%v,%v = %v", i, j, n)
						return n, true
					}
				}
				n++
			}
		}

		// Otherwise, check for empties.
		j := ctrl.first(empty)
		if j < 8 {
			n := i*8 + j
			t.log("found vacant", "%v,%v = %v", i, j, n)
			return n, false
		}
	}
}

func (t *Table[K, V]) ctrl() *unsafe2.VLA[ctrl] {
	return unsafe2.Beyond[ctrl](t)
}

func (t *Table[K, V]) keys() *unsafe2.VLA[K] {
	ctrl := unsafe2.Beyond[ctrl](t)
	last := ctrl.Get(int(t.hard/8) - 1)
	return unsafe2.Beyond[K](last)
}

func (t *Table[K, V]) values() *unsafe2.VLA[V] {
	ctrl := unsafe2.Beyond[ctrl](t)
	last := ctrl.Get(int(t.hard/8) - 1)
	keys := unsafe2.Beyond[K](last)
	last2 := keys.Get(int(t.hard) - 1)
	return unsafe2.Beyond[V](last2)
}

func (t *Table[K, V]) probe(hash fxhash) prober {
	return newProber(t.ctrl(), int(t.hard)/8, hash)
}

func (t *Table[K, V]) log(op, format string, args ...any) {
	dbg.Log([]any{"%p", t}, op, format, args...)
}

// loadFactor calculates the capacity of a table with n elements, implementing
// a load factor of 7/8.
//
// The returned value is always a power of two divisible by 8.
func loadFactor(len int) (soft, hard uint32) {
	if len < 8 {
		len = 7
	}

	// Go generates better code for unsigned arithmetic here.
	e := uint(len)
	n := e * 8 / 7
	// Make sure that n is a power of two. Pick the next power of
	// two after n.
	if bits.OnesCount(n) != 1 {
		n = uint(1) << bits.Len(n)
	}
	return uint32(n / 8 * 7), uint32(n)
}

// Format implements [fmt.Formatter].
func (t *Table[K, V]) Format(s fmt.State, verb rune) {
	kv := "%v/%#x: " + fmt.FormatString(s, verb)
	first := true

	fmt.Fprint(s, "[")
	for k, v := range t.All() {
		if !first {
			fmt.Fprint(s, ", ")
		}
		first = false
		fmt.Fprintf(s, kv, k, k, v)
	}
	fmt.Fprint(s, "]")
}

// Dump returns an in-memory dump of t.
func (t *Table[K, V]) Dump() string {
	var k K
	var v V

	buf := new(strings.Builder)
	size, align := Layout[K, V](int(t.len))
	end := unsafe2.Addr[byte](unsafe2.AddrOf(t)).Add(size)
	fmt.Fprintf(buf, "%p:%v: Table[%T, %T]\n", t, end, k, v)
	fmt.Fprintf(buf, "len: %v, cap: %v/%v, load factor: %v\n", t.len, t.soft, t.hard, float64(t.len)/float64(t.hard))
	fmt.Fprintf(buf, "seed: %016x, layout: [%d:%d]\n", uint64(t.seed), size, align)

	fmt.Fprintf(buf, "ctrl:")
	ctrl := unsafe2.Cast[unsafe2.VLA[byte]](t.ctrl())
	for i := range int(t.hard) {
		switch i % 16 {
		case 0:
			fmt.Fprintf(buf, "\n  %p:%04d", ctrl.Get(i), i)
		case 8:
			fmt.Fprint(buf, " ")
		}
		fmt.Fprintf(buf, " %02x", *ctrl.Get(i))
	}
	fmt.Fprintln(buf)

	fmt.Fprintf(buf, "keys:")
	keys := t.keys()
	perLine := max(1, int(16/unsafe.Sizeof(k)))
	for i := range int(t.hard) {
		if i%perLine == 0 {
			fmt.Fprintf(buf, "\n  %p/%04d:", keys.Get(i), i)
		}
		fmt.Fprintf(buf, " %x", unsafe2.AnyBytes(*keys.Get(i)))
	}
	fmt.Fprintln(buf)

	fmt.Fprintf(buf, "values:")
	values := t.values()
	perLine = max(1, int(16/unsafe.Sizeof(v)))
	for i := range int(t.hard) {
		if i%perLine == 0 {
			fmt.Fprintf(buf, "\n  %p/%04d:", values.Get(i), i)
		}
		fmt.Fprintf(buf, " %x", unsafe2.AnyBytes(*values.Get(i)))
	}
	fmt.Fprintln(buf)

	return buf.String()
}

//fastpb:stencil InitU8xU8 Table.Init[uint8, uint8] search -> searchU8xU8 searchFunc -> searchFuncU8xU8
//fastpb:stencil InitU32xU8 Table.Init[uint32, uint8] search -> searchU32xU8 searchFunc -> searchFuncU32xU8
//fastpb:stencil InitU64xU8 Table.Init[uint64, uint8] search -> searchU64xU8 searchFunc -> searchFuncU64xU8
//fastpb:stencil InitU8xU32 Table.Init[uint8, uint32] search -> searchU8xU32 searchFunc -> searchFuncU8xU32
//fastpb:stencil InitU32xU32 Table.Init[uint32, uint32] search -> searchU32xU32 searchFunc -> searchFuncU32xU32
//fastpb:stencil InitU64xU32 Table.Init[uint64, uint32] search -> searchU64xU32 searchFunc -> searchFuncU64xU32
//fastpb:stencil InitU8xU64 Table.Init[uint8, uint64] search -> searchU8xU64 searchFunc -> searchFuncU8xU64
//fastpb:stencil InitU32xU64 Table.Init[uint32, uint64] search -> searchU32xU64 searchFunc -> searchFuncU32xU64
//fastpb:stencil InitU64xU64 Table.Init[uint64, uint64] search -> searchU64xU64 searchFunc -> searchFuncU64xU64

//fastpb:stencil InsertU8xU8 Table.Insert[uint8, uint8] search -> searchU8xU8 searchFunc -> searchFuncU8xU8
//fastpb:stencil InsertU32xU8 Table.Insert[uint32, uint8] search -> searchU32xU8 searchFunc -> searchFuncU32xU8
//fastpb:stencil InsertU64xU8 Table.Insert[uint64, uint8] search -> searchU64xU8 searchFunc -> searchFuncU64xU8
//fastpb:stencil InsertU8xU32 Table.Insert[uint8, uint32] search -> searchU8xU32 searchFunc -> searchFuncU8xU32
//fastpb:stencil InsertU32xU32 Table.Insert[uint32, uint32] search -> searchU32xU32 searchFunc -> searchFuncU32xU32
//fastpb:stencil InsertU64xU32 Table.Insert[uint64, uint32] search -> searchU64xU32 searchFunc -> searchFuncU64xU32
//fastpb:stencil InsertU8xU64 Table.Insert[uint8, uint64] search -> searchU8xU64 searchFunc -> searchFuncU8xU64
//fastpb:stencil InsertU32xU64 Table.Insert[uint32, uint64] search -> searchU32xU64 searchFunc -> searchFuncU32xU64
//fastpb:stencil InsertU64xU64 Table.Insert[uint64, uint64] search -> searchU64xU64 searchFunc -> searchFuncU64xU64

//fastpb:stencil searchU8xU8 Table.search[uint8, uint8]
//fastpb:stencil searchU32xU8 Table.search[uint32, uint8]
//fastpb:stencil searchU64xU8 Table.search[uint64, uint8]
//fastpb:stencil searchU8xU32 Table.search[uint8, uint32]
//fastpb:stencil searchU32xU32 Table.search[uint32, uint32]
//fastpb:stencil searchU64xU32 Table.search[uint64, uint32]
//fastpb:stencil searchU8xU64 Table.search[uint8, uint64]
//fastpb:stencil searchU32xU64 Table.search[uint32, uint64]
//fastpb:stencil searchU64xU64 Table.search[uint64, uint64]

//fastpb:stencil searchFuncU8xU8 Table.searchFunc[uint8, uint8]
//fastpb:stencil searchFuncU32xU8 Table.searchFunc[uint32, uint8]
//fastpb:stencil searchFuncU64xU8 Table.searchFunc[uint64, uint8]
//fastpb:stencil searchFuncU8xU32 Table.searchFunc[uint8, uint32]
//fastpb:stencil searchFuncU32xU32 Table.searchFunc[uint32, uint32]
//fastpb:stencil searchFuncU64xU32 Table.searchFunc[uint64, uint32]
//fastpb:stencil searchFuncU8xU64 Table.searchFunc[uint8, uint64]
//fastpb:stencil searchFuncU32xU64 Table.searchFunc[uint32, uint64]
//fastpb:stencil searchFuncU64xU64 Table.searchFunc[uint64, uint64]

//fastpb:stencil LookupI32xU32 Table.Lookup[int32, uint32] search -> searchI32xU32
//fastpb:stencil searchI32xU32 Table.search[int32, uint32]
