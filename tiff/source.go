package tiff

import "image"

// source reads rows of an image as TIFF samples, little-endian, with the
// horizontal predictor applied if asked for. Colour types follow
// golang.org/x/image/tiff: Paletted (8 bits and a colour map), Gray,
// Gray16, NRGBA and NRGBA64 (unassociated alpha), RGBA and RGBA64
// (associated alpha), anything else as RGBA through At.
type source struct {
	m             image.Image
	b             image.Rectangle
	bitsPerSample []uint32
	photometric   uint32
	extraSamples  uint32
	colorMap      []uint32
	predictor     bool
	bpp           int // bytes per pixel
}

func newSource(m image.Image, predictor bool) *source {
	s := &source{m: m, b: m.Bounds(), predictor: predictor, photometric: pRGB}
	switch m := m.(type) {
	case *image.Paletted:
		s.photometric, s.bitsPerSample, s.bpp = pPaletted, []uint32{8}, 1
		s.colorMap = make([]uint32, 256*3)
		for i := 0; i < 256 && i < len(m.Palette); i++ {
			r, g, b, _ := m.Palette[i].RGBA()
			s.colorMap[i], s.colorMap[i+256], s.colorMap[i+512] = r, g, b
		}
	case *image.Gray:
		s.photometric, s.bitsPerSample, s.bpp = pBlackIsZero, []uint32{8}, 1
	case *image.Gray16:
		s.photometric, s.bitsPerSample, s.bpp = pBlackIsZero, []uint32{16}, 2
	case *image.NRGBA:
		s.extraSamples, s.bitsPerSample, s.bpp = 2, []uint32{8, 8, 8, 8}, 4
	case *image.NRGBA64:
		s.extraSamples, s.bitsPerSample, s.bpp = 2, []uint32{16, 16, 16, 16}, 8
	case *image.RGBA64:
		s.extraSamples, s.bitsPerSample, s.bpp = 1, []uint32{16, 16, 16, 16}, 8
	default: // *image.RGBA and anything else
		s.extraSamples, s.bitsPerSample, s.bpp = 1, []uint32{8, 8, 8, 8}, 4
	}
	return s
}

func (s *source) rowBytes() int { return s.b.Dx() * s.bpp }

// rows returns rows [y0, y1), 0-based within the image.
func (s *source) rows(y0, y1 int) []byte {
	n := s.rowBytes()
	out := make([]byte, n*(y1-y0))
	for y := y0; y < y1; y++ {
		s.row(y+s.b.Min.Y, out[(y-y0)*n:][:n])
	}
	return out
}

func (s *source) row(y int, dst []byte) {
	x0, w := s.b.Min.X, s.b.Dx()
	switch m := s.m.(type) {
	case *image.Paletted:
		copy(dst, m.Pix[m.PixOffset(x0, y):][:w])
		s.predict8(dst, 1)
	case *image.Gray:
		copy(dst, m.Pix[m.PixOffset(x0, y):][:w])
		s.predict8(dst, 1)
	case *image.Gray16:
		swap16(dst, m.Pix[m.PixOffset(x0, y):][:2*w])
		s.predict16(dst, 1)
	case *image.NRGBA:
		copy(dst, m.Pix[m.PixOffset(x0, y):][:4*w])
		s.predict8(dst, 4)
	case *image.RGBA:
		copy(dst, m.Pix[m.PixOffset(x0, y):][:4*w])
		s.predict8(dst, 4)
	case *image.NRGBA64:
		swap16(dst, m.Pix[m.PixOffset(x0, y):][:8*w])
		s.predict16(dst, 4)
	case *image.RGBA64:
		swap16(dst, m.Pix[m.PixOffset(x0, y):][:8*w])
		s.predict16(dst, 4)
	default:
		for x := range w {
			r, g, b, a := s.m.At(x0+x, y).RGBA()
			dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
		}
		s.predict8(dst, 4)
	}
}

// swap16 copies big-endian 16-bit samples as little-endian ones.
func swap16(dst, src []byte) {
	for i := 0; i+1 < len(src); i += 2 {
		dst[i], dst[i+1] = src[i+1], src[i]
	}
}

// predict8 replaces each 8-bit sample by its difference to the same
// sample of the pixel before (TIFF 6.0, section 14), right to left.
func (s *source) predict8(row []byte, spp int) {
	if !s.predictor {
		return
	}
	for i := len(row) - 1; i >= spp; i-- {
		row[i] -= row[i-spp]
	}
}

// predict16 does so for little-endian 16-bit samples.
func (s *source) predict16(row []byte, spp int) {
	if !s.predictor {
		return
	}
	step := 2 * spp
	for i := len(row) - 2; i >= step; i -= 2 {
		v := uint16(row[i]) | uint16(row[i+1])<<8
		p := uint16(row[i-step]) | uint16(row[i-step+1])<<8
		v -= p
		row[i], row[i+1] = byte(v), byte(v>>8)
	}
}
