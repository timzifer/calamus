package webp

import (
	"container/heap"
	"slices"
)

// bitWriter is VP8L's bit stream: least significant bit first, no byte
// alignment anywhere, so the streams of bands can be spliced.
type bitWriter struct {
	b    []byte
	acc  uint64
	nAcc uint
}

func (w *bitWriter) put(v uint32, n uint) {
	w.acc |= uint64(v) << w.nAcc
	w.nAcc += n
	for w.nAcc >= 8 {
		w.b = append(w.b, byte(w.acc))
		w.acc >>= 8
		w.nAcc -= 8
	}
}

// splice appends the stream t.
func (w *bitWriter) splice(t *bitWriter) {
	if w.nAcc == 0 {
		w.b = append(w.b, t.b...)
	} else {
		for _, c := range t.b {
			w.put(uint32(c), 8)
		}
	}
	if t.nAcc > 0 {
		w.put(uint32(t.acc), t.nAcc)
	}
}

func (w *bitWriter) finish() []byte {
	if w.nAcc > 0 {
		w.b = append(w.b, byte(w.acc))
		w.acc, w.nAcc = 0, 0
	}
	return w.b
}

// code is a prefix code: per symbol its length and its bits, reversed so
// that they go out most significant bit first, as VP8L reads them.
type code struct {
	lengths []uint8
	bits    []uint16
	single  bool // one symbol only: it takes no bits
}

func (c *code) write(w *bitWriter, sym int) {
	if !c.single {
		w.put(uint32(c.bits[sym]), uint(c.lengths[sym]))
	}
}

// newCode builds a code from symbol counts, no longer than maxLen bits.
func newCode(counts []uint32, maxLen int) *code {
	c := &code{lengths: lengthLimited(counts, maxLen), bits: make([]uint16, len(counts))}
	used := 0
	for _, l := range c.lengths {
		if l > 0 {
			used++
		}
	}
	c.single = used <= 1
	canonical(c.lengths, c.bits)
	return c
}

// canonical assigns canonical codes to lengths (shorter first, then by
// symbol) and stores them bit-reversed.
func canonical(lengths []uint8, out []uint16) {
	var count [16]int
	for _, l := range lengths {
		count[l]++
	}
	count[0] = 0
	var next [16]int
	c := 0
	for l := 1; l < 16; l++ {
		c = (c + count[l-1]) << 1
		next[l] = c
	}
	for s, l := range lengths {
		if l == 0 {
			continue
		}
		v := next[l]
		next[l]++
		var r uint16
		for i := 0; i < int(l); i++ {
			r = r<<1 | uint16(v>>i&1)
		}
		out[s] = r
	}
}

// lengthLimited returns Huffman code lengths for counts, none longer than
// maxLen: a Huffman tree, rebuilt from flattened counts while it is too
// deep, as libwebp does. A single used symbol gets length 1; unused ones 0.
func lengthLimited(counts []uint32, maxLen int) []uint8 {
	lengths := make([]uint8, len(counts))
	var syms []int
	for s, c := range counts {
		if c > 0 {
			syms = append(syms, s)
		}
	}
	switch len(syms) {
	case 0:
		return lengths
	case 1:
		lengths[syms[0]] = 1
		return lengths
	}
	floor := uint32(1)
	for {
		depth := huffmanDepths(counts, syms, floor)
		if slices.Max(depth) <= maxLen {
			for i, s := range syms {
				lengths[s] = uint8(depth[i])
			}
			return lengths
		}
		floor *= 2
	}
}

type node struct {
	weight      uint64
	left, right int // children, -1 for a leaf
	sym         int // index into syms for a leaf
	order       int // tie-break, for a deterministic tree
}

type nodeHeap struct {
	nodes []node
	idx   []int
}

func (h nodeHeap) Len() int { return len(h.idx) }
func (h nodeHeap) Less(i, j int) bool {
	a, b := h.nodes[h.idx[i]], h.nodes[h.idx[j]]
	if a.weight != b.weight {
		return a.weight < b.weight
	}
	return a.order < b.order
}
func (h nodeHeap) Swap(i, j int) { h.idx[i], h.idx[j] = h.idx[j], h.idx[i] }
func (h *nodeHeap) Push(x any)   { h.idx = append(h.idx, x.(int)) }
func (h *nodeHeap) Pop() any {
	x := h.idx[len(h.idx)-1]
	h.idx = h.idx[:len(h.idx)-1]
	return x
}

// huffmanDepths returns the depth of each used symbol in a Huffman tree
// of the counts, each raised to at least floor.
func huffmanDepths(counts []uint32, syms []int, floor uint32) []int {
	h := &nodeHeap{}
	for i, s := range syms {
		h.nodes = append(h.nodes, node{weight: uint64(max(counts[s], floor)), left: -1, right: -1, sym: i, order: i})
		h.idx = append(h.idx, i)
	}
	heap.Init(h)
	for h.Len() > 1 {
		a := heap.Pop(h).(int)
		b := heap.Pop(h).(int)
		h.nodes = append(h.nodes, node{weight: h.nodes[a].weight + h.nodes[b].weight, left: a, right: b, order: len(h.nodes)})
		heap.Push(h, len(h.nodes)-1)
	}
	depth := make([]int, len(syms))
	var walk func(n, d int)
	walk = func(n, d int) {
		if h.nodes[n].left < 0 {
			depth[h.nodes[n].sym] = d
			return
		}
		walk(h.nodes[n].left, d+1)
		walk(h.nodes[n].right, d+1)
	}
	walk(h.idx[0], 0)
	return depth
}

// codeLengthOrder is the order code length code lengths are written in.
var codeLengthOrder = [19]int{17, 18, 0, 1, 2, 3, 4, 5, 16, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// writeCode writes the code c for an alphabet (RFC 9649, 3.7.2.1).
func writeCode(w *bitWriter, c *code) {
	var used []int
	for s, l := range c.lengths {
		if l > 0 {
			used = append(used, s)
		}
	}
	// Simple code: one or two symbols below 256.
	if len(used) <= 2 && (len(used) == 0 || used[len(used)-1] < 256) {
		w.put(1, 1)
		if len(used) == 0 {
			used = []int{0}
		}
		w.put(uint32(len(used)-1), 1)
		if used[0] < 2 {
			w.put(0, 1)
			w.put(uint32(used[0]), 1)
		} else {
			w.put(1, 1)
			w.put(uint32(used[0]), 8)
		}
		if len(used) == 2 {
			w.put(uint32(used[1]), 8)
		}
		// The decoder gives two symbols the codes 0 and 1 in the order
		// written, one symbol none.
		if len(used) == 2 {
			c.lengths[used[0]], c.lengths[used[1]] = 1, 1
			c.bits[used[0]], c.bits[used[1]] = 0, 1
			c.single = false
		}
		return
	}
	w.put(0, 1)

	// The code lengths, run-length coded with 16 (repeat the previous
	// length 3–6 times), 17 (3–10 zeros) and 18 (11–138 zeros).
	type token struct{ sym, extra, nExtra int }
	var tokens []token
	ls := c.lengths
	for i := 0; i < len(ls); {
		v := ls[i]
		run := 1
		for i+run < len(ls) && ls[i+run] == v {
			run++
		}
		i += run
		if v == 0 {
			for run >= 3 {
				switch {
				case run >= 11:
					n := min(run, 138)
					tokens = append(tokens, token{18, n - 11, 7})
					run -= n
				default:
					n := min(run, 10)
					tokens = append(tokens, token{17, n - 3, 3})
					run -= n
				}
			}
			for ; run > 0; run-- {
				tokens = append(tokens, token{0, 0, 0})
			}
			continue
		}
		tokens = append(tokens, token{int(v), 0, 0})
		run--
		for run >= 3 {
			n := min(run, 6)
			tokens = append(tokens, token{16, n - 3, 2})
			run -= n
		}
		for ; run > 0; run-- {
			tokens = append(tokens, token{int(v), 0, 0})
		}
	}
	var counts [19]uint32
	for _, t := range tokens {
		counts[t.sym]++
	}
	clc := newCode(counts[:], 7)
	n := 19
	for n > 4 && clc.lengths[codeLengthOrder[n-1]] == 0 {
		n--
	}
	w.put(uint32(n-4), 4)
	for _, s := range codeLengthOrder[:n] {
		w.put(uint32(clc.lengths[s]), 3)
	}
	w.put(0, 1) // the lengths of the whole alphabet follow
	for _, t := range tokens {
		clc.write(w, t.sym)
		if t.nExtra > 0 {
			w.put(uint32(t.extra), uint(t.nExtra))
		}
	}
}
