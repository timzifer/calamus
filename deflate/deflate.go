// Package deflate lets calamus's encoders use another deflate compressor
// than compress/flate. An adapter is a few lines; it is best kept in a
// module of its own, so that its dependency reaches only programs that
// ask for it. (klauspost/compress/flate, tried here, writes the same bytes
// as compress/flate from Go 1.27 on, about 10–15 % faster.)
//
// The encoders cut a stream into bands and join them, so a compressor
// must write raw deflate (RFC 1951) and offer what compress/flate offers:
// a sync flush that ends on a byte boundary, a dictionary to prime a band
// with the bytes before it, and Reset to be reused.
package deflate

import (
	"compress/flate"
	"io"
)

// Writer is a raw deflate compressor, as *compress/flate.Writer is one.
type Writer interface {
	io.WriteCloser
	// Flush ends the data written so far on a byte boundary with an empty
	// stored block, without ending the stream (a sync flush).
	Flush() error
	// Reset discards the writer's state and makes it write to w, with
	// the level and dictionary it was made with.
	Reset(w io.Writer)
}

// Compressor makes Writers. Levels are compress/flate's, from
// flate.HuffmanOnly to flate.BestCompression. Encoders keep Writers made
// without a dictionary for reuse, per Compressor and level, so a
// Compressor must be comparable (an empty struct or a pointer is).
type Compressor interface {
	NewWriter(w io.Writer, level int) (Writer, error)
	NewWriterDict(w io.Writer, level int, dict []byte) (Writer, error)
}

// Standard is compress/flate, what the encoders use unless told
// otherwise.
var Standard Compressor = standard{}

type standard struct{}

func (standard) NewWriter(w io.Writer, level int) (Writer, error) {
	return flate.NewWriter(w, level)
}

func (standard) NewWriterDict(w io.Writer, level int, dict []byte) (Writer, error) {
	return flate.NewWriterDict(w, level, dict)
}
