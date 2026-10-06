package gif

// GIF's LZW, as compress/lzw writes it (LSB first, no early change), cut
// into bands: each band starts with a Clear code, so it needs no table
// from the band before, and the bands' bit streams are spliced together
// without padding, as GIF knows no byte alignment.

const (
	lzwMaxWidth   = 12
	lzwMaxCode    = 1<<lzwMaxWidth - 1
	lzwTableBits  = 13
	lzwTableSize  = 1 << lzwTableBits
	lzwTableMask  = lzwTableSize - 1
	lzwInvalidEnt = 0
	lzwNoCode     = 1<<32 - 1
)

// bitStream is an LSB-first bit stream: whole bytes and up to 7 pending bits.
type bitStream struct {
	b    []byte
	acc  uint32
	nAcc uint
}

func (s *bitStream) put(code uint32, width uint) {
	s.acc |= code << s.nAcc
	s.nAcc += width
	for s.nAcc >= 8 {
		s.b = append(s.b, byte(s.acc))
		s.acc >>= 8
		s.nAcc -= 8
	}
}

// splice appends the stream t.
func (s *bitStream) splice(t *bitStream) {
	if s.nAcc == 0 {
		s.b = append(s.b, t.b...)
	} else {
		for _, c := range t.b {
			s.put(uint32(c), 8)
		}
	}
	if t.nAcc > 0 {
		s.put(t.acc, t.nAcc)
	}
}

// finish pads the last byte with zero bits.
func (s *bitStream) finish() []byte {
	if s.nAcc > 0 {
		s.b = append(s.b, byte(s.acc))
		s.acc, s.nAcc = 0, 0
	}
	return s.b
}

// lzwBand encodes pix as one band of a stream. The first band starts with
// a Clear code; every band but the last ends with one, written at the
// width the decoder has reached, so the next band starts from a fresh
// table and width without knowing this band's; the last band ends with
// EOI. The code-width bookkeeping is compress/lzw's writer's, so a stream
// of one band is the very stream compress/lzw writes.
func lzwBand(pix [][]byte, litWidth uint, first, last bool) *bitStream {
	var (
		s        bitStream
		clear    = uint32(1) << litWidth
		width    = litWidth + 1
		hi       = clear + 1
		overflow = clear << 1
		saved    = uint32(lzwNoCode)
		table    [lzwTableSize]uint32
	)
	n := 0
	for _, row := range pix {
		n += len(row)
	}
	s.b = make([]byte, 0, n/2+16)
	if first {
		s.put(clear, width)
	}
	// incHi advances to the next code, widening or clearing as the
	// decoder will; it reports whether the table was cleared.
	incHi := func() bool {
		hi++
		if hi == overflow {
			width++
			overflow <<= 1
		}
		if hi == lzwMaxCode {
			s.put(clear, width)
			width = litWidth + 1
			hi = clear + 1
			overflow = clear << 1
			table = [lzwTableSize]uint32{}
			return true
		}
		return false
	}
	for _, row := range pix {
		for _, c := range row {
			literal := uint32(c)
			if saved == lzwNoCode {
				saved = literal
				continue
			}
			key := saved<<8 | literal
			hash := (key>>12 ^ key) & lzwTableMask
			found := false
			for h, t := hash, table[hash]; t != lzwInvalidEnt; {
				if key == t>>12 {
					saved = t & lzwMaxCode
					found = true
					break
				}
				h = (h + 1) & lzwTableMask
				t = table[h]
				hash = h
			}
			if found {
				continue
			}
			s.put(saved, width)
			saved = literal
			if incHi() {
				continue
			}
			table[hash] = key<<12 | hi
		}
	}
	if saved != lzwNoCode {
		s.put(saved, width)
		incHi()
	}
	if last {
		s.put(clear+1, width)
	} else {
		s.put(clear, width)
	}
	return &s
}
