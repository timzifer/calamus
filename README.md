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
| `calamus/webp` | lossless WebP | see below | planned |

## PNG

A drop-in for `image/png`; the filtered scanlines are byte for byte those
`image/png` writes.

```go
import "github.com/timzifer/calamus/png"

err := png.Encode(w, img) // like image/png's Encode, on all cores

enc := png.Encoder{CompressionLevel: png.BestSpeed, Workers: 0} // 0 = GOMAXPROCS
err = enc.Encode(w, img)
```

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

30 frames of 640×480: 5.6× faster than encoding the frames one by one
with `image/png`. Tests decode every frame (wrapped as a still PNG) with
`image/png` and check the sequence numbers; Pillow read the animation.

### Speed

Against `image/png` at the same compression level (`BestSpeed`), 16
hardware threads (Ryzen 7 5800H), ratios only:

| images | 1 worker | 16 workers | size |
|---|---|---|---|
| PDF pages rendered by [cera](https://github.com/timzifer/cera): arXiv papers (16 pages) | 1.26× faster | 5.4× faster | +0.0 to +0.3 % |
| … pdf.js test files (36 pages) | 1.43× | 3.2× | −2.6 to +3.5 % |
| … technical drawings (8 pages) | 1.27× | 6.7× | −1.5 to +3.4 % |
| synthetic photo, A4 at 150 dpi | 1.1× | 7.0× | 0.0 % |
| synthetic user interface | 1.5× | 4.1× | +2.5 % |

(`go test -bench .` for the synthetic ones.) Small images are encoded as
one band, so there is nothing lost below a few hundred kilobytes.

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

Against `image/jpeg`, a synthetic photo at A4 and 150 dpi, 16 hardware
threads: **6.2× faster** with 16 workers, as fast with one; the restart
markers add 0.01–0.1 % to the file. Tests check that banded and unbanded
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

A synthetic photo at A4 and 150 dpi, against x/image/tiff with Deflate, 16
hardware threads:

| | time | size |
|---|---|---|
| Deflate, 1 worker | as fast | same |
| Deflate, 16 workers | 5.1× faster | +0.9 % |
| Deflate with predictor, 16 workers | | −54 % |
| LZW, 16 workers | 6.6× faster | +36 % |
| LZW with predictor, 16 workers | 11.7× faster | −41 % |

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

Against `image/gif`, 16 hardware threads:

| | 16 workers | size |
|---|---|---|
| animation, 30 frames of 640×480 | 6.0× faster | same |
| one frame of 2400×1800 | 3.1× faster | +0.02 % |
| A4 photo at 150 dpi, Floyd-Steinberg (default) | 6.5× faster | same |
| A4 photo at 150 dpi, `draw.Src` | 6.4× faster | same |

Tests check the bytes against `image/gif` with one worker, the decoded
frames with many, and the dithering against `image/draw` bit for bit;
Pillow read banded files alike; a fuzz test compares one and many workers.

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
- **WebP**: hard. Lossless WebP uses whole-image transforms, a colour cache
  and an entropy image; lossy WebP predicts across macroblocks and allows
  at most eight token partitions.

## Credits

The band scheme follows pigz (Mark Adler) and mtpng (Brooke Vibber, MIT).
The planned fast mode follows fpng (Rich Geldreich, public domain). No code
is taken from either; where some is, it will be named here.

## License

MIT
