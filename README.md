# calamus

Image encoders in pure Go that use all cores: each cuts the image into
bands, encodes them concurrently and joins them into one ordinary file
that any decoder reads. No cgo, every GOOS/GOARCH including `js/wasm`.

| package | format | how it splits | status |
|---|---|---|---|
| `calamus/png` | PNG, animated PNG (APNG) | bands of one zlib stream, each primed with its neighbour's last 32 KiB; frames concurrently | ready |
| `calamus/jpeg` | baseline JPEG | bands of MCU rows between restart markers | ready |
| `calamus/tiff` | TIFF | strips, compressed independently by design | ready |
| `calamus/gif` | GIF, animated GIF | frames concurrently; LZW bands spliced bit by bit; Floyd-Steinberg as a wavefront | ready |
| `calamus/webp` | lossless WebP, animated WebP | transforms per pixel, LZ77 per band, one set of codes, bit streams spliced; frames concurrently | ready |

## PNG

A drop-in for `image/png`; the filtered scanlines are byte for byte those
`image/png` writes.

```go
import "github.com/timzifer/calamus/png"

err := png.Encode(w, img) // like image/png's Encode, on all cores

enc := png.Encoder{CompressionLevel: png.BestSpeed, Workers: 0} // 0 = GOMAXPROCS
err = enc.Encode(w, img)
```

`Encoder.Compressor` takes another deflate compressor than compress/flate
(package `deflate`: raw deflate with sync flush, dictionaries and Reset).
klauspost/compress/flate was tried for it: from Go 1.27 on it writes the
same bytes as compress/flate, only about 10–15 % faster, so calamus ships
no adapter.

### Streaming

A renderer that draws in bands can hand each band over as soon as it is
drawn, so that drawing and encoding overlap. The header fixes the colour
type up front; `WriteRows` takes rows from any goroutine, in any order,
encodes them on the calling goroutine and returns, so the band's buffer
can be drawn into again; the bytes go out in row order.

```go
w, err := (&png.Encoder{}).NewWriter(out, png.Header{
	Width: 2480, Height: 3508, ColorModel: color.RGBAModel, Opaque: true,
})
err = w.WriteRows(band) // band.Bounds() says which rows, e.g. image.Rect(0, 512, 2480, 768)
err = w.Close()         // after every row
```

Each band is compressed on its own (its first row uses no filter that
reads the row above, and its compressor starts empty), so no band waits
for another. That costs compression at the boundaries: bands of 256 rows
or more cost at most 0.4 % on the benchmark corpus, of 64 rows up to 3 %,
of 16 rows up to 13 %.

### Fast mode

`CompressionLevel: png.FastCompression` is calamus's own level, after
[fpng](https://github.com/richgel999/fpng): every row filtered with Up,
deflate matches only one pixel back, and prefix codes per block from the
symbols counted on the way. Against `image/png` at `BestSpeed`
([report](bench/reports/2026-10-08-amd-ryzen-7-5800h-with-radeon-graphics-windows-png-fast.md),
same machine and corpus as above):

| images | 1 worker: time | 16 workers: time | size |
|---|---|---|---|
| photos, test graphics, a diagram (500² and larger) | 0.38–0.45 | 0.09–0.27; a 500² photo 0.45 (one band) | 0.89–1.06 |
| noise | 0.22 | 0.08 | 1.00 |
| a rendered document page, a text-like page | 0.80–0.86 | 0.18–0.19 | **1.43–2.25** |

It is the faster choice for photos and graphics. It is not for text and
line art: their repeats lie further back than a pixel, so the files come
out up to twice as large for little gain in time; use `BestSpeed` there.
For a batch of images, one worker per image gets 1.17–2.49× image/png's
throughput at 16 goroutines.

### Animated PNG

`png.EncodeAll` writes an APNG: every frame is a zlib stream of its own
(the first in IDAT, the others in fdAT chunks), so frames are encoded
concurrently, and the bands of a large frame as above. Decoders without
APNG show the first frame. All frames share a colour type: the palette if
every frame has the same one, else RGB or RGBA (16-bit if every frame is).

```go
err := png.EncodeAll(w, &png.Animation{Frames: []png.Frame{
	{Image: f0, Delay: 40 * time.Millisecond},
	{Image: f1, Delay: 40 * time.Millisecond, Dispose: png.DisposeBackground, Blend: png.BlendOver},
}})
```

Tests decode every frame (wrapped as a still PNG) with `image/png` and
check the sequence numbers; Pillow read the animation.

### Speed

Measured on one machine (Ryzen 7 5800H, 8 cores, 16 threads, Windows)
over the [benchmark corpus](bench/README.md): ratios to `image/png` at the
same level (`BestSpeed`), time per image, lower is faster. The
[report](bench/reports/2026-10-07-amd-ryzen-7-5800h-with-radeon-graphics-windows.md) has every case, the method and the
raw ratios; it holds for that machine and corpus.

| images | 1 worker | 16 workers | size |
|---|---|---|---|
| real: photos, test graphics, document pages, diagrams (500² and larger) | 0.83–0.94 | 0.15–0.48; a 500² photo 0.92 (one band) | ±0.1 % |
| synthetic: text-like page, noise (A4 at 150 dpi) | 0.86–0.94 | 0.13–0.17 | ±0.1 % |
| a 64² photo | 0.58 | 0.61 (one band) | +0.1 % |

What the workers cost: 16 workers spend 0.85–1.36× image/png's CPU time
on one image. For many images at once, one worker per image is the better
use of the cores: 1.12–1.36× image/png's throughput at 16 goroutines,
where 16 workers on one image at a time reach 0.12–0.83×. Memory: with
one worker, rows stream into the compressor and the peak heap is
1.2–1.25× image/png's, the process high-water mark at par. With 16
workers, the bands waiting to be written and their compressors (each
primed with its neighbour's bytes) take 2.6–41× image/png's heap and up
to 1.9× its high-water mark on images of 800×500 and larger (the
report's memory tables).

### How

- **Bands, in parallel.** Each band of rows is filtered on its own (a row's
  filter needs only the previous row's raw pixels) and deflated as a raw
  stream ending in a sync flush, so the pieces concatenate into one zlib
  stream. Each band's compressor is primed with the last 32 KiB of the
  previous band's filtered bytes, so matches reach across the boundary as
  they would in one stream; the band filters those bytes itself and needs
  nothing from its neighbour. The Adler-32 is combined from the bands'
  (zlib's `adler32_combine`). The scheme is that of
  [pigz](https://zlib.net/pigz/) and [mtpng](https://github.com/bvibber/mtpng).
- **The same filters as `image/png`**: libpng's heuristic (the least sum of
  absolute differences), with a row equal to the previous one taking Up at
  once, as it would win anyway. Colour types are chosen as `image/png`
  chooses them; `*image.RGBA`, `*image.NRGBA`, `*image.Gray`, `*image.Gray16`,
  `*image.RGBA64`, `*image.NRGBA64` and `*image.Paletted` are read
  directly, other images through `At`.
- Tests check every colour type, band count and compression level against
  `image/png` (the same header chunks, the same filtered bytes, the same
  decoded pixels), and a fuzz test does so for random images.

### Not yet

- Each band allocates its own compressor (the standard library cannot
  reset one with a new dictionary): about 1 MB per band.
- A `Fast` mode after [fpng](https://github.com/richgel999/fpng): Up filter
  only, run-length matches, precomputed Huffman tables.
- Streaming: handing bands over as a renderer finishes them.

## JPEG

A drop-in for `image/jpeg` (4:2:0 baseline, grayscale for `*image.Gray`).
With one worker it writes the very bytes `image/jpeg` writes (Go 1.27;
the forward DCT is ported from it). With more, the scan is cut into bands
of MCU rows, each one restart interval: a restart marker resets the DC
predictions and byte-aligns the coder, so the bands encode independently
and join into one ordinary scan that every decoder reads.

```go
import "github.com/timzifer/calamus/jpeg"

err := jpeg.Encode(w, img, &jpeg.Options{Quality: 85}) // like image/jpeg's Encode
err = (&jpeg.Encoder{Quality: 85, Workers: 0}).Encode(w, img)
```

Against `image/jpeg` at quality 75 ([report](bench/reports/2026-10-07-amd-ryzen-7-5800h-with-radeon-graphics-windows.md), same machine
and corpus as for PNG): with one worker as fast (0.99–1.02 of its time);
with 16 workers 0.12–0.24 of its time on images of 500² and larger, at
1.2–1.6× its CPU time; the restart markers add up to 0.1 % to the file.
image/jpeg allocates almost nothing; with 16 workers calamus allocates
about the output's size on a first call and 1.6–7× image/jpeg's few
kilobytes once its buffers are pooled. For a batch of images, one worker
per image is as fast as image/jpeg on as many goroutines. Tests check that banded and unbanded
files decode to the same pixels (`image/jpeg`, and libjpeg-turbo through
Pillow for a sample), and a fuzz test does so for random images.

## TIFF

A drop-in for `golang.org/x/image/tiff`'s encoder: the same colour types
and tags, plus **LZW** and the horizontal **predictor** for Deflate and
LZW (x/image writes neither). Strips are compressed independently by
design, so they are compressed concurrently; the IFD follows them.

```go
import "github.com/timzifer/calamus/tiff"

err := tiff.Encode(w, img, &tiff.Options{Compression: tiff.Deflate, Predictor: true})
```

Against x/image/tiff with Deflate, time per image
([report](bench/reports/2026-10-06-amd-ryzen-7-5800h-with-radeon-graphics-windows-tiff-gif-webp.md), same machine and corpus): with
one worker 0.99–1.10 (pure noise 1.34); with 16 workers 0.14–0.51 on
images of 500² and larger, but 1.48 on pure noise and 1.07 on a 64² photo;
files up to 1.6 % larger. LZW and the predictor are other configurations,
compared in `go test -bench TIFFConfigurations` in [bench](bench/README.md).

Tests decode every colour type, compression, predictor and worker count
with x/image/tiff and compare the pixels; libtiff (through Pillow) read
all 30 combinations of a sample with several strips alike; the LZW coder
(MSB first, with TIFF's early change) has a round-trip fuzz test.

## GIF

A drop-in for `image/gif` (`Encode`, `EncodeAll`, the same `GIF` type).
Three things run concurrently:

- **frames** of an animation, each an LZW stream of its own;
- **bands** of a large frame: each band ends with a Clear code written at
  the width the decoder has reached, so the next band starts afresh, and
  the bands' bit streams are spliced without padding (GIF knows no byte
  alignment). With one band a frame's data is the very stream `image/gif`
  writes, and with one worker the file is byte for byte `image/gif`'s;
- **quantising** a true-colour image: Floyd-Steinberg dithering (the
  default) runs as a wavefront, row y on pixel x as soon as row y−1 has
  passed pixel x+1, with `image/draw`'s result to the bit; `draw.Src` in
  bands.

```go
import "github.com/timzifer/calamus/gif"

err := gif.EncodeAll(w, anim)  // like image/gif's EncodeAll
err = gif.Encode(w, photo, nil) // quantised to Plan 9 with Floyd-Steinberg, on all cores
```

Against `image/gif`, quantising to Plan 9 with Floyd-Steinberg and
encoding ([report](bench/reports/2026-10-06-amd-ryzen-7-5800h-with-radeon-graphics-windows-tiff-gif-webp.md), same machine and corpus):
with one worker as fast (0.99–1.05 of its time); with 16 workers 0.12–0.23
on images of 500² and larger, but 1.7 on a 64² photo; the same pixels,
files up to 0.6 % larger.

Tests check the bytes against `image/gif` with one worker, the decoded
frames with many, and the dithering against `image/draw` bit for bit;
Pillow read banded files alike; a fuzz test compares one and many workers.

## WebP

Lossless WebP (VP8L), still and animated. Go's standard library has no
WebP encoder; this one is written from the format (RFC 9649), and its
output is checked with golang.org/x/image/webp and libwebp. One frame is
encoded in bands too:

- the transforms (subtract green, predictor with the best of 14 modes per
  16×16 tile) work per pixel;
- LZ77 matching runs per band and may reach back into earlier bands,
  whose pixels the decoder has;
- the colour cache's state at each band's start comes from one quick
  serial pass, as it depends on the pixels alone;
- the bands' symbol counts give one set of prefix codes, and each band's
  symbols are coded on their own; the bit streams are spliced, as VP8L
  knows no byte alignment.

An animation's frames are encoded concurrently (each an `ANMF` chunk).

```go
import "github.com/timzifer/calamus/webp"

err := webp.Encode(w, img)
err = webp.EncodeAll(w, &webp.Animation{Frames: []webp.Frame{{Image: f0, Duration: 50 * time.Millisecond}}})
```

Against [nativewebp](https://github.com/HugoSmits86/nativewebp) v1.3.0
(lossless WebP in pure Go) at its default level
([report](bench/reports/2026-10-06-amd-ryzen-7-5800h-with-radeon-graphics-windows-tiff-gif-webp.md), same machine and corpus): with one
worker 0.26–0.62 of its time, with 16 workers 0.07–0.19 on images of 500²
and larger; files 2–8 % smaller on photos, graphics and diagrams, 29 % on
a text-like page, 1 % larger on a document page. calamus/webp has one fixed
effort, so these compare whole configurations. libwebp writes smaller
files still: it also uses a colour transform, region-wise prefix codes and
a costlier LZ77 search. Those are the next steps.

## Other formats

The same trick works wherever a format lets a stream be cut into pieces
that are coded independently and then concatenated:

- **JPEG (baseline)**: yes, and well (done, see above). Restart markers (`DRI`, then
  `RST0`–`RST7` every n MCU rows) reset the DC predictors and byte-align the
  Huffman coder, so bands of MCU rows can be colour-converted, transformed,
  quantised and coded in parallel and joined with the markers. With the
  standard Huffman tables of Annex K (as `image/jpeg` uses) no global pass
  is needed; every decoder reads restart markers. Cost: two bytes and some
  padding bits per band. The DCT dominates JPEG encoding, so it should
  scale close to the core count. Optimised Huffman tables would need a
  first parallel pass to count symbols. Progressive JPEG is more involved.
- **TIFF**: trivially; strips and tiles are compressed independently by
  design (done, see above).
- **GIF**: yes (done, see above). LZW cannot be primed, but bands can be
  cut with Clear codes; the palette is global, and error diffusion carries
  from row to row, which a wavefront keeps exact.
- **WebP**: lossless, yes (done, see above): the transforms are per pixel,
  the colour cache's state at a band's start is cheap to know, and the bit
  stream has no alignment to keep. Lossy WebP (VP8) predicts across
  macroblocks and allows at most eight token partitions; it is not here.

## Credits

The band scheme follows pigz (Mark Adler) and mtpng (Brooke Vibber, MIT).
The planned fast mode follows fpng (Rich Geldreich, public domain). No code
is taken from either; where some is, it will be named here.

## License

MIT
