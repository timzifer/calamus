package png

import "bytes"

// PNG filter types.
const (
	ftNone = iota
	ftSub
	ftUp
	ftAverage
	ftPaeth
)

// filterer turns raw rows into filtered scanlines, choosing a filter per
// row with libpng's heuristic, as image/png does: the least sum of
// absolute values, filters tried in the order Up, Paeth, None, Sub,
// Average, the first of equal sums kept. A row equal to the previous one
// takes Up at once: it would win anyway, with a sum of 0.
type filterer struct {
	src       *source
	noFilter  bool // palette images and no compression: filter None, as image/png does
	fast      bool // FastCompression: Up on every row
	cur, prev []byte
	cand      [5][]byte
	zeros     []byte // an Up-filtered row equal to the previous one
}

func newFilterer(src *source, level CompressionLevel) *filterer {
	n := src.rowBytes()
	f := &filterer{src: src, noFilter: src.colorType == ctPalette || level == NoCompression, fast: level == FastCompression}
	f.cur, f.prev, f.zeros = make([]byte, n), make([]byte, n), make([]byte, n)
	if !f.noFilter {
		for i := range f.cand {
			f.cand[i] = make([]byte, n)
		}
	}
	return f
}

// each filters rows [y0, y1) one at a time and hands each to fn as its
// filter type and filtered bytes, which are valid until fn returns.
func (f *filterer) each(y0, y1 int, fn func(ft byte, row []byte) error) error {
	if y0 > 0 {
		f.src.row(y0-1, f.prev)
	} else {
		clear(f.prev)
	}
	for y := y0; y < y1; y++ {
		f.src.row(y, f.cur)
		var err error
		switch {
		case f.noFilter:
			err = fn(ftNone, f.cur)
		case y == 0 && f.src.detached && f.fast:
			err = fn(ftNone, f.cur)
		case y == 0 && f.src.detached:
			ft := f.chooseWithoutPrev()
			err = fn(byte(ft), f.cand[ft])
		case f.fast:
			up := f.cand[ftUp][:len(f.cur)]
			for i, c := range f.cur {
				up[i] = c - f.prev[i]
			}
			err = fn(ftUp, up)
		case bytes.Equal(f.cur, f.prev):
			err = fn(ftUp, f.zeros)
		default:
			ft := f.choose()
			err = fn(byte(ft), f.cand[ft])
		}
		if err != nil {
			return err
		}
		f.cur, f.prev = f.prev, f.cur
	}
	return nil
}

// rows appends the filtered scanlines of rows [y0, y1) to out.
func (f *filterer) rows(y0, y1 int, out []byte) []byte {
	f.each(y0, y1, func(ft byte, row []byte) error {
		out = append(append(out, ft), row...)
		return nil
	})
	return out
}

func abs8(b byte) int {
	if b < 128 {
		return int(b)
	}
	return 256 - int(b)
}

// choose fills the candidate rows and returns the best filter. A filter
// whose running sum reaches the best so far is abandoned.
func (f *filterer) choose() int {
	cur, prev, bpp := f.cur, f.prev, f.src.bpp
	n := len(cur)
	prev = prev[:n]

	// Up.
	up := f.cand[ftUp][:n]
	sum := 0
	for i := range n {
		up[i] = cur[i] - prev[i]
		sum += abs8(up[i])
	}
	best, ft := sum, ftUp

	// Paeth.
	pa := f.cand[ftPaeth][:n]
	sum = 0
	for i := 0; i < bpp && i < n; i++ {
		pa[i] = cur[i] - prev[i]
		sum += abs8(pa[i])
	}
	for i := bpp; i < n && sum < best; i++ {
		pa[i] = cur[i] - paeth(cur[i-bpp], prev[i], prev[i-bpp])
		sum += abs8(pa[i])
	}
	if sum < best {
		best, ft = sum, ftPaeth
	}

	// None.
	sum = 0
	for i := 0; i < n && sum < best; i++ {
		sum += abs8(cur[i])
	}
	if sum < best {
		best, ft = sum, ftNone
		copy(f.cand[ftNone], cur)
	}

	// Sub.
	sb := f.cand[ftSub][:n]
	sum = 0
	for i := 0; i < bpp && i < n; i++ {
		sb[i] = cur[i]
		sum += abs8(sb[i])
	}
	for i := bpp; i < n && sum < best; i++ {
		sb[i] = cur[i] - cur[i-bpp]
		sum += abs8(sb[i])
	}
	if sum < best {
		best, ft = sum, ftSub
	}

	// Average.
	av := f.cand[ftAverage][:n]
	sum = 0
	for i := 0; i < bpp && i < n; i++ {
		av[i] = cur[i] - prev[i]/2
		sum += abs8(av[i])
	}
	for i := bpp; i < n && sum < best; i++ {
		av[i] = cur[i] - uint8((int(cur[i-bpp])+int(prev[i]))/2)
		sum += abs8(av[i])
	}
	if sum < best {
		ft = ftAverage
	}
	return ft
}

// chooseWithoutPrev is choose for a row whose previous row the decoder
// has but the encoder does not: of the filters that do not read it, None
// and Sub, in image/png's order, the first of equal sums kept.
func (f *filterer) chooseWithoutPrev() int {
	cur, bpp := f.cur, f.src.bpp
	n := len(cur)
	best := 0
	for i := range n {
		best += abs8(cur[i])
	}
	copy(f.cand[ftNone], cur)
	sb := f.cand[ftSub][:n]
	sum := 0
	for i := 0; i < bpp && i < n; i++ {
		sb[i] = cur[i]
		sum += abs8(sb[i])
	}
	for i := bpp; i < n && sum < best; i++ {
		sb[i] = cur[i] - cur[i-bpp]
		sum += abs8(sb[i])
	}
	if sum < best {
		return ftSub
	}
	return ftNone
}

// paeth is the Paeth predictor (PNG specification, 9.4).
func paeth(a, b, c uint8) uint8 {
	pc := int(c)
	pa := int(b) - pc
	pb := int(a) - pc
	pc = pa + pb
	if pa < 0 {
		pa = -pa
	}
	if pb < 0 {
		pb = -pb
	}
	if pc < 0 {
		pc = -pc
	}
	if pa <= pb && pa <= pc {
		return a
	} else if pb <= pc {
		return b
	}
	return c
}
