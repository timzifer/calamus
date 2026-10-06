# calamus benchmarks

A module of its own, so that the corpus and the libraries it is rendered
with (cera, figure, gogpu) never reach a program that only imports
calamus.

## Getting the corpus

```sh
git submodule update --init bench/testdata/libjxl   # libjxl test data, pinned
cd bench
go run ./cmd/corpus fetch                           # four PDFs, checked by SHA-256
go run ./cmd/corpus list                            # every fixture and its status
```

Nothing is downloaded while benchmarks run. A fixture whose input is
missing is skipped, and says how to get it.

## Running

```sh
go test -bench .                    # the short suite: one case per category
go test -bench . -corpus=full       # every fixture
go test -bench 'PNG/document'       # one format, one category
```

Before anything is timed, every implementation encodes each fixture once
and its output is decoded and checked: PNG and TIFF must give back the
input's pixels, GIF image/gif's, JPEG image/jpeg's (see
`internal/benchcase` for what is timed and what is not).

Results are compared as ratios to the reference named in each result
(`size/image-png`, …), never as absolute times: absolute times say more
about the machine than about the encoder.

## Latency, throughput, CPU and memory

`go test -bench` times one encode at a time. `cmd/measure` measures what
it cannot, each job in a fresh process with `GOMAXPROCS` set to its CPU
budget, the implementations taking turns:

- **one image** at budgets P = 1, 2, 4, …: time to encode it and the CPU
  time it took;
- **a batch** of independent images at the same budgets: the reference
  on P goroutines, calamus with one worker per image, with P workers one
  image at a time, and the mixes in between; throughput, CPU per image and
  time per image;
- **memory**: sampled peak heap, bytes allocated and the process's
  high-water mark, cold and warm, with a control that only holds the
  input; and animations written through a slow writer.

```sh
go run ./cmd/measure                      # short corpus, PNG and JPEG
go run ./cmd/measure -corpus=full -runs=7
go run ./cmd/measure -parts=memory -match=photo/large
```

It writes `reports/<date>-<cpu>-<os>.md` and `.json`: ratios only, with the
machine, the method, the dependencies and the corpus checksums, and every
run's ratios. A report holds for its machine and corpus.

## Configurations

Each benchmark names its reference; a ratio means nothing without both
sides and their settings. "workers" rows differ only in calamus's worker
count; "configuration" rows change what is encoded and are trade-offs,
not the effect of workers.

| benchmark | reference | compared | output contract | kind |
|---|---|---|---|---|
| `PNG/…/best-speed`, `…/default` | image/png at that level | calamus/png at the same named level, 1 and GOMAXPROCS workers | decodes to the input's pixels | workers |
| `JPEG` | image/jpeg, quality 75, 4:2:0 | calamus/jpeg, quality 75 | decodes to image/jpeg's pixels (MSE ≤ 1 for Go < 1.27) | workers |
| `TIFF` | x/image/tiff, Deflate | calamus/tiff, Deflate | decodes to the input's pixels | workers |
| `TIFFConfigurations` | calamus/tiff, Deflate | Deflate + predictor, LZW, LZW + predictor | decodes to the input's pixels | configuration |
| `GIF/…/floyd-steinberg`, `…/src` | image/gif, Plan 9 palette, that drawer | calamus/gif, same palette and drawer | decodes to image/gif's frame | workers (quantising and encoding) |
| `GIFPaletted` | image/gif on a pre-quantised frame | calamus/gif on the same frame | decodes to the same indices | workers (LZW and file only) |
| `WebP` | nativewebp v1.3.0, default level | nativewebp best speed and best compression; calamus/webp (fixed effort) | decodes to the input's 8-bit NRGBA | configuration |
| `CrossFormat` | image/png, default level | calamus/png, calamus/webp | lossless at 8 bits; 16-bit inputs skipped | configuration (format) |

Not compared, for want of an equivalent reference:

- **APNG**: no other Go encoder writes it; encoding each frame as a still
  PNG would measure frame compression, not an animation encoder.
- **Animated GIF and WebP**: `BenchmarkEncodeAll` in the gif package
  times generated animations; the corpus holds still images only.
- **libwebp**: native code; a comparison would have to state its effort,
  threads and binding, and is left out of this module.

## The corpus

`go run ./cmd/corpus list` prints the full table. Each category is
benchmarked at its own small, medium and large size.

| category | real or synthetic | inputs |
|---|---|---|
| photo | real | wesaturate (64², 500²: RGB, gray, alpha), raw.pixls (16-bit), flower (2268×1512, RGB and alpha) |
| graphic | real | wide-gamut test images (1000², RGB and alpha) |
| document | real | arXiv and pdf.js pages rendered by cera at 72, 150 and 300 dpi |
| diagram | real | lines, heatmap and a dark scatter plot drawn by figure at 400×250, 800×500 and 2400×1500 |
| synthetic | synthetic | text-like page, user interface, gradient with noise, flat colour, pure noise; 360×640, A4 at 150 and 300 dpi |

Every input is verified: files by their SHA-256, and every image by the
SHA-256 of its pixels (`corpus/pixels.go`), so that another version of
cera, figure or a generator cannot change what is measured unnoticed. A
deliberate change is recorded with `go run ./cmd/corpus pixels`, in a
commit of its own.

## Sources and licences

- **libjxl test data** ([github.com/libjxl/testdata](https://github.com/libjxl/testdata)),
  a git submodule, unmodified. Copyright The JPEG XL Project Authors,
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Its
  `external/` directories keep their sources' licences:
  - `external/wesaturate`: photos from wesaturate.com, CC0 1.0;
  - `external/raw.pixls`: photos from raw.pixls.us, CC0 1.0;
  - `external/wide-gamut-tests`: from
    [codelogic/wide-gamut-tests](https://github.com/codelogic/wide-gamut-tests),
    Apache 2.0.
- **PDFs** are fetched from their sources and not redistributed: pages
  of the [pdf.js](https://github.com/mozilla/pdf.js) test suite at a
  pinned revision, and arXiv papers 1706.03762v7 and 1512.03385v1 under
  the terms stated on arxiv.org. They are the documents
  [cera](https://github.com/timzifer/cera) pins for its own corpus.
- **Diagrams and synthetic images** are generated by this module (MIT).
