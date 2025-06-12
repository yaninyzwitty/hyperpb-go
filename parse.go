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

package fastpb

import (
	"fmt"
	"math"
	"math/bits"
	"runtime"
	"strings"
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/bufbuild/fastpb/internal/arena"
	"github.com/bufbuild/fastpb/internal/dbg"
	"github.com/bufbuild/fastpb/internal/swiss"
	"github.com/bufbuild/fastpb/internal/sync2"
	"github.com/bufbuild/fastpb/internal/unsafe2"
	"github.com/bufbuild/fastpb/internal/zc"
)

const (
	defaultMaxMisses = 4
	defaultMaxDepth  = 1000
	signBits         = 0x80_80_80_80_80_80_80_80

	// 29 total bits.
	maxFieldTag = 0b00001111_01111111_01111111_01111111_01111111
)

var (
	stackPool = sync2.Pool[[]parserFrame]{}
	ppPool    = sync2.Pool[parserP]{
		Reset: func(pp *parserP) { *pp = parserP{} },
	}
)

// CompileOption is a configuration setting for [Type.Unmarshal].
type UnmarshalOption func(*parserP)

// MaxDecodeMisses sets the number of decode misses allowed in the parser before
// switching to the slow path.
//
// Large values may improve performance for common protos, but introduce a
// potential DoS vector due to quadratic worst case performance. The default
// is 8.
func MaxDecodeMisses(n int) UnmarshalOption {
	return func(p3 *parserP) { p3.maxMisses = n }
}

// MaxDepth sets the maximum recursion depth for the parser.
//
// Setting a large value enables potential DoS vectors.
func MaxDepth(n int) UnmarshalOption {
	return func(p3 *parserP) { p3.maxDepth = min(n, math.MaxUint32) }
}

// startParse is the top-level entry point for message parsing.
func startParse(m *message, data []byte, options ...UnmarshalOption) (err error) {
	if m.context.src != nil {
		panic("fastpb: attempted to parse message using in-use Context")
	}

	if len(data) > math.MaxUint32 {
		return &errParse{code: errCodeTooBig}
	}

	m.context.lock.Lock()

	pp := ppPool.Get()
	pp.maxMisses = defaultMaxMisses
	pp.maxDepth = defaultMaxDepth

	for _, opt := range options {
		if opt != nil {
			opt(pp)
		}
	}

	m.context.src = ensure9BytesPastEnd(data, false)
	m.context.len = len(data)
	// The arena keeps m.context alive, so we don't need to KeepAlive src.

	if len(data) == 0 {
		return nil
	}

	stack := stackPool.Get()
	if cap(*stack) < pp.maxDepth {
		*stack = make([]parserFrame, pp.maxDepth)
	}

	pp.stack.top = unsafe2.AddrOf(unsafe.SliceData(*stack))
	pp.stack.bottom = pp.stack.top.Add(pp.maxDepth)

	pp.stack.ptr = pp.stack.bottom

	defer func() {
		if pp.err.code != 0 && recover() != nil {
			// Make a copy of the error, since pp will get re-used by a future
			// run of this function.
			parseErr := pp.err
			err = &parseErr

			if dbg.Enabled {
				buf := new(strings.Builder)
				for _, frame := range pp.stackSlice() {
					fmt.Fprintf(buf, "- %#v\n", frame)
				}

				dbg.Log(nil, "fail",
					"%v\n"+
						"trace to fail() call:\n%s"+
						"stack:\n%s", err, dbg.Stack(6), buf)
			}
		}

		// These would all normally go in their own defers, but having a single
		// defer is noticeably faster.
		stackPool.Put(stack)
		ppPool.Put(pp)
		m.context.lock.Unlock()
	}()

	p1 := parser1{
		c_: unsafe2.AddrOf(m.context),
		b_: unsafe2.AddrOf(m.context.src),
	}
	p2 := parser2{
		pp_: unsafe2.AddrOf(pp),
	}

	p1, p2 = p1.message(p2, len(data), m)
	p2.scratch = 0
	loop(p1, p2)

	return err
}

// ensure9BytesPastEnd ensures that it is always possible to read nine bytes
// beyond the end of data. This allows us to elide virtually all bounds checks
// in the parser, since it will only ever look ahead at most nine bytes (to
// parse a rare ten-byte varint).
//
// This function accomplishes this by checking that loading nine bytes from the
// end of data does not cross a 4K page boundary. If it does not, it means that
// we can always load past the end a little bit, because page protection is not
// more granular than that on any platform we care about. If this condition is
// not met, we copy the slice in such a way as to force this condition to be
// met.
//
// If forceCopy is set, this copy is performed unconditionally.
func ensure9BytesPastEnd(data []byte, forceCopy bool) *byte {
	end := unsafe2.AddrOf(unsafe.SliceData(data))
	end += unsafe2.Addr[byte](cap(data))

	_, up := end.Misalign(0x1000)
	if up >= 9 && !forceCopy {
		// All good, we have nine or more bytes ahead of us before the next
		// page boundary.
		return unsafe.SliceData(data)
	}

	// Copy to a new slice with just enough capacity.
	data = append(make([]byte, 0, len(data)+9), data...)
	return unsafe.SliceData(data)
}

// parser1 is half of the state for the TDP parser.
//
// This struct must no more than four fields, and all four fields must be
// word-sized or smaller, so that it fits in registers AND does not trigger
// go.dev/issue/72897.
//
// For this reason, the parser state is split into two structs that will fit
// in registers and will not be spilled. This means that functions with the
// [parseFunc] signature will keep all of the parser data in registers with
// minimal spillage. Ideally this would all be in a single struct, but see the
// above bug.
//
// Moreover, these structs should contain no pointers; pointers have instead
// been replaced with addresses, all of which are rooted at the call to
// startParse. This avoids unnecessary spilling for GC stack scanning, since
// those pointers are already findable elsewhere.
//
// Generic parser functions are homed under parser1, with a parser2 argument,
// such that these functions have the following signature:
//
// func(parser1, parser2) (parser1, parser2)
//
// Some functions do not have the signature because they are guaranteed inline
// candidates.
//
// Note that returning no values is slower than returning the parser state: this
// is because it will force the caller to spill the parser state across the
// call.
type parser1 struct {
	c_ unsafe2.Addr[Context]
	b_ unsafe2.Addr[byte]
	e_ unsafe2.Addr[byte] // One past the end of the stream.
	g_ uint64             // End-of-group tag.
}

// parser2 is the other half of the state for the TDP parser. See [parser1].
type parser2 struct {
	m_  unsafe2.Addr[message]
	f_  unsafe2.Addr[fieldParser]
	pp_ unsafe2.Addr[parserP]

	// A scratch register that is preserved across *most* calls. Thunks
	// do not preserve the scratch register, and some functions in this file
	// do not either.
	scratch uint64
}

// parserP is parser state that is passed behind a pointer.
type parserP struct {
	_ unsafe2.NoCopy

	err   errParse
	stack struct {
		ptr         unsafe2.Addr[parserFrame]
		top, bottom unsafe2.Addr[parserFrame]
	}

	t_ unsafe2.Addr[typeParser]

	maxMisses, maxDepth int
}

// parserFrame is a recursion frame for the parser.
type parserFrame struct {
	e unsafe2.Addr[byte]
	g uint64
	m unsafe2.Addr[message]
	t unsafe2.Addr[typeParser]
	f unsafe2.Addr[fieldParser]
}

func (p1 parser1) c() *Context {
	return p1.c_.AssertValid()
}

func (p1 parser1) arena() *arena.Arena {
	return &p1.c_.AssertValid().arena
}

func (p1 parser1) b() *byte {
	// There is an exciting bug that can occur where we dereference p1.b_
	// while it points to the end of the input slice. Being able to do have
	// p1.b_ equal the one-past-the-end spot is nice, but if we dereference it,
	// Go may scan through this pointer, and mark the allocation it points to.
	// If it happens to point to freed memory, the GC panics, because this is
	// an unrecoverable constraint violation.
	//
	// This assert makes sure that none of our large test suite accidentally
	// performs this illegal maneuver.
	//
	// Annoyingly this means we also need to be careful in parser1.buf(),
	// because we cannot form a zero-sized slice to the end of an allocation.
	dbg.Assert(p1.b_ < p1.e_,
		"p1.b_ cannot point one past the end: need %v < %v", p1.b_, p1.e_)
	return p1.b_.AssertValid()
}

func (p2 parser2) m() *message {
	return p2.m_.AssertValid()
}

func (p2 parser2) t() *typeParser {
	return p2.pp().t_.AssertValid()
}

func (p2 parser2) f() *fieldParser {
	return p2.f_.AssertValid()
}

func (p2 parser2) pp() *parserP {
	return p2.pp_.AssertValid()
}

func (p1 parser1) len() int {
	return int(p1.e_ - p1.b_)
}

// fail causes a parse failure by panicking with the given error code.
func (p1 parser1) fail(p2 parser2, err errCode) {
	p2.pp().err = errParse{
		code:   err,
		offset: p1.b_.Sub(unsafe2.AddrOf(p1.c().src)),
	}

	_ = *(*byte)(nil) // Trigger a panic without calling runtime.gopanic. Linters hate this!
}

func (pp *parserP) stackSlice() []parserFrame {
	n := pp.stack.bottom.Sub(pp.stack.ptr)
	return unsafe.Slice(pp.stack.ptr.AssertValid(), n)
}

// push pushes a parser frame.
//
//go:nosplit
func (p1 parser1) push(p2 parser2, end unsafe2.Addr[byte]) (parser1, parser2) {
	if dbg.Enabled {
		p1, p2 = logPush(p1, p2)
	}

	if p2.pp().stack.ptr == p2.pp().stack.top {
		p1.fail(p2, errCodeRecursionDepth)
	}

	p2.pp().stack.ptr = p2.pp().stack.ptr.Add(-1)

	// Note: a single frame is just too large to hit Go's SROA pass (same bug
	// that results in p1/p2 being two structs). Thus, we write each field
	// separately to avoid wasteful stack traffic.
	frame := p2.pp().stack.ptr.AssertValid()
	frame.e = p1.e_
	frame.g = p1.g_
	frame.m = p2.m_
	frame.t = p2.pp().t_
	frame.f = p2.f_

	p1.e_ = end
	return p1, p2
}

// Outlined so that push() does not hit the stack size limit for nosplit.
//
//go:noinline
func logPush(p1 parser1, p2 parser2) (parser1, parser2) {
	p1.log(p2, "push", "%v/%v/%v", p2.pp().stack.top, p2.pp().stack.ptr, p2.pp().stack.bottom)
	return p1, p2
}

// pop pops a parser frame.
//
// Returns whether the last frame was popped.
//
//go:nosplit
func (p1 parser1) pop(p2 parser2) (parser1, parser2, bool) {
	if dbg.Enabled {
		s := &p2.pp().stack
		p1.log(p2, "pop", "%v/%v/%v\n%s", s.top, s.ptr, s.bottom,
			p2.m().dump())
	}

	last := p2.pp().stack.ptr.AssertValid()
	p1.e_ = last.e
	p1.g_ = last.g
	p2.m_ = last.m
	p2.pp().t_ = last.t
	p2.f_ = last.f
	p2.pp().stack.ptr = p2.pp().stack.ptr.Add(1)

	return p1, p2, p2.pp().stack.ptr == p2.pp().stack.bottom
}

// atLeast fails the parse if there aren't at least n bytes left to parse.
//
//go:nosplit
func (p1 parser1) atLeast(p2 parser2, n uint64) (parser1, parser2) {
	if n <= uint64(p1.len()) {
		return p1, p2
	}

	p1.fail(p2, errCodeTruncated)
	return p1, p2
}

// buf returns the data left to parse.
func (p1 parser1) buf() []byte {
	if p1.len() == 0 {
		return nil
	}
	return unsafe2.Slice(p1.b(), p1.len())
}

func (p1 parser1) advance(n int) parser1 {
	p1.b_ = p1.b_.Add(n)
	return p1
}

//go:nosplit
func (p1 parser1) varint(p2 parser2) (parser1, parser2, uint64) {
	if dbg.Enabled {
		// Force this function to behave as if it is not nosplit in debug mode,
		// so that we don't overflow the nosplit stack when we turn on
		// debugging.
		return p1.varintYesSplit(p2)
	}

	return p1.varintNoSplit(p2)
}

//go:noinline
func (p1 parser1) varintYesSplit(p2 parser2) (parser1, parser2, uint64) {
	return p1.varintNoSplit(p2)
}

//go:nosplit
func (p1 parser1) varintNoSplit(p2 parser2) (parser1, parser2, uint64) {
	// Inlined from protowire.ConsumeVarint to minimize spills and remove
	// bounds checks.
	var b byte
	var x uint64
	var i int
	p := p1.b()

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if int8(b) >= 0 {
		goto exit
	}
	x -= 0x80 << (i * 7)
	i++

	b = *p
	p = unsafe2.Add(p, 1)
	x |= uint64(b) << (i * 7)
	if b <= 1 {
		goto exit
	}

	p1.fail(p2, errCodeOverflow)

exit:
	if dbg.Enabled {
		len := int(unsafe2.AddrOf(p) - p1.b_) // For debug only.
		p1.log(p2, "varint", "%d:%#x (%d bytes)", x, x, len)
	}

	p1.b_ = unsafe2.AddrOf(p)
	if p1.len() < 0 {
		p1.fail(p2, errCodeTruncated)
	}

	return p1, p2, x
}

// fixed32 parses a 32-bit fixed-width integer.
func (p1 parser1) fixed32(p2 parser2) (parser1, parser2, uint32) {
	p1, p2 = p1.atLeast(p2, 4)
	x := unsafe2.ByteLoad[uint32](p1.b(), 0)
	p1 = p1.advance(4)

	p1.log(p2, "fixed32", "%d:%#x (%d bytes)", x, x, 4)
	return p1, p2, x
}

// fixed32 parses a 64-bit fixed-width integer.
func (p1 parser1) fixed64(p2 parser2) (parser1, parser2, uint64) {
	p1, p2 = p1.atLeast(p2, 8)
	x := unsafe2.ByteLoad[uint64](p1.b(), 0)
	p1 = p1.advance(8)

	p1.log(p2, "fixed64", "%d:%#x (%d bytes)", x, x, 8)
	return p1, p2, x
}

// lengthPrefix parses a varint up to the current length.
//
//go:nosplit
func (p1 parser1) lengthPrefix(p2 parser2) (parser1, parser2, int) {
	var n uint64
	p1, p2, n = p1.varint(p2)

	{
		// Explicit inlining of atLeast(). len() is guaranteed to fit in a
		// uint32.
		if n <= uint64(p1.len()) {
			return p1, p2, int(n)
		}

		p1.fail(p2, errCodeTruncated)
	}

	return p1, p2, int(n)
}

// bytes parses a length-delimited byte buffer.
func (p1 parser1) bytes(p2 parser2) (parser1, parser2, zc.Range) {
	var n int
	p1, p2, n = p1.lengthPrefix(p2)

	r := zc.NewRaw(p1.b_.Sub(unsafe2.AddrOf(p1.c().src)), n)
	p1 = p1.advance(n)

	if dbg.Enabled {
		text := r.Bytes(p1.c().src)
		p1.log(p2, "bytes", "%#v, %q", r, text)
	}
	return p1, p2, r
}

// utf8 parses a length-delimited byte buffer, and validates it for UTF8.
func (p1 parser1) utf8(p2 parser2) (parser1, parser2, zc.Range) {
	var n int
	p1, p2, n = p1.lengthPrefix(p2)

	e := p1.b_.Add(n)

	// All non-spatial errors are accumulated so we only have to do one branch
	// at the end.
	ok := true
	for p := p1.b_; p < e; {
		n := min(8, e-p)
		// Fast path for ASCII: simply check that all of the bytes don't have
		// their sign bits set.
		bytes := *unsafe2.Cast[uint64](p.AssertValid())
		mask := uint64(signBits) >> ((8 - n) * 8)
		ascii := bits.TrailingZeros64(bytes&mask) / 8
		p1.log(p2, "ascii bytes", "%016x, %d bytes", bytes, ascii)
		p = p.Add(ascii)

		if ascii == 8 {
			continue
		}

		// Need to parse a multi-byte rune.
		// The possible encodings are like this:
		//
		// 110xxxxx 10xxxxxx
		// 1110xxxx 10xxxxxx 10xxxxxx
		// 11110xxx 10xxxxxx 10xxxxxx 10xxxxxx
		//
		// We can use LeadingZeros8 to find which of these cases we're in.

		first := *p.AssertValid()
		count := uint(bits.LeadingZeros8(^first)) // Total # of bytes.
		p1.log(p2, "wide rune", "%#08b, %d bytes", first, count)
		if count-2 > 2 || uint(e-p) < count {
			// In the above, counts of 0, 1, 2, 3, 4, 5, 6, 7, and 8 map
			// to -2, -1, 0, 1, 2, 3, 4, 5, 6. All of these except 0, 1, and
			// 2 compare as >2, since count is unsigned.
			goto fail
		}

		// Bounds check is complete here. We are free to load four bytes
		// and mask off what we don't need. We can't re-use bytes here
		// because the rune might straddle a boundary.
		raw := *unsafe2.Cast[uint32](p.AssertValid())
		p1.log(p2, "wide rune bits", "%08b, %d bytes", unsafe2.Bytes(&raw), count)

		// This puts the contents of the first byte into r.
		r := rune(raw & ((1 << (8 - count)) - 1))

		i := count - 1
	decode:
		r <<= 6
		raw >>= 8
		r |= rune(raw & 0b111111)

		if raw&0b11_000000 != 0b10_000000 {
			ok = false
		}

		i--
		if i > 0 {
			goto decode
		}

		p1.log(p2, "decoded rune", "U+%04x (%q)", r, r)

		// Next we check that the length is correct. To do this, we need
		// to map the following ranges like so (note that we don't need to
		// worry about ASCII, as above):
		//
		// U+0080..U+07FF -> 2
		// U+0800..U+FFFF -> 3
		// U+FFFF...      -> 4
		//
		// bits.Len / 4 will map each of these to ranges as follows:
		//
		// U+0080..U+07FF -> 8..11 -> 2
		// U+0800..U+FFFF -> 12..16 -> 3..4
		// U+10000...     -> 17...  -> 4
		//
		// This is almost correct. We just need to subtract 1 in the case
		// that bits.Len returns 16.
		wantCount := bits.Len32(uint32(r))
		if wantCount == 16 {
			wantCount--
		}
		wantCount /= 4

		p1.log(p2, "rune size", "want: %d, got: %d", wantCount, count)
		if wantCount != int(count) {
			ok = false
		}

		// Finally, we can check that this rune is in the valid range.
		if r&^0x7ff == 0xd800 { // This checks for the surrogate range.
			ok = false
		}
		if r > 0x10ffff {
			ok = false
		}

		p = p.Add(int(count))
	}

	if ok {
		r := zc.NewRaw(p1.b_.Sub(unsafe2.AddrOf(p1.c().src)), n)
		p1 = p1.advance(n)

		if dbg.Enabled {
			text := r.Bytes(p1.c().src)
			p1.log(p2, "utf8", "%#v, %q", r, text)
		}
		return p1, p2, r
	}

fail:
	p1.fail(p2, errCodeUTF8)
	return p1, p2, 0
}

// message pushes a new message to be parsed onto the parser stack.
//
//go:nosplit
func (p1 parser1) message(p2 parser2, len int, m *message) (parser1, parser2) {
	if len == 0 {
		return p1, p2
	}

	// Preserve this register across the call to push.
	p2.scratch = uint64(unsafe2.AddrOf(m))

	if p1.b_.Add(len) != p1.e_ {
		// We don't need to push a new frame if the new message would cause
		// the current frame to be empty once it gets popped.
		p1, p2 = p1.push(p2, p1.b_.Add(len))
	}

	p1.g_ = ^uint64(0)
	p2.m_ = unsafe2.Addr[message](p2.scratch)
	p2.pp().t_ = unsafe2.AddrOf(p2.m().ty().raw.parser)
	if dbg.Enabled {
		p1, p2 = logMessage(p1, p2)
	}

	p2.f_ = unsafe2.AddrOf(&p2.pp().t_.AssertValid().entry)

	return p1, p2
}

// message pushes a new map entry to be parsed onto the parser stack.
//
//go:nosplit
func (p1 parser1) mapEntry(p2 parser2, len int, m *message) (parser1, parser2) {
	if len == 0 {
		return p1, p2
	}

	// Preserve this register across the call to push.
	p2.scratch = uint64(unsafe2.AddrOf(m))

	if p1.b_.Add(len) != p1.e_ {
		// We don't need to push a new frame if the new message would cause
		// the current frame to be empty once it gets popped.
		p1, p2 = p1.push(p2, p1.b_.Add(len))
	}

	p1.g_ = ^uint64(0)
	p2.m_ = unsafe2.Addr[message](p2.scratch)
	p2.pp().t_ = unsafe2.AddrOf(p2.m().ty().raw.parser.mapEntry)
	if dbg.Enabled {
		p1, p2 = logMessage(p1, p2)
	}

	p2.f_ = unsafe2.AddrOf(&p2.pp().t_.AssertValid().entry)

	return p1, p2
}

// Outlined so that push() does not hit the stack size limit for nosplit.
//
//go:noinline
func logMessage(p1 parser1, p2 parser2) (parser1, parser2) {
	p1.log(
		p2, "new", "%#x, %v, %v",
		p2.m_,
		p2.m().ty(),
		p2.m().ty().raw,
	)
	p1.log(p2, "tags", "%v%v\n", p2.t().tags.Dump(), p2.t().tags)
	return p1, p2
}

// loop is the core parser loop. This function is not recursive.
func loop(p1 parser1, p2 parser2) {
	// Need this to match the ABI of returning from a thunk.
	p2.f_ = unsafe2.AddrOf(p2.f().nextOk)

	// This code is dynamically unreachable, but it forces Go to schedule
	// the fail block slightly differently in a way that is more in our favor
	// for branch scheduling.
	if p2.scratch != 0 {
		goto truncated
	}

checkDone:
	if p1.len() == 0 {
		goto pop
	}

number:
	{
		var tag fieldTag
		// The purpose of this block is to decode a varint without actually doing
		// any of the shifts to delete the sign bits. Instead:
		//
		// 1. Load n := 8 bytes from p1.b. Machinery elsewhere ensures this load
		//    will not segfault.
		//
		// 2. Determine how many bytes are in this varint using ctz(^n & K), where
		//    K has all of its sign bits set. This is the number of bit places up to
		//    the first cleared sign bit; it is always equal to 7 mod 8 unless no
		//    sign bits are present.
		//
		//    To ensure we can subtract off 7, we want to clear the highest sign bit
		//    of n. If it is set, which is a rare case, then we need to check for
		//    potential overflow in the next eight bytes.
		//
		//    This ensures that ctz(^n & K) is (8 - n) * 8 + 7, where n is the
		//    number of sign bits set in the word, up to 7.
		//
		// 3. We mask off all bytes past the first byte without a sign bit.
		//
		// 4. We set all of the sign bits to zero.
		//
		// This means that if a varint is over-long encoded, all of the extra
		// bytes turn into zeros. For example, if we have 0xaabbccddeeff0081
		// (litte-endian), we get a value of 15 for ctz(^n & K), so there are 6
		// irrelevant bytes past the 00. We mask those off and get 0x0081, and after
		// removing sign bits, we get 0x0001, which is the minimal encoding.

		// This block cannot be outlined into its own function for performance
		// reasons.

		// Load up to eight bytes for the varint (at most 5 will be used).
		tag = unsafe2.ByteLoad[fieldTag](p1.b(), 0)
		// Flip all of the sign bits. This essentially clears the sign bits
		// of all of the varint bytes except the highest one's.
		tag ^= signBits

		// Determine the number of cleared sign bits. This will tell us how
		// many bits to mask off as "irrelevant".
		//
		// In a varint (big-endian order) like 0a8b8c8d, this will be looking
		// at ctz(80000000) = 31. Thus we need to mask off 64 - 31 = 33 bits.
		tagBits := uint(bits.TrailingZeros64(uint64(tag) & signBits))

		// tagMask will have its first (bits+1) bytes set to 0xff. Here, we shift
		// 0x100 to save on an add instruction on bytes.
		// The &63 is to ensure that Go does not generate a cmov to implement
		// the x<<64 == 0 case.
		tag &= (fieldTag(0b10) << ((tagBits - 1) & 63)) - 1
		// No need to strip the sign bits, the ^= above already did that.

		// Consume the tag.
		tagBytes := (tagBits / 8) + 1
		p1.log(p2, "number", "%v (%d bytes)", tag, tagBytes)
		if tagBytes > uint(p1.len()) {
			goto truncated
		}
		p1 = p1.advance(int(tagBytes))

		p2.scratch = uint64(tag)
		if tagBits < 64 {
			goto field
		}

		p1, p2 = p1.checkLargeVarint(p2)
	}

field:
	{
		tries := p2.pp().maxMisses
		tag := fieldTag(p2.scratch)
		for {
			p1.log(p2, "try", "%v, %v, %v", tag, tries, p2.f())

			if dbg.Enabled {
				// Run the GC frequently in debug mode to smoke out bugs where
				// we've left a stack root unmarked.
				runtime.GC()
			}

			if p2.f().tag == tag {
				// Try to keep the Context in L1 cache by loading a byte from it
				// before every thunk. This makes sure that short thunks that
				// do not allocate any memory do not cause it to fall out of
				// the cache, slowing down memory allocations due to the need
				// to pull the arena's internal pointers from L2 cache.
				unsafe2.Ping(p1.c())

				thunk := &p2.f().thunk
				p1.log(p2, "call", "%v, %#x", dbg.Func(thunk.Get()), p2.f_)
				p1, p2 = thunk.Get()(p1, p2)
				p1.log(p2, "ret", "%v, %#x", dbg.Func(thunk.Get()), p2.f_)

				p2.f_ = unsafe2.AddrOf(p2.f().nextOk)

				p2.scratch = 0 // Make sure no one relies on this being preserved.
				goto checkDone
			}

			p2.f_ = unsafe2.AddrOf(p2.f().nextErr)

			tries--
			if tries > 0 {
				continue
			}

			break
		}

		p1.log(p2, "miss", "%v", tag)
		// Check for tag overflow.
		if tag > maxFieldTag {
			p1.fail(p2, errCodeOverflow)
		}

		// Finish parsing number into a varint.
		// This is a manual inlining of tag.decode.
		mask := fieldTag(0x7f)
		i := 0
		// Repeated 5 times.
		var tag2 uint64
		tag2 |= uint64(tag&mask) >> i
		mask <<= 8
		i++
		tag2 |= uint64(tag&mask) >> i
		mask <<= 8
		i++
		tag2 |= uint64(tag&mask) >> i
		mask <<= 8
		i++
		tag2 |= uint64(tag&mask) >> i
		mask <<= 8
		i++
		tag2 |= uint64(tag&mask) >> i
		mask <<= 8
		i++
		// Repeat end.
		p1.log(p2, "decode number", "%d", tag2)
		_, _ = i, mask

		// Check if we know about this field number.
		p1, p2, tag2 = p1.byTag(p2, tag2)
		if p2.f() != nil {
			p1.log(p2, "goto field", "%d", tag2)
			goto field
		}

		// Skip this field, and keep skipping fields until we find a field
		// number we recognize.
		for {
			p1, p2 = p1.unknown(p2, tag2)
			if p1.len() == 0 {
				goto pop
			}

			p2.scratch = uint64(p1.b_)
			p1, p2, tag2 = p1.varint(p2)
			if tag2 > math.MaxInt32<<3 {
				p1.fail(p2, errCodeOverflow)
			}

			p1, p2, tag2 = p1.byTag(p2, tag2)
			if p2.f() != nil {
				p1.b_ = unsafe2.Addr[byte](p2.scratch)
				p1.log(p2, "goto number", "%d", tag2)
				goto number
			}
		}
	}

pop:
	{
		if dbg.Enabled {
			p1.log(
				p2, "finish", "%v, ty: %p:%s %v",
				p2.m_,
				p2.m().ty().raw,
				p2.m().ty().Descriptor().FullName(),
				p2.m().ty().raw,
			)
		}

		var done bool
		p1, p2, done = p1.pop(p2)
		if done {
			return
		}

		// Only need to pop once: message() makes sure to avoid creating multiple
		// frames with the same end pointer.

		goto number
	}

truncated:
	// Route all failures in loop() here to force Go to schedule them as the
	// cold side of the branch leading to it.
	p1.fail(p2, errCodeTruncated)
}

func (p1 parser1) byTag(p2 parser2, tag2 uint64) (parser1, parser2, uint64) {
	t := p2.t()
	p := swiss.LookupI32xU32(t.tags, int32(tag2))
	if p == nil {
		p2.f_ = 0
		return p1, p2, tag2
	}
	p2.f_ = unsafe2.AddrOf(t.fields().Get(int(*p)))
	return p1, p2, tag2
}

//go:noinline
func (p1 parser1) unknown(p2 parser2, tag2 uint64) (parser1, parser2) {
	if tag2&^0xffffffff != 0 {
		p1.fail(p2, errCodeOverflow)
	}

	// Rewind the stream to find the start offset of this field. We can do this
	// because we know that tag2 is nonzero, so first we can trim off leading
	// zero bytes for an over-long varint, and then skip back the minimum
	// number of bytes needed to store tag2.
	start := p1.b_
	start--
	for *start.AssertValid()&0x7f == 0 {
		start--
	}
	start = start.Add(1 - protowire.SizeVarint(tag2))

	num := protowire.Number(tag2 >> 3)
	ty := protowire.Type(tag2 & 0b111)
	if num == 0 {
		p1.fail(p2, errCodeFieldNumber)
	}

	m := protowire.ConsumeFieldValue(num, ty, p1.buf())
	p1.log(p2, "unknown", "%d, %d, %d bytes", num, ty, m)
	if m < 0 {
		p1.fail(p2, errCode(-m))
	}
	p1 = p1.advance(m)

	if !p2.t().discardUnknown {
		r := zc.New(p1.c().src, start.AssertValid(), int(p1.b_-start))
		cold := p2.m().mutableCold()
		if cold.unknown.Len() > 0 {
			last := unsafe2.Add(cold.unknown.Ptr(), cold.unknown.Len()-1)
			if r.Start() == last.End() {
				*last = zc.NewRaw(last.Start(), last.Len()+r.Len())
				return p1, p2
			}
		}
		cold.unknown = cold.unknown.AppendOne(p1.arena(), r)
	}

	return p1, p2
}

//go:noinline
func (p1 parser1) checkLargeVarint(p2 parser2) (parser1, parser2) {
	// This is a very large varint. We need to check the next two words.
	// This is a slow path, so we can afford to not be efficient.
	switch unsafe2.Load(p1.b(), -1) {
	case 0x00:
	case 0x80:
		if unsafe2.Load(p1.b(), 0) != 0x00 {
			p1.fail(p2, errCodeOverflow)
		}
		p1 = p1.advance(1)
	default:
		p1.fail(p2, errCodeOverflow)
	}

	return p1, p2
}

// log logs debugging information during a parse.
func (p1 parser1) log(p2 parser2, op, format string, args ...any) {
	if !dbg.Enabled {
		return
	}

	start := p1.b_.Sub(unsafe2.AddrOf(p1.c().src))
	end := p1.e_.Sub(unsafe2.AddrOf(p1.c().src))
	height := p2.pp().stack.bottom.Sub(p2.pp().stack.ptr)
	var b byte
	if p1.b_ < p1.e_ {
		b = *p1.b()
	}
	dbg.Log(
		[]any{
			"%p:%p:%d [%d:%d] = 0x%02x",
			p1.c(), p2.m(), height, start, end, b,
		},
		op, format, args...,
	)
}
