// Package corpus is the benchmark corpus: real images from the libjxl
// test data, documents rendered with cera, diagrams rendered with figure,
// and synthetic images, each at the sizes its category is benchmarked at.
//
// Nothing here is fetched while benchmarks run. The libjxl test data is a
// git submodule at a pinned commit; the PDFs are downloaded once by
//
//	go run ./cmd/corpus fetch
//
// and verified by their SHA-256. Renders and generated images are checked
// against the SHA-256 of their pixels, so that a different cera, figure or
// generator cannot silently change what is measured.
package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io/fs"
	"os"
	"path/filepath"
)

// Sizes, per category: a fixture is small, medium or large for what its
// category is, not by one pixel count for all.
const (
	Small  = "small"
	Medium = "medium"
	Large  = "large"
)

// Fixture is one benchmark input.
type Fixture struct {
	// Name is category/size/detail.
	Name     string
	Category string // photo, document, diagram, graphic, synthetic
	Size     string // Small, Medium or Large
	// Real is false for synthetic images, which are reported apart.
	Real bool
	// Short fixtures make up the default suite; the full suite is all.
	Short bool
	// Source and License say where the input comes from and on what terms.
	Source, License string
	// File is the input file below the corpus directory, with its SHA-256;
	// empty for generated images.
	File, FileSHA256 string
	// PixelSHA256 is the SHA-256 of the image's pixels (see PixelSHA256),
	// from pixels.go, which cmd/corpus pixels writes.
	PixelSHA256 string

	load func(dir string, data []byte) (image.Image, error)
}

// ErrMissing reports that a fixture's input is not in the corpus
// directory: the submodule is not checked out or the PDFs not fetched.
var ErrMissing = errors.New("corpus input missing")

// Load reads, renders or generates the fixture's image from the corpus
// directory dir, verifying its file and pixels.
func (f Fixture) Load(dir string) (image.Image, error) {
	m, err := f.LoadUnchecked(dir)
	if err != nil {
		return nil, err
	}
	if got := PixelSHA256(m); got != f.PixelSHA256 {
		return nil, fmt.Errorf("%s: pixels have SHA-256 %s, want %s", f.Name, got, f.PixelSHA256)
	}
	return m, nil
}

// LoadUnchecked is Load without the pixel check, to record new checksums.
func (f Fixture) LoadUnchecked(dir string) (image.Image, error) {
	var data []byte
	if f.File != "" {
		var err error
		data, err = os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.File)))
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s: %w: %s", f.Name, ErrMissing, f.File)
		}
		if err != nil {
			return nil, err
		}
		if got := sum(data); got != f.FileSHA256 {
			return nil, fmt.Errorf("%s: %s has SHA-256 %s, want %s", f.Name, f.File, got, f.FileSHA256)
		}
	}
	m, err := f.load(dir, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Name, err)
	}
	return m, nil
}

// All returns every fixture; Short those of the default suite.
func All() []Fixture {
	var fs []Fixture
	fs = append(fs, libjxl()...)
	fs = append(fs, documents()...)
	fs = append(fs, diagrams()...)
	fs = append(fs, synthetic()...)
	for i := range fs {
		fs[i].PixelSHA256 = pixelSHA256[fs[i].Name]
	}
	return fs
}

// Short returns the fixtures of the default suite.
func Short() []Fixture {
	var out []Fixture
	for _, f := range All() {
		if f.Short {
			out = append(out, f)
		}
	}
	return out
}

// PixelSHA256 hashes m's pixels as 8-bit NRGBA, or as 16-bit NRGBA64 if
// m has more than 8 bits a sample, together with its bounds.
func PixelSHA256(m image.Image) string {
	h := sha256.New()
	b := m.Bounds()
	fmt.Fprintf(h, "%d %d %d %d\n", b.Min.X, b.Min.Y, b.Max.X, b.Max.Y)
	if deep(m) {
		n := image.NewNRGBA64(b)
		draw.Draw(n, b, m, b.Min, draw.Src)
		h.Write(n.Pix)
	} else {
		n := image.NewNRGBA(b)
		draw.Draw(n, b, m, b.Min, draw.Src)
		h.Write(n.Pix)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func deep(m image.Image) bool {
	switch m.(type) {
	case *image.Gray16, *image.RGBA64, *image.NRGBA64:
		return true
	}
	return false
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
