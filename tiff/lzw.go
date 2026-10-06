package tiff

// TIFF's LZW (TIFF 6.0, section 13): codes are written MSB first and
// widen one code early ("early change"), unlike GIF's LZW and Go's
// compress/lzw. Each strip is a stream of its own, from a Clear code to
// an EOI code.

const (
	lzwClear    = 256
	lzwEOI      = 257
	lzwFirst    = 258
	lzwMaxWidth = 12
	lzwMaxCode  = 1<<lzwMaxWidth - 2 // the last code before the table is cleared (early change)
	lzwHashBits = 13
	lzwHashSize = 1 << lzwHashBits
	lzwInvalid  = 0xffffffff
)

// lzwEncoder compresses one strip.
type lzwEncoder struct {
	out   []byte
	bits  uint32
	nBits uint
	width uint
	next  uint32 // the next code to assign
	table [lzwHashSize]uint32
}

func (e *lzwEncoder) emit(code uint32) {
	e.bits |= code << (32 - e.width - e.nBits)
	e.nBits += e.width
	for e.nBits >= 8 {
		e.out = append(e.out, byte(e.bits>>24))
		e.bits <<= 8
		e.nBits -= 8
	}
}

func (e *lzwEncoder) reset() {
	for i := range e.table {
		e.table[i] = lzwInvalid
	}
	e.width = 9
	e.next = lzwFirst
}

// compress appends the LZW stream of src to out.
func lzwCompress(out, src []byte) []byte {
	e := &lzwEncoder{out: out}
	e.reset()
	e.emit(lzwClear)
	if len(src) == 0 {
		e.emit(lzwEOI)
		return e.flush()
	}
	code := uint32(src[0])
	for _, c := range src[1:] {
		// The table maps (code, byte) to a code; key and value share one
		// uint32 per slot: key in the high 20 bits, value in the low 12.
		key := code<<8 | uint32(c)
		h := (key>>12 ^ key) & (lzwHashSize - 1)
		found := false
		for t := e.table[h]; t != lzwInvalid; {
			if t>>12 == key {
				code = t & 0xfff
				found = true
				break
			}
			h = (h + 1) & (lzwHashSize - 1)
			t = e.table[h]
		}
		if found {
			continue
		}
		e.emit(code)
		code = uint32(c)
		if e.next >= lzwMaxCode {
			// Table full: start afresh, as the decoder does on Clear. The
			// decoder may have widened after the code just written.
			if e.next+1 >= 1<<e.width && e.width < lzwMaxWidth {
				e.width++
			}
			e.emit(lzwClear)
			e.reset()
			continue
		}
		e.table[h] = key<<12 | e.next
		e.next++
		// Early change: widen when the next code would need the next width
		// one code before it is reached.
		if e.next+1 > 1<<e.width && e.width < lzwMaxWidth {
			e.width++
		}
	}
	e.emit(code)
	// The decoder adds a table entry for the code just read; widen as it will.
	if e.next+1 >= 1<<e.width && e.width < lzwMaxWidth {
		e.width++
	}
	e.emit(lzwEOI)
	return e.flush()
}

func (e *lzwEncoder) flush() []byte {
	if e.nBits > 0 {
		e.out = append(e.out, byte(e.bits>>24))
		e.bits, e.nBits = 0, 0
	}
	return e.out
}
