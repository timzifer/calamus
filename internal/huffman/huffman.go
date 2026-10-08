// Package huffman builds length-limited canonical prefix codes, as
// deflate and VP8L use them: lengths per symbol, and codes stored
// bit-reversed for a least-significant-bit-first stream.
package huffman

import (
	"container/heap"
	"slices"
)

// Canonical assigns canonical codes to lengths (shorter first, then by
// symbol) and stores them bit-reversed.
func Canonical(lengths []uint8, out []uint16) {
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

// LengthLimited returns Huffman code lengths for counts, none longer than
// maxLen: a Huffman tree, rebuilt from flattened counts while it is too
// deep, as libwebp does. A single used symbol gets length 1; unused ones 0.
func LengthLimited(counts []uint32, maxLen int) []uint8 {
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
