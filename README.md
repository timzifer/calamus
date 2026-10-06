# calamus

A fast PNG encoder in pure Go: on all cores, no cgo, every GOOS/GOARCH
including `js/wasm`. Its output is an ordinary PNG that any decoder reads.

> Status: design. Nothing to import yet.

## Why

`image/png` encodes on one core and tries all five filters on every row,
even at `BestSpeed`. When rendering itself is fast, encoding dominates:
[cera](https://github.com/timzifer/cera)'s command line spends 61–88 % of
its time in `image/png` (cera#67).

## How

- **Bands, in parallel.** Each band of rows is filtered on its own (a row's
  filter needs only the previous row's raw pixels) and deflated as a raw
  stream ending in a sync flush, so the pieces concatenate into one zlib
  stream. Each band's compressor is primed with the last 32 KiB of the
  previous band's filtered bytes, which keeps the ratio. The Adler-32 is
  combined from the bands'. The scheme is the one of
  [pigz](https://zlib.net/pigz/) and [mtpng](https://github.com/bvibber/mtpng).
- **Default mode:** libpng's filter heuristic (least sum of absolute
  differences), a row equal to the previous one taking Up at once, flate.
  Output as small as `image/png`'s.
- **Fast mode:** [fpng](https://github.com/richgel999/fpng)'s tricks, per
  band: the Up filter only, run-length matches at distance 3 or 4, and
  precomputed Huffman tables. Somewhat larger files, much faster on each core.
- **Streaming.** Bands can be handed over as a renderer finishes them, so
  that rendering and encoding overlap.

A prototype of the band scheme, measured on cera's pages, ran 4.3–5.4×
faster than `image/png` at `BestSpeed` on 16 threads, at the same size.

## Planned API

```go
enc := calamus.Encoder{Workers: 0, Mode: calamus.Default} // 0 = all cores
err := enc.Encode(w, img) // *image.RGBA, *image.NRGBA, *image.Gray, *image.Paletted fast; any image.Image
```

## Credits

The band scheme follows pigz (Mark Adler) and mtpng (Brooke Vibber, MIT).
The fast mode follows fpng (Rich Geldreich, public domain). No code is
taken from either; where some is, it will be named here.

## License

MIT
