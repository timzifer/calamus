package png

import (
	"bytes"
	"image"
	stdpng "image/png"
	"io"
	"math/rand/v2"
	"sync/atomic"
	"testing"

	"github.com/timzifer/calamus/deflate"
)

// countingCompressor is compress/flate, counting the writers it makes.
type countingCompressor struct{ plain, dict atomic.Int32 }

func (c *countingCompressor) NewWriter(w io.Writer, level int) (deflate.Writer, error) {
	c.plain.Add(1)
	return deflate.Standard.NewWriter(w, level)
}

func (c *countingCompressor) NewWriterDict(w io.Writer, level int, dict []byte) (deflate.Writer, error) {
	c.dict.Add(1)
	return deflate.Standard.NewWriterDict(w, level, dict)
}

func TestCompressorIsUsed(t *testing.T) {
	r := rand.New(rand.NewPCG(41, 42))
	m := images(r, 640, 1500)["rgba-alpha"]
	c := &countingCompressor{}
	var got, want bytes.Buffer
	if err := (&Encoder{Workers: 8, Compressor: c}).Encode(&got, m); err != nil {
		t.Fatal(err)
	}
	if c.plain.Load() == 0 || c.dict.Load() == 0 {
		t.Fatalf("compressor made %d plain and %d primed writers", c.plain.Load(), c.dict.Load())
	}
	// compress/flate behind the interface writes what it writes directly.
	if err := (&Encoder{Workers: 8}).Encode(&want, m); err != nil {
		t.Fatal(err)
	}
	_, gi := chunks(t, got.Bytes())
	_, wi := chunks(t, want.Bytes())
	if !bytes.Equal(gi, wi) {
		t.Fatal("the stream differs from compress/flate's")
	}
	if _, err := stdpng.Decode(&got); err != nil {
		t.Fatal(err)
	}
	// FastCompression has a compressor of its own.
	c2 := &countingCompressor{}
	if err := (&Encoder{CompressionLevel: FastCompression, Compressor: c2}).Encode(io.Discard, image.NewRGBA(image.Rect(0, 0, 50, 50))); err != nil {
		t.Fatal(err)
	}
	if c2.plain.Load()+c2.dict.Load() != 0 {
		t.Fatal("FastCompression used the compressor")
	}
}
