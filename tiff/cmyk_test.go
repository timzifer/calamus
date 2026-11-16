package tiff

import (
	"bytes"
	"compress/zlib"
	"image"
	"io"
	"math/rand/v2"
	"testing"

	xtiff "golang.org/x/image/tiff"
	"golang.org/x/image/tiff/lzw"
)

// ifdOf reads the first IFD of a little-endian TIFF: each tag's values,
// and the bytes of its UNDEFINED ones.
func ifdOf(t testing.TB, b []byte) (map[int][]uint32, map[int][]byte) {
	t.Helper()
	if len(b) < 8 || string(b[:4]) != "II*\x00" {
		t.Fatal("not a little-endian TIFF")
	}
	off := int(le.Uint32(b[4:]))
	n := int(le.Uint16(b[off:]))
	vals, raw := map[int][]uint32{}, map[int][]byte{}
	for i := range n {
		e := b[off+2+12*i:]
		tag, typ, count := int(le.Uint16(e)), int(le.Uint16(e[2:])), int(le.Uint32(e[4:]))
		size := count * typeLen[typ]
		if typ == dtRational {
			size = 8 * count
		}
		v := e[8:12]
		if size > 4 {
			v = b[le.Uint32(e[8:]):]
		}
		v = v[:size]
		switch typ {
		case dtShort:
			for k := range count {
				vals[tag] = append(vals[tag], uint32(le.Uint16(v[2*k:])))
			}
		case dtLong:
			for k := range count {
				vals[tag] = append(vals[tag], le.Uint32(v[4*k:]))
			}
		case dtUndefined:
			raw[tag] = v
		}
	}
	return vals, raw
}

// checkCMYK reads a TIFF of separated samples, its strips decompressed and
// the predictor undone, and compares it with m.
func checkCMYK(t testing.TB, b []byte, m *image.CMYK) {
	t.Helper()
	tags, _ := ifdOf(t, b)
	want := map[int][]uint32{
		tPhotometricInterpretation: {pSeparated}, tSamplesPerPixel: {4},
		tBitsPerSample: {8, 8, 8, 8}, tInkSet: {inkSetCMYK},
	}
	for tag, v := range want {
		if !equal32(tags[tag], v) {
			t.Fatalf("tag %d: %v, want %v", tag, tags[tag], v)
		}
	}
	if tags[tExtraSamples] != nil {
		t.Fatal("extra samples")
	}
	w, h := m.Rect.Dx(), m.Rect.Dy()
	var pix []byte
	for i, off := range tags[tStripOffsets] {
		s := b[off:][:tags[tStripByteCounts][i]]
		var r io.Reader = bytes.NewReader(s)
		switch tags[tCompression][0] {
		case cDeflate:
			zr, err := zlib.NewReader(r)
			if err != nil {
				t.Fatal(err)
			}
			r = zr
		case cLZW:
			r = lzw.NewReader(r, lzw.MSB, 8)
		}
		d, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		pix = append(pix, d...)
	}
	if len(pix) != 4*w*h {
		t.Fatalf("%d bytes, want %d", len(pix), 4*w*h)
	}
	for y := range h {
		row := pix[4*w*y:][:4*w]
		if p := tags[tPredictor]; p != nil && p[0] == prHorizontal {
			for i := 4; i < len(row); i++ {
				row[i] += row[i-4]
			}
		}
		if !bytes.Equal(row, m.Pix[m.PixOffset(m.Rect.Min.X, m.Rect.Min.Y+y):][:4*w]) {
			t.Fatalf("row %d differs", y)
		}
	}
}

func equal32(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestICCProfile embeds a profile, of odd length, in TIFFs of ink and of
// RGB, which x/image/tiff still reads.
func TestICCProfile(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	icc := make([]byte, 1001)
	for i := range icc {
		icc[i] = byte(r.IntN(256))
	}
	ms := testImages(r, 40, 30)
	for _, name := range []string{"cmyk", "rgba", "gray"} {
		var buf bytes.Buffer
		enc := Encoder{Options: Options{Compression: LZW, Predictor: true, ICCProfile: icc}, Workers: 4}
		if err := enc.Encode(&buf, ms[name]); err != nil {
			t.Fatal(err)
		}
		if _, raw := ifdOf(t, buf.Bytes()); !bytes.Equal(raw[tICCProfile], icc) {
			t.Errorf("%s: profile not embedded", name)
		}
		if c, ok := ms[name].(*image.CMYK); ok {
			checkCMYK(t, buf.Bytes(), c)
		} else if _, err := xtiff.Decode(bytes.NewReader(buf.Bytes())); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
