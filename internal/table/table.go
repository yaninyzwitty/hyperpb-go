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

// Package table provides a simple map implementation specialized for
// immutability, size, arena-friendliness, and 32-bit integer keys.
//
// The map implementation is an open-addressing table using quadratic probing
// and a simple, fxhash-derived hash function.
package table

import (
	"fmt"
	"math"
	"math/bits"
	"unsafe"

	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/unsafe2"
)

const (
	rotate = 5
	key    = 0x517cc1b727220a95

	maxEntries = math.MaxInt32 / 8

	empty = math.MaxInt32
)

// Table is a simple map implementation specialized for
// immutability, size, arena-friendliness, and 32-bit integer keys.
type Table[V any] struct {
	// The data pointer for this table. This is equal to the offset in
	// out at which the table is appended to in New.
	Data *byte
}

// Entry is an entry for building a table with [New].
type Entry[V any] struct {
	// NOTE: the value math.MaxInt32 is reserved for empty slots!
	Key   int32
	Value V
}

// New builds a table for the given entries, which will be appended to out.
//
// V must not contain pointers.
func New[V comparable](out []byte, entries ...Entry[V]) ([]byte, Table[V]) {
	if len(entries) > maxEntries {
		panic(fmt.Sprintf("tdp/internal/table: cannot create table of length %d; max is %d", len(entries), maxEntries))
	}

	buckets := buckets(len(entries))
	size, align := layout[V](buckets)
	_, padding := unsafe2.Misalign(unsafe.SliceData(out), align)

	skip := len(out)
	out = append(out, make([]byte, padding+size)...)
	t := Table[V]{unsafe2.Add(unsafe.SliceData(out), skip)}

	unsafe2.ByteStore(t.Data, 0, uint32(buckets))
	_, keys, vals := t.unpack()

	// Fill keys with empties.
	for i := range buckets {
		unsafe2.Store(keys, i, empty)
	}

	// Load up the entries in the map.
	for _, e := range entries {
		if e.Key == empty {
			panic(fmt.Sprintf("fastpb/internal/table: cannot use %d as a key", e.Key))
		}

		h := int(fx32(uint32(e.Key)))
		for i := range buckets {
			h = probe(h, i, buckets)
			if unsafe2.Load(keys, h) == empty {
				unsafe2.Store(keys, h, e.Key)
				unsafe2.Store(vals, h, e.Value)
				break
			}
		}
	}

	if dbg.Enabled {
		// Self-test.
		for _, e := range entries {
			v := t.Lookup(e.Key)
			if v == nil || *v != e.Value {
				t.log("self test", "fail: t[%d] was %v, not %v", e.Key, v, &e.Value)
			}
		}
	}

	return out, t
}

// Lookup looks for the given key in a table.
//
// Returns nil if the key is not found.
func (t Table[V]) Lookup(key int32) *V {
	buckets, keys, vals := t.unpack()

	h := int(fx32(uint32(key)))
	t.log("hash", "%d/%#x -> %d", key, key, h)
	for i := range buckets {
		h = probe(h, i, buckets)
		k := unsafe2.Load(keys, h)

		t.log("probe", "%d: %d/%#x", h, k, k)
		switch k {
		case empty:
			return nil
		case key:
			return unsafe2.Add(vals, h)
		}
	}
	return nil
}

// Bytes returns the backing byte array for this table.
func (t Table[V]) Bytes() []byte {
	bytes, _ := layout[V](t.buckets())
	return unsafe.Slice(t.Data, bytes)
}

// Format implements [fmt.Formatter].
func (t Table[V]) Format(s fmt.State, verb rune) {
	buckets, keys, vals := t.unpack()

	kv := "%v/%#x: " + fmt.FormatString(s, verb)
	first := true

	fmt.Fprint(s, "[")
	for i := range buckets {
		k := unsafe2.Load(keys, i)
		if k == empty {
			continue
		}

		if !first {
			fmt.Fprint(s, ", ")
		}
		first = false
		fmt.Fprintf(s, kv, k, k, unsafe2.Load(vals, i))
	}
	fmt.Fprint(s, "]")
}

// buckets returns the number of buckets in this table.
func (t Table[V]) buckets() int {
	return int(unsafe2.ByteLoad[uint32](t.Data, 0))
}

// unpack extracts the key and value pointers from a byte array pointing to a
// table with the given number of buckets.
func (t Table[V]) unpack() (int, *int32, *V) {
	buckets := t.buckets()
	data := unsafe2.Add(t.Data, unsafe2.Int32Size)

	_, align := unsafe2.Layout[V]()
	bytes := buckets * unsafe2.Int32Size
	_, padding := unsafe2.Addr[byte](bytes).Misalign(align)

	keys := unsafe2.Cast[int32](data)
	vals := unsafe2.Cast[V](unsafe2.ByteAdd(unsafe2.Add(keys, buckets), padding))

	t.log("unpack", "%d %p/%p", buckets, keys, vals)
	return buckets, keys, vals
}

// probe implements quadratic probing using triangular numbers. Calling This
// function with the a value will produce the next value in the sequence.
//
// buckets must be a power of 2.
func probe(prev, i, buckets int) int {
	// We evaluate f(i) = (i^2 + i)/2 mod buckets recursively, noting that for
	// j = i+1,
	//
	//  f(j) = (j^2 + j)/2
	//       = (i^2 + 2i + 1 + i + 1)/2
	//       = (i^2 + i)/2 + (2i+2)/2
	//       = f(i) + i + 1
	//       = f(i) + j
	return (prev + i) & (buckets - 1)
}

// buckets returns the size of a [Table] with the given number of entries.
//
// This essentially calculates an adjusted size with a given load factor.
func buckets(entries int) int {
	// Set up buckets so that entries = buckets * 7/8, for a load factor of
	// ~0.88.

	// Go generates better code for unsigned arithmetic here.
	e := uint(entries)
	n := e * 8 / 7

	// Make sure that n is a power of two. Pick the next power of
	// two after n.
	if bits.OnesCount(n) != 1 {
		n = uint(1) << bits.Len(n)
	}
	return int(n)
}

// layout returns the layout for a table with the given number of buckets.
func layout[V any](buckets int) (size, align int) {
	size, align = unsafe2.Layout[V]()
	align = min(align, unsafe2.Int32Align)

	// One extra "bucket" to hold the bucket count.
	bytes := (buckets + 1) * unsafe2.Int32Size
	_, padding := unsafe2.Addr[byte](bytes).Misalign(align)
	bytes += padding
	bytes += buckets * size

	return bytes, align
}

// fx32 is a variation of fxhash for 32-bit integers.
//
// See https://docs.rs/fxhash
func fx32(n uint32) uint32 {
	// (n <<< 5) ^ -n is a bijection on n, and not zero when n is zero, so
	// the whole hash is unlikely to "zero out".
	//
	// This is admittedly less a hash function and more a single round of a
	// crappy ARX-type block cypher.
	return (bits.RotateLeft32(n, rotate) ^ -n) * uint32(key&math.MaxUint32)
}

func (t Table[V]) log(op, format string, args ...any) {
	dbg.Log([]any{"%p", t.Data}, op, format, args...)
}
