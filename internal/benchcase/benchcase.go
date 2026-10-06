// Package benchcase runs encoder benchmarks the same way for every format
// and implementation, so that their results compare.
//
// The measured operation is one call of an Encoder's Encode, which builds
// its encoder and writes the complete file into a bytes.Buffer. The
// buffer is reused and reset between calls and is already grown to the
// output size before timing, so output allocation is not timed for any
// implementation. Fixtures are built before Run.
//
// Before timing, each implementation encodes once: an error fails the
// benchmark, and check must accept the output. That call also warms any
// pools, so the timings are of repeated, warmed encoding; first calls in
// a fresh process are a separate measurement.
//
// Every result reports allocations, the output size in bytes and its
// ratio to the named reference's output, whichever sub-benchmarks are
// selected.
package benchcase

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"testing"
)

// Encoder is one implementation and configuration under measurement.
type Encoder struct {
	Name   string
	Encode func(w io.Writer) error
}

// Run benchmarks ref and encs as sub-benchmarks of b, named after them.
// check(want, got) validates an output got against the reference output
// want, outside timing, by decoding it as the format's contract demands.
func Run(b *testing.B, ref Encoder, encs []Encoder, check func(want, got []byte) error) {
	var want bytes.Buffer
	if err := ref.Encode(&want); err != nil {
		b.Fatalf("%s: %v", ref.Name, err)
	}
	if err := check(want.Bytes(), want.Bytes()); err != nil {
		b.Fatalf("%s: invalid output: %v", ref.Name, err)
	}
	for _, e := range append([]Encoder{ref}, encs...) {
		b.Run(e.Name, func(b *testing.B) {
			var buf bytes.Buffer
			if err := e.Encode(&buf); err != nil {
				b.Fatal(err)
			}
			if err := check(want.Bytes(), buf.Bytes()); err != nil {
				b.Fatalf("invalid output: %v", err)
			}
			b.ReportAllocs()
			for b.Loop() {
				buf.Reset()
				if err := e.Encode(&buf); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(buf.Len()), "bytes")
			b.ReportMetric(float64(buf.Len())/float64(want.Len()), "size/"+ref.Name)
		})
	}
}

// SamePixels reports where a and b differ, comparing bounds and every
// pixel's 16-bit premultiplied colour.
func SamePixels(a, b image.Image) error {
	if a.Bounds() != b.Bounds() {
		return fmt.Errorf("bounds %v and %v", a.Bounds(), b.Bounds())
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return fmt.Errorf("pixel %d,%d: %v and %v", x, y, a.At(x, y), b.At(x, y))
			}
		}
	}
	return nil
}
