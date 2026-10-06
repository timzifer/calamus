package png

import (
	"encoding/binary"
	"image"
	"image/color"
)

// PNG colour types.
const (
	ctGray    = 0
	ctRGB     = 2
	ctPalette = 3
	ctRGBA    = 6
)

// Image types with a fast path.
const (
	kindGeneric = iota
	kindGray
	kindGray16
	kindRGBA
	kindNRGBA
	kindRGBA64
	kindNRGBA64
	kindPaletted
)

// source reads the rows of an image as raw PNG scanlines. The colour type
// is chosen as image/png chooses it, so that both write the same pixels.
type source struct {
	m         image.Image
	b         image.Rectangle
	w, h      int
	colorType uint8
	depth     uint8
	bpp       int // bytes per complete pixel, for the filters (at least 1)
	pal       color.Palette
	kind      int
}

func newSource(m image.Image) *source {
	s := &source{m: m, b: m.Bounds()}
	s.w, s.h = s.b.Dx(), s.b.Dy()
	if _, ok := m.(image.PalettedImage); ok {
		s.pal, _ = m.ColorModel().(color.Palette)
	}
	switch {
	case s.pal != nil:
		s.colorType, s.bpp = ctPalette, 1
		switch {
		case len(s.pal) <= 2:
			s.depth = 1
		case len(s.pal) <= 4:
			s.depth = 2
		case len(s.pal) <= 16:
			s.depth = 4
		default:
			s.depth = 8
		}
	default:
		switch m.ColorModel() {
		case color.GrayModel:
			s.colorType, s.depth, s.bpp = ctGray, 8, 1
		case color.Gray16Model:
			s.colorType, s.depth, s.bpp = ctGray, 16, 2
		case color.RGBAModel, color.NRGBAModel, color.AlphaModel:
			if opaque(m) {
				s.colorType, s.depth, s.bpp = ctRGB, 8, 3
			} else {
				s.colorType, s.depth, s.bpp = ctRGBA, 8, 4
			}
		default:
			if opaque(m) {
				s.colorType, s.depth, s.bpp = ctRGB, 16, 6
			} else {
				s.colorType, s.depth, s.bpp = ctRGBA, 16, 8
			}
		}
	}
	switch m.(type) {
	case *image.Gray:
		s.kind = kindGray
	case *image.Gray16:
		s.kind = kindGray16
	case *image.RGBA:
		s.kind = kindRGBA
	case *image.NRGBA:
		s.kind = kindNRGBA
	case *image.RGBA64:
		s.kind = kindRGBA64
	case *image.NRGBA64:
		s.kind = kindNRGBA64
	case *image.Paletted:
		s.kind = kindPaletted
	}
	return s
}

// rowBytes is the length of a scanline without its filter byte.
func (s *source) rowBytes() int {
	switch s.colorType {
	case ctPalette:
		return (s.w*int(s.depth) + 7) / 8
	case ctGray:
		return s.w * int(s.depth) / 8
	case ctRGB:
		return s.w * 3 * int(s.depth) / 8
	}
	return s.w * 4 * int(s.depth) / 8
}

// opaque reports whether every pixel of m is opaque, as image/png checks it.
func opaque(m image.Image) bool {
	if o, ok := m.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	b := m.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := m.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}
	}
	return true
}

func (s *source) paletteChunks() (plte, trns []byte) {
	plte = make([]byte, 3*len(s.pal))
	alpha := make([]byte, len(s.pal))
	last := -1
	for i, c := range s.pal {
		n := color.NRGBAModel.Convert(c).(color.NRGBA)
		plte[3*i], plte[3*i+1], plte[3*i+2] = n.R, n.G, n.B
		alpha[i] = n.A
		if n.A != 0xff {
			last = i
		}
	}
	if last >= 0 {
		trns = alpha[:last+1]
	}
	return plte, trns
}

// row writes scanline y (0-based within the image) into dst, which holds
// rowBytes bytes.
func (s *source) row(y int, dst []byte) {
	y += s.b.Min.Y
	x0 := s.b.Min.X
	switch s.colorType {
	case ctPalette:
		s.paletteRow(y, dst)
	case ctGray:
		if s.depth == 8 {
			if m, ok := s.m.(*image.Gray); ok && s.kind == kindGray {
				copy(dst, m.Pix[m.PixOffset(x0, y):][:s.w])
				return
			}
			for x := range s.w {
				dst[x] = color.GrayModel.Convert(s.m.At(x0+x, y)).(color.Gray).Y
			}
			return
		}
		if m, ok := s.m.(*image.Gray16); ok && s.kind == kindGray16 {
			copy(dst, m.Pix[m.PixOffset(x0, y):][:2*s.w])
			return
		}
		for x := range s.w {
			c := color.Gray16Model.Convert(s.m.At(x0+x, y)).(color.Gray16)
			binary.BigEndian.PutUint16(dst[2*x:], c.Y)
		}
	case ctRGB:
		if s.depth == 8 {
			s.rgb8Row(x0, y, dst)
			return
		}
		if m, ok := s.m.(*image.RGBA64); ok && s.kind == kindRGBA64 {
			pix := m.Pix[m.PixOffset(x0, y):][:8*s.w]
			for x := range s.w {
				copy(dst[6*x:6*x+6], pix[8*x:8*x+6])
			}
			return
		}
		for x := range s.w {
			r, g, b, _ := s.m.At(x0+x, y).RGBA()
			binary.BigEndian.PutUint16(dst[6*x:], uint16(r))
			binary.BigEndian.PutUint16(dst[6*x+2:], uint16(g))
			binary.BigEndian.PutUint16(dst[6*x+4:], uint16(b))
		}
	default:
		if s.depth == 8 {
			s.rgba8Row(x0, y, dst)
			return
		}
		if m, ok := s.m.(*image.NRGBA64); ok && s.kind == kindNRGBA64 {
			copy(dst, m.Pix[m.PixOffset(x0, y):][:8*s.w])
			return
		}
		for x := range s.w {
			c := color.NRGBA64Model.Convert(s.m.At(x0+x, y)).(color.NRGBA64)
			binary.BigEndian.PutUint16(dst[8*x:], c.R)
			binary.BigEndian.PutUint16(dst[8*x+2:], c.G)
			binary.BigEndian.PutUint16(dst[8*x+4:], c.B)
			binary.BigEndian.PutUint16(dst[8*x+6:], c.A)
		}
	}
}

// rgb8Row writes an opaque 8-bit row as RGB.
func (s *source) rgb8Row(x0, y int, dst []byte) {
	var pix []byte
	switch m := s.m.(type) {
	case *image.RGBA:
		pix = m.Pix[m.PixOffset(x0, y):][:4*s.w]
	case *image.NRGBA:
		pix = m.Pix[m.PixOffset(x0, y):][:4*s.w]
	}
	if pix != nil {
		dst = dst[:3*s.w]
		for x := range s.w {
			p := pix[4*x : 4*x+3 : 4*x+3]
			d := dst[3*x : 3*x+3 : 3*x+3]
			d[0], d[1], d[2] = p[0], p[1], p[2]
		}
		return
	}
	for x := range s.w {
		r, g, b, _ := s.m.At(x0+x, y).RGBA()
		dst[3*x], dst[3*x+1], dst[3*x+2] = uint8(r>>8), uint8(g>>8), uint8(b>>8)
	}
}

// rgba8Row writes an 8-bit row with alpha as non-premultiplied RGBA, the
// way color.NRGBAModel converts.
func (s *source) rgba8Row(x0, y int, dst []byte) {
	switch m := s.m.(type) {
	case *image.NRGBA:
		copy(dst, m.Pix[m.PixOffset(x0, y):][:4*s.w])
		return
	case *image.RGBA:
		pix := m.Pix[m.PixOffset(x0, y):][:4*s.w]
		for x := range s.w {
			p := pix[4*x : 4*x+4 : 4*x+4]
			d := dst[4*x : 4*x+4 : 4*x+4]
			switch a := p[3]; a {
			case 0xff:
				d[0], d[1], d[2], d[3] = p[0], p[1], p[2], 0xff
			case 0:
				d[0], d[1], d[2], d[3] = 0, 0, 0, 0
			default:
				// color.NRGBAModel on the 16-bit values.
				a16 := uint32(a) * 0x101
				d[0] = uint8((uint32(p[0]) * 0x101 * 0xffff / a16) >> 8)
				d[1] = uint8((uint32(p[1]) * 0x101 * 0xffff / a16) >> 8)
				d[2] = uint8((uint32(p[2]) * 0x101 * 0xffff / a16) >> 8)
				d[3] = a
			}
		}
		return
	}
	for x := range s.w {
		c := color.NRGBAModel.Convert(s.m.At(x0+x, y)).(color.NRGBA)
		dst[4*x], dst[4*x+1], dst[4*x+2], dst[4*x+3] = c.R, c.G, c.B, c.A
	}
}

// paletteRow writes indices, packed to the palette's bit depth.
func (s *source) paletteRow(y int, dst []byte) {
	x0 := s.b.Min.X
	var idx []byte
	if m, ok := s.m.(*image.Paletted); ok && s.kind == kindPaletted {
		idx = m.Pix[m.PixOffset(x0, y):][:s.w]
	}
	at := func(x int) byte {
		if idx != nil {
			return idx[x]
		}
		return s.m.(image.PalettedImage).ColorIndexAt(x0+x, y)
	}
	if s.depth == 8 {
		if idx != nil {
			copy(dst, idx)
			return
		}
		for x := range s.w {
			dst[x] = at(x)
		}
		return
	}
	clear(dst)
	per := 8 / int(s.depth)
	for x := range s.w {
		shift := 8 - int(s.depth)*(x%per+1)
		dst[x/per] |= at(x) << shift
	}
}
