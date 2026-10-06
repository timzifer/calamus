# calamus

Image encoders in pure Go that use all cores: each cuts the image into
bands, encodes them concurrently and joins them into one ordinary file
that any decoder reads. No cgo, every GOOS/GOARCH including `js/wasm`.

| package | format | how it splits | status |
|---|---|---|---|
| `calamus/png` | PNG | bands of one zlib stream, each primed with its neighbour's last 32 KiB | ready |
| `calamus/jpeg` | baseline JPEG | bands of MCU rows between restart markers | ready |
| `calamus/tiff` | TIFF | strips, compressed independently by design | planned |
| `calamus/gif` | GIF | LZW bands, each starting with a clear code | planned |
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
  design.
- **GIF**: partly. LZW cannot be primed, but each band can start with a
  clear code (as [gip](https://github.com/tenox7/gip) does). The palette is
  global: quantisation must come first, over the whole image.
- **WebP**: hard. Lossless WebP uses whole-image transforms, a colour cache
  and an entropy image; lossy WebP predicts across macroblocks and allows
  at most eight token partitions.

## Credits

The band scheme follows pigz (Mark Adler) and mtpng (Brooke Vibber, MIT).
The planned fast mode follows fpng (Rich Geldreich, public domain). No code
is taken from either; where some is, it will be named here.

## License

MIT
