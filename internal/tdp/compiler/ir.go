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

package compiler

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/bufbuild/fastpb/internal/debug"
	"github.com/bufbuild/fastpb/internal/scc"
	"github.com/bufbuild/fastpb/internal/stats"
	"github.com/bufbuild/fastpb/internal/tdp"
	"github.com/bufbuild/fastpb/internal/tdp/dynamic"
	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/unsafe2/layout"
)

// ir is analysis information about a message type for generating a parser
// and a dynamic type for it.
type ir struct {
	d protoreflect.MessageDescriptor

	// Each Protobuf field has three associated pieces of data that can be
	// sorted in different orders. There is the field inside of a [Type],
	// the field's parsers (which there may be more than one of per tField),
	// and the field's struct offsets (which may be shared by t).
	t []tField
	p []pField
	s []sField

	hot, cold int
	layout    tdp.TypeLayout
}

type tField struct {
	d      protoreflect.FieldDescriptor
	prof   FieldProfile
	arch   *Archetype
	offset tdp.Offset
}

type pField struct {
	tIdx int // Index in ir.t.
	aIdx int // Index in ir.t[tIdx].arch.parsers.

	hot  bool // If true, this parser should be in the "hot" part of the stream.
	next int  // The next parser to execute, as an index into ir.p.
}

type sField struct {
	tIdx []int // Index in ir.t. May be more than one!

	layout layout.Layout
	bits   uint32
	offset tdp.Offset
	hot    bool
}

// sccInfo is information associated with a particular strongly connected
// component of messages.
type sccInfo struct {
	// Whether any message in this component has a required field, or a
	// submessage that transitively contains one.
	//
	// Note that every message in a component contains every other message
	// therein as a transitive submessage, so it is sufficient to just check for
	// required fields across the whole component.
	//
	// Later, when this gets written into a tdp.Type, we will use this
	// information to determine which fields in a message can contain required
	// fields.
	hasRequired bool
}

// newIR generates an intermediate representation for a given message.
func newIR(c *compiler, md protoreflect.MessageDescriptor) *ir {
	ir := &ir{d: md}

	// Classify all of the fields into archetypes.
	for _, fd := range c.fields(md) {
		prof := c.profile(fd)
		arch := c.Backend.SelectArchetype(fd, prof)

		if arch.Bits > 0 && arch.Oneof {
			panic(fmt.Sprintf("oneof archetype for %v requested bits; this is a bug", fd.FullName()))
		}
		ir.t = append(ir.t, tField{
			d:    fd,
			prof: prof,
			arch: arch,
		})
	}

	return ir
}

// this component, and that they are stored in c.sccInfo.
func newSCCInfo(c *compiler, component *scc.Component[*ir]) *sccInfo {
	info := new(sccInfo)

	// Add contributions from dependencies.
	for dep := range component.Deps() {
		info.hasRequired = info.hasRequired || c.sccInfo[dep].hasRequired
	}

	// Add contributions from component members.
	for _, ir := range component.Members() {
		for _, t := range ir.t {
			info.hasRequired = info.hasRequired || t.d.Cardinality() == protoreflect.Required
		}
	}

	return info
}

// doLayout computes the layout information for the type this IR represents.
func (ir *ir) doLayout(c *compiler) {
	for tIdx, t := range ir.t {
		// Protoc will always place oneof members contiguously in the fields
		// array of a message. This means that if this is not the first member
		// of a oneof, the most recent value in ir.s will be the current oneof's
		// struct slot.
		if t.arch.Oneof &&
			t.d.ContainingOneof().Fields().Get(0).Index() != t.d.Index() {
			last := &ir.s[len(ir.s)-1]
			last.tIdx = append(last.tIdx, tIdx)
			continue
		}

		ir.s = append(ir.s, sField{tIdx: []int{tIdx}})
	}

	// Next, lay out the struct by sorting the struct members by alignment.
	var bits, whichWords int
	for i := range ir.s {
		sf := &ir.s[i]
		var temp stats.Mean
		for _, j := range sf.tIdx {
			arch := ir.t[j].arch
			sf.layout = sf.layout.Max(arch.Layout)
			sf.bits = max(sf.bits, arch.Bits)

			temp.Record(ir.t[j].prof.DecodeProbability)
		}

		bits += int(sf.bits)
		sf.hot = temp.Get() >= 0

		if ir.t[sf.tIdx[0]].arch.Oneof {
			whichWords++
		}
	}

	// Append hidden zero-size fields at the end to ensure that the stride of
	// this type is divisible by 8.
	ir.s = append(ir.s, sField{layout: layout.Of[[0]uint64](), hot: true})
	ir.s = append(ir.s, sField{layout: layout.Of[[0]uint64](), hot: false})

	slices.SortStableFunc(ir.s, func(a, b sField) int {
		// Sort hot fields before cold fields. This simplifies the code below.
		switch {
		case a.hot == b.hot:
			return -cmp.Compare(a.layout.Align, b.layout.Align)
		case a.hot:
			return -1
		default:
			return 1
		}
	})

	// Figure out the number of bit words we need. We use 32-bit words.
	const bitsPerWord = 32
	bitWords := (bits + bitsPerWord - 1) / bitsPerWord // Divide and round up.
	ir.layout.BitWords = bitWords + whichWords

	ir.hot = layout.Size[dynamic.Message]()
	ir.hot += (bitWords + whichWords) * 4

	ir.cold = layout.Size[dynamic.Cold]()

	var nextBit uint32
	nextWhichWord := uint32(ir.hot - whichWords*4)
	for i := range ir.s {
		sf := &ir.s[i]
		if sf.layout.Align == 0 {
			continue
		}

		// Allocate bit and byte storage for this field.
		size := &ir.hot
		if !sf.hot {
			size = &ir.cold
		}

		_, up := unsafe2.Addr[byte](*size).Misalign(sf.layout.Align)
		*size += up
		if debug.Enabled && up > 0 {
			// Note alignment padding required for the previous field.
			if i == 0 && sf.hot {
				ir.layout.BitWords += up / 4
			} else if ir.s[i-1].hot == sf.hot {
				f := ir.layout.Fields
				f[len(f)-1].Padding = uint32(up)
			}
		}

		sf.offset.Data = int32(*size)
		if !sf.hot {
			sf.offset.Data = ^sf.offset.Data
		}
		*size += sf.layout.Size

		if sf.bits > 0 {
			sf.offset.Bit = nextBit
			nextBit += sf.bits
		}

		oneof := sf.tIdx != nil && ir.t[sf.tIdx[0]].arch.Oneof
		if oneof {
			sf.offset.Bit = nextWhichWord
			nextWhichWord += 4
		}

		// Copy the offset information into each field that uses this struct
		// slot.
		for _, j := range sf.tIdx {
			ir.t[j].offset = sf.offset
			if oneof {
				ir.t[j].offset.Number = uint32(ir.t[j].d.Number())
			}
		}

		if debug.Enabled && sf.tIdx != nil {
			index := sf.tIdx[0]
			if ir.t[index].arch.Oneof {
				index = ^ir.t[index].d.ContainingOneof().Index()
			}

			ir.layout.Fields = append(ir.layout.Fields, tdp.FieldLayout{
				Size:   uint32(sf.layout.Size),
				Align:  uint32(sf.layout.Align),
				Bits:   sf.bits,
				Index:  index,
				Offset: sf.offset,
			})
		}
	}

	if ir.hot > math.MaxInt32 {
		panic(fmt.Errorf("fastpb: message struct for %v too large (%d bytes, max is %d)", ir.d.FullName(), ir.hot, math.MaxInt32))
	}
	if ir.cold > math.MaxInt32 {
		panic(fmt.Errorf("fastpb: message struct for %v too large (%d bytes, max is %d)", ir.d.FullName(), ir.cold, math.MaxInt32))
	}

	if debug.Enabled {
		// Print the resulting layout for this struct.
		ir.logLayout(c)
	}
}

func (ir *ir) doSchedule(c *compiler) {
	for tIdx, t := range ir.t {
		hot := t.prof.DecodeProbability >= 0.5
		for j := range t.arch.Parsers {
			ir.p = append(ir.p, pField{
				tIdx: tIdx,
				aIdx: j,
				hot:  j == 0 && hot,
			})
		}
	}

	// Now, sort the parsers into the hot and cold sides. Stable sort is
	// particularly important here!
	slices.SortStableFunc(ir.p, func(a, b pField) int {
		var aCold, bCold int
		if !a.hot {
			aCold = 1
		}
		if !b.hot {
			bCold = 1
		}
		return cmp.Compare(aCold, bCold)
	})

	// Now, lay out control flow between parsers. Each parser points to the
	// first one after it that refers to a different field or oneof, except
	// for cold parsers, which always point to a hot parser.
	//
	// For this purpose, we build a table of the index of the first hot parser
	// for each field/oneof. Oneof indices are entered as their complements.
	table := make(map[int]int, len(ir.t))
	idx := func(tIdx int) int {
		tf := ir.t[tIdx]
		if tf.arch.Oneof {
			return ^tf.d.ContainingOneof().Index()
		}
		return tf.d.Index()
	}

	for i, pf := range ir.p {
		if !pf.hot {
			continue
		}

		j := idx(pf.tIdx)
		if _, ok := table[j]; !ok {
			table[j] = i
		}
	}

	for i := range ir.p {
		pf := &ir.p[i]

		p := ir.t[pf.tIdx].arch.Parsers[pf.aIdx]
		if p.Retry {
			pf.next = i
			continue
		}

		orig := idx(pf.tIdx)
	loop:
		for tIdx := pf.tIdx; tIdx < len(ir.t); tIdx++ {
			i := idx(tIdx)
			j, ok := table[i]
			if !ok {
				continue
			}

			// j is the index of *some* hot parser. This may be for the same
			// field/oneof as the current index, so we need to keep incrementing
			// it until it either:
			//
			// 1. Points to a cold parser, and hence it should just wrap around
			//    to the first parser in the stream.
			//
			// 2. We hit a parser for a different field/oneof.
			for ; ; j++ {
				if j == len(ir.p) {
					break loop // Wraparound.
				}
				next := ir.p[j]
				if !next.hot {
					break loop // We reached the cold section.
				}

				if idx(next.tIdx) != orig {
					pf.next = j
					break loop
				}
			}
		}
	}

	if debug.Enabled {
		// Print the parser CFG.
		c.log("cfg", "%s\n%v", ir.d.FullName(), debug.Formatter(func(buf fmt.State) {
			for i, pf := range ir.p {
				tf := ir.t[pf.tIdx]
				fmt.Fprintf(buf, "  #%d: %v#%d -> #%d\n", i, tf.d.Name(), pf.aIdx, pf.next)
			}
		}))
	}
}

func (ir *ir) logLayout(c *compiler) {
	c.log("layout", "%s, %d/%d\n%v", ir.d.FullName(), ir.hot, ir.cold,
		debug.Formatter(func(buf fmt.State) {
			start := layout.Size[dynamic.Message]()
			fmt.Fprintf(buf, "  %#04x(-)[%d:4:0] [%d]uint32\n", start, 4*ir.layout.BitWords, ir.layout.BitWords)
			for _, sf := range ir.s {
				if sf.tIdx == nil {
					continue
				}

				tf := ir.t[sf.tIdx[0]]
				name := tf.d.Name()
				if tf.arch.Oneof {
					name = "oneof:" + tf.d.ContainingOneof().Name()
				}

				fmt.Fprintf(buf, "  %#04x", sf.offset.Data)
				if sf.bits > 0 {
					fmt.Fprintf(buf, "(%v)", sf.offset.Bit)
				} else {
					fmt.Fprint(buf, "(-)")
				}
				fmt.Fprintf(buf, "[%d:%d:%d]", sf.layout.Size, sf.layout.Align, sf.bits)

				fmt.Fprintf(buf, " %s: ", name)
				switch tf.d.Cardinality() {
				case protoreflect.Optional:
					if tf.d.HasOptionalKeyword() {
						fmt.Fprint(buf, "optional ")
					}
				case protoreflect.Repeated:
					fmt.Fprint(buf, "repeated ")
				case protoreflect.Required:
					fmt.Fprint(buf, "required ")
				}
				if m := tf.d.Message(); m != nil {
					fmt.Fprintf(buf, "%v (%v) ", m.FullName(), tf.d.Kind())
				} else if e := tf.d.Enum(); e != nil {
					fmt.Fprintf(buf, "%v (%v) ", e.FullName(), tf.d.Kind())
				} else {
					fmt.Fprintf(buf, "%v ", tf.d.Kind())
				}
				fmt.Fprintln(buf)
			}
		}))
}
