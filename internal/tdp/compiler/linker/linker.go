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

// Package linker provides a general-purpose in-memory linker, that is used to
// assemble the output buffer within the compiler.
package linker

import (
	"errors"
	"fmt"
	"iter"
	"math"

	"buf.build/go/hyperpb/internal/debug"
	"buf.build/go/hyperpb/internal/xunsafe"
	"buf.build/go/hyperpb/internal/xunsafe/layout"
)

// Linker implements a primitive linker, for writing symbols to an output buffer
// and resolving relocations at the very end.
//
// A zero value is ready to use.
type Linker struct {
	symbols  []*Sym       // Symbols in order they are added to the linker.
	database map[any]*Sym // Map of names to symbols.
}

// NewSymbol allocates a new symbol. Symbols are laid out in the order in
// which they are added to the linker.
func (l *Linker) NewSymbol(name any) *Sym {
	s := &Sym{name: name}
	l.symbols = append(l.symbols, s)

	if l.database == nil {
		l.database = make(map[any]*Sym)
	}
	if _, ok := l.database[name]; ok {
		panic(fmt.Sprintf("hyperpb: symbol defined twice: %#v", name))
	}
	l.database[name] = s

	return s
}

// Symbols returns an iterator over all symbols in l with the given name type.
//
// Returns the names of the symbols and, if this is called after [Linker.Link],
// their final offsets.
func Symbols[Name any](l *Linker) iter.Seq2[Name, int] {
	return func(yield func(Name, int) bool) {
		for _, sym := range l.symbols {
			name, ok := sym.name.(Name)
			if ok && !yield(name, sym.offset) {
				return
			}
		}
	}
}

// Link executes the final link and returns the result.
//
// alloc is used to obtain a suitable buffer for the size of the linked program.
func (l *Linker) Link(alloc func(size, align int) []byte) ([]byte, error) {
	// First, figure out the total size of the program.
	offset := 0
	align := 1
	for _, sym := range l.symbols {
		align = max(align, sym.align)
		offset = layout.RoundUp(offset, sym.align)
		sym.offset = offset // Record the start offset, *after* the padding!
		debug.Log(nil, "symbol", "%s @ %#x", sym.name, sym.offset)
		offset += len(sym.data)
	}

	if offset > math.MaxInt32 {
		return nil, errors.New("type has too many dependencies")
	}

	// Get a buffer big enough for what we need.
	buf := alloc(offset, align)

	// Copy over each symbol, resolving relocations as we go.
	offset = 0
	for _, sym := range l.symbols {
		offset = layout.RoundUp(offset, sym.align)
		copy(buf[offset:], sym.data)

		for i, rel := range sym.rels {
			// Find the referenced symbol.
			ref, ok := l.database[rel.Symbol]
			if !ok {
				return nil, fmt.Errorf("undefined symbol: %v", rel.Symbol)
			}

			// Find the location we need to write to in this symbol.
			target := &buf[offset+int(rel.Offset)]

			switch rel.Kind {
			case Address:
				value := &buf[ref.offset]
				xunsafe.ByteStore(target, 0, value)

				debug.Log(nil, "rel:address", "%#v/%d %p->%p", rel.Symbol, i, target, value)

			case Abs32:
				value := uint32(ref.offset)
				xunsafe.ByteStore(target, 0, value)

				debug.Log(nil, "rel:abs32", "%#v/%d %p->%#x", rel.Symbol, i, target, value)

			default:
				return nil, fmt.Errorf("invalid relocation kind: %v", rel.Kind)
			}
		}

		offset += len(sym.data)
	}

	return buf, nil
}
