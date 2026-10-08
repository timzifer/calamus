package png

import (
	"encoding/binary"
	"io"
	"math/bits"

	"github.com/timzifer/calamus/internal/huffman"
)

// FastCompression is calamus's own level, after fpng: every row filtered
// with Up, and deflate matches only one pixel back, found eight bytes at
// a time; prefix codes are built per block from its symbol counts, kept
// on the way, or deflate's fixed codes for small blocks. It is faster
// than BestSpeed on photos and graphics at about their size, and writes
// much larger files of text and line art, whose repeats lie further back
// than a pixel. Files decode with any PNG decoder.
const FastCompression CompressionLevel = -4

// Deflate's lengths and distances (RFC 1951, 3.2.5).
var (
	lengthBase  = [29]uint16{3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 15, 17, 19, 23, 27, 31, 35, 43, 51, 59, 67, 83, 99, 115, 131, 163, 195, 227, 258}
	lengthExtra = [29]uint8{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 5, 5, 0}
	// lengthCode is the length symbol's index (0 for 257) of each length
	// from 3 to 258.
	lengthCode [259]uint8
)

func init() {
	for c := range lengthBase {
		hi := 258
		if c+1 < len(lengthBase) {
			hi = int(lengthBase[c+1]) - 1
		}
		for l := int(lengthBase[c]); l <= hi; l++ {
			lengthCode[l] = uint8(c)
		}
	}
}

// distCode is the distance symbol and its extra bits for distances 1 to
// 8, the most bytes a pixel has.
func distCode(d int) (code, extra, extraBits int) {
	switch {
	case d <= 4:
		return d - 1, 0, 0
	case d <= 6:
		return 4, d - 5, 1
	}
	return 5, d - 7, 1
}

// fastMaxMatch is deflate's longest match.
const fastMaxMatch = 258

// tokens: a literal byte, or a match (matchFlag | length) at the fixed
// distance.
const matchFlag = 1 << 16

// fastMatcher finds matches one pixel back in a stream handed over in
// pieces, keeping the bytes it needs from the previous piece, and counts
// the symbols of the tokens it makes.
type fastMatcher struct {
	dist int
	hist []byte // the last dist bytes before the current piece
	work []byte
	lit  [286]uint32 // literal/length symbol counts since reset
	n    int         // matches since reset
}

func (m *fastMatcher) reset() {
	m.lit = [286]uint32{}
	m.n = 0
}

// tokenize appends the tokens of p to toks. A match ends where p ends;
// the next piece starts matching afresh, with the history.
func (m *fastMatcher) tokenize(p []byte, toks []uint32) []uint32 {
	d := m.dist
	data := append(append(m.work[:0], m.hist...), p...)
	m.work = data
	i := len(m.hist)
	n := len(data)
	for i < n {
		l := 0
		switch {
		case i < d:
		case i+8 <= n:
			// Eight bytes against the eight one pixel back at once.
			x := binary.LittleEndian.Uint64(data[i:]) ^ binary.LittleEndian.Uint64(data[i-d:])
			if x&0xff != 0 {
				// The bytes that differ from the byte one pixel back cannot
				// start a match: literals, as many as there are in a row.
				k := 1
				for k < 8 && byte(x>>(8*k)) != 0 {
					k++
				}
				for _, c := range data[i : i+k] {
					toks = append(toks, uint32(c))
					m.lit[c]++
				}
				i += k
				continue
			}
			if x != 0 {
				l = bits.TrailingZeros64(x) / 8
				break
			}
			l = 8
			for i+l+8 <= n && l < fastMaxMatch {
				x = binary.LittleEndian.Uint64(data[i+l:]) ^ binary.LittleEndian.Uint64(data[i+l-d:])
				if x != 0 {
					l += bits.TrailingZeros64(x) / 8
					break
				}
				l += 8
			}
			if l >= 8 && x == 0 {
				for i+l < n && data[i+l] == data[i+l-d] {
					l++
				}
			}
		default:
			for i+l < n && l < 3 && data[i+l] == data[i+l-d] {
				l++
			}
		}
		if l >= 3 {
			l = min(l, fastMaxMatch)
			toks = append(toks, matchFlag|uint32(l))
			m.lit[257+int(lengthCode[l])]++
			m.n++
			i += l
			continue
		}
		toks = append(toks, uint32(data[i]))
		m.lit[data[i]]++
		i++
	}
	k := max(0, n-d)
	m.hist = append(m.hist[:0], data[k:]...)
	return toks
}

// fastTable is a pair of prefix codes for a block.
type fastTable struct {
	litLen  [286]uint8
	dist    [30]uint8
	litBits [286]uint16
	dstBits [30]uint16
	// header is the dynamic block header after BFINAL and BTYPE, as
	// (bits, count) pairs; none for deflate's fixed codes.
	header []fastBits
	btype  uint32
}

type fastBits struct {
	v uint32
	n uint8
}

// fixedCodes are deflate's fixed codes (RFC 1951, 3.2.6), for blocks too
// small to pay for a header and for building their own.
var fixedCodes = func() *fastTable {
	t := &fastTable{btype: 1}
	for i := range t.litLen {
		switch {
		case i < 144:
			t.litLen[i] = 8
		case i < 256:
			t.litLen[i] = 9
		case i < 280:
			t.litLen[i] = 7
		default:
			t.litLen[i] = 8
		}
	}
	for i := range t.dist {
		t.dist[i] = 5
	}
	// Canonical over all 288 and 32 symbols, of which the last two of each
	// are never used.
	var lit [288]uint8
	copy(lit[:], t.litLen[:])
	lit[286], lit[287] = 8, 8
	var litBits [288]uint16
	huffman.Canonical(lit[:], litBits[:])
	copy(t.litBits[:], litBits[:])
	var dist [32]uint8
	for i := range dist {
		dist[i] = 5
	}
	var dstBits [32]uint16
	huffman.Canonical(dist[:], dstBits[:])
	copy(t.dstBits[:], dstBits[:])
	return t
}()

// fastSmallBlock is the token count below which a block uses the fixed
// codes.
const fastSmallBlock = 4096

func newFastTable(litLen [286]uint8, dist [30]uint8) *fastTable {
	t := &fastTable{litLen: litLen, dist: dist, btype: 2}
	huffman.Canonical(t.litLen[:], t.litBits[:])
	huffman.Canonical(t.dist[:], t.dstBits[:])
	t.header = dynamicHeader(t.litLen[:], t.dist[:])
	return t
}

// codeLengthOrder is the order of the code length code's lengths.
var codeLengthOrder = [19]int{16, 17, 18, 0, 8, 7, 9, 6, 10, 5, 11, 4, 12, 3, 13, 2, 14, 1, 15}

// dynamicHeader is the header of a dynamic Huffman block for the code
// lengths given (RFC 1951, 3.2.7).
func dynamicHeader(lit, dist []uint8) []fastBits {
	nLit := len(lit)
	for nLit > 257 && lit[nLit-1] == 0 {
		nLit--
	}
	nDist := len(dist)
	for nDist > 1 && dist[nDist-1] == 0 {
		nDist--
	}
	all := append(append([]uint8(nil), lit[:nLit]...), dist[:nDist]...)
	// Run-length code the lengths: 16 repeats the previous 3–6 times, 17
	// and 18 write 3–10 and 11–138 zeros.
	type clSym struct{ sym, extra, extraBits int }
	var syms []clSym
	for i := 0; i < len(all); {
		v := all[i]
		run := 1
		for i+run < len(all) && all[i+run] == v {
			run++
		}
		i += run
		if v == 0 {
			for run >= 11 {
				k := min(run, 138)
				syms = append(syms, clSym{18, k - 11, 7})
				run -= k
			}
			if run >= 3 {
				syms = append(syms, clSym{17, run - 3, 3})
				run = 0
			}
		} else {
			syms = append(syms, clSym{int(v), 0, 0})
			run--
			for run >= 3 {
				k := min(run, 6)
				syms = append(syms, clSym{16, k - 3, 2})
				run -= k
			}
		}
		for range run {
			syms = append(syms, clSym{int(v), 0, 0})
		}
	}
	var counts [19]uint32
	for _, s := range syms {
		counts[s.sym]++
	}
	clLen := huffman.LengthLimited(counts[:], 7)
	clBits := make([]uint16, 19)
	huffman.Canonical(clLen, clBits)
	nCL := 19
	for nCL > 4 && clLen[codeLengthOrder[nCL-1]] == 0 {
		nCL--
	}
	h := []fastBits{{uint32(nLit - 257), 5}, {uint32(nDist - 1), 5}, {uint32(nCL - 4), 4}}
	for _, s := range codeLengthOrder[:nCL] {
		h = append(h, fastBits{uint32(clLen[s]), 3})
	}
	for _, s := range syms {
		h = append(h, fastBits{uint32(clBits[s.sym]), clLen[s.sym]})
		if s.extraBits > 0 {
			h = append(h, fastBits{uint32(s.extra), uint8(s.extraBits)})
		}
	}
	return h
}

// fastBlock writes one band as deflate blocks, least significant bit
// first, to w, each with codes built from its own symbols.
type fastBlock struct {
	w       io.Writer
	m       fastMatcher
	dcode   int // the distance's symbol and extra bits
	dx      uint32
	dxn     uint8
	pending []uint32 // the tokens not yet written
	out     []byte
	acc     uint64
	nAcc    uint
	err     error
}

func newFastBlock(w io.Writer, bpp int) *fastBlock {
	b := &fastBlock{w: w, m: fastMatcher{dist: bpp}}
	c, x, xn := distCode(bpp)
	b.dcode, b.dx, b.dxn = c, uint32(x), uint8(xn)
	return b
}

func (b *fastBlock) put(v uint32, n uint) {
	b.acc |= uint64(v) << b.nAcc
	b.nAcc += n
	if b.nAcc >= 32 {
		b.out = append(b.out, byte(b.acc), byte(b.acc>>8), byte(b.acc>>16), byte(b.acc>>24))
		b.acc >>= 32
		b.nAcc -= 32
		if len(b.out) >= 32<<10 {
			b.flushOut()
		}
	}
}

func (b *fastBlock) flushOut() {
	if b.err == nil && len(b.out) > 0 {
		_, b.err = b.w.Write(b.out)
	}
	b.out = b.out[:0]
}

// fastBlockTokens is how many tokens a block gathers before it is
// written: a block's codes cost a header and building them, which a
// highly compressible image, with few tokens per byte, should not pay
// every few kilobytes.
const fastBlockTokens = 1 << 16

// Write tokenizes the filtered bytes p. The tokens are written as a
// block once there are fastBlockTokens of them, or at close, when it is
// known whether the block is the last.
func (b *fastBlock) Write(p []byte) (int, error) {
	if len(b.pending) >= fastBlockTokens {
		b.block(b.pending, false)
		b.pending = b.pending[:0]
	}
	b.pending = b.m.tokenize(p, b.pending)
	return len(p), b.err
}

// block writes the tokens as one dynamic block.
func (b *fastBlock) block(toks []uint32, final bool) {
	t := fixedCodes
	if len(toks) >= fastSmallBlock {
		lc := b.m.lit
		lc[256]++
		if lc[0] == 0 {
			lc[0] = 1 // two codes at least: decoders want complete codes
		}
		var dc [30]uint32
		dc[b.dcode] = uint32(b.m.n) + 1
		dc[(b.dcode+1)%6] = 1
		var lit [286]uint8
		var dist [30]uint8
		copy(lit[:], huffman.LengthLimited(lc[:], 15))
		copy(dist[:], huffman.LengthLimited(dc[:], 15))
		t = newFastTable(lit, dist)
	}
	b.m.reset()
	f := uint32(0)
	if final {
		f = 1
	}
	b.put(f|t.btype<<1, 3) // BFINAL, BTYPE
	for _, h := range t.header {
		b.put(h.v, uint(h.n))
	}
	// A literal's code, and a length's code with its extra bits, as one
	// entry each: the bits, and their count above bit 24. The distance is
	// the same for every match.
	var litE [256]uint32
	for c := range 256 {
		litE[c] = uint32(t.litBits[c]) | uint32(t.litLen[c])<<24
	}
	var lenE [fastMaxMatch + 1]uint32
	for l := 3; l <= fastMaxMatch; l++ {
		c := int(lengthCode[l])
		n := uint32(t.litLen[257+c])
		lenE[l] = uint32(t.litBits[257+c]) | uint32(l-int(lengthBase[c]))<<n | (n+uint32(lengthExtra[c]))<<24
	}
	dv := uint64(t.dstBits[b.dcode]) | uint64(b.dx)<<t.dist[b.dcode]
	dn := uint(t.dist[b.dcode]) + uint(b.dxn)

	acc, nAcc, out := b.acc, b.nAcc, b.out
	for _, tk := range toks {
		var e uint32
		if tk&matchFlag == 0 {
			e = litE[tk]
		} else {
			e = lenE[tk&^matchFlag]
		}
		acc |= uint64(e&0xffffff) << nAcc
		nAcc += uint(e >> 24)
		if tk&matchFlag != 0 {
			// A length with its extra bits and a distance with its extra
			// bits can together pass what 64 bits hold on top of 31.
			if nAcc >= 32 {
				out = append(out, byte(acc), byte(acc>>8), byte(acc>>16), byte(acc>>24))
				acc >>= 32
				nAcc -= 32
			}
			acc |= dv << nAcc
			nAcc += dn
		}
		if nAcc >= 32 {
			out = append(out, byte(acc), byte(acc>>8), byte(acc>>16), byte(acc>>24))
			acc >>= 32
			nAcc -= 32
		}
	}
	b.acc, b.nAcc, b.out = acc, nAcc, out
	b.put(uint32(t.litBits[256]), uint(t.litLen[256]))
	if len(b.out) >= 32<<10 {
		b.flushOut()
	}
}

// close writes the last block and ends the band on a byte boundary: the
// last band with the final block's padding, the others with an empty
// stored block, as a sync flush does.
func (b *fastBlock) close(last bool) error {
	b.block(b.pending, last)
	b.pending = b.pending[:0]
	if !last {
		b.put(0, 3) // BFINAL 0, BTYPE 00: stored
	}
	for b.nAcc%8 != 0 {
		b.put(0, 1)
	}
	for ; b.nAcc > 0; b.nAcc -= 8 {
		b.out = append(b.out, byte(b.acc))
		b.acc >>= 8
	}
	if !last {
		b.out = append(b.out, 0, 0, 0xff, 0xff) // LEN 0, NLEN
	}
	b.flushOut()
	return b.err
}
