package tiff

import (
	"bytes"
	"io"
	"math/rand/v2"
	"testing"

	"golang.org/x/image/tiff/lzw"
)

func lzwRoundTrip(t testing.TB, src []byte) {
	t.Helper()
	enc := lzwCompress(nil, src)
	r := lzw.NewReader(bytes.NewReader(enc), lzw.MSB, 8)
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("decode %d bytes: %v", len(src), err)
	}
	if !bytes.Equal(got, src) {
		t.Fatalf("round trip of %d bytes differs", len(src))
	}
}

func TestLZWRoundTrip(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	inputs := [][]byte{nil, {0}, {1, 1, 1, 1}, bytes.Repeat([]byte{7}, 100000)}
	for _, n := range []int{10, 255, 256, 511, 512, 4000, 5000, 70000, 300000} {
		noise := make([]byte, n)
		for i := range noise {
			noise[i] = byte(r.IntN(256))
		}
		few := make([]byte, n)
		for i := range few {
			few[i] = byte(r.IntN(4))
		}
		inputs = append(inputs, noise, few)
	}
	for _, in := range inputs {
		lzwRoundTrip(t, in)
	}
}

func FuzzLZW(f *testing.F) {
	f.Add([]byte("TOBEORNOTTOBEORTOBEORNOT"))
	f.Fuzz(func(t *testing.T, b []byte) { lzwRoundTrip(t, b) })
}
