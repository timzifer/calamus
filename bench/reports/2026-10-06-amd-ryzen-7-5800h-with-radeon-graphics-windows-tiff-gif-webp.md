# calamus measurements, 2026-10-06

Ratios to the reference only, one machine, the short corpus. Time, CPU and memory: below 1.00 is less than the reference. Throughput: above 1.00 is more. `~` marks a range across 1.00 (inconclusive). These results hold for this machine and corpus; ratios can shift on other processors and with the machine's load. Raw per-run ratios: [2026-10-06-amd-ryzen-7-5800h-with-radeon-graphics-windows-tiff-gif-webp.json](2026-10-06-amd-ryzen-7-5800h-with-radeon-graphics-windows-tiff-gif-webp.json).

## Machine and method

| | |
|---|---|
| CPU | AMD Ryzen 7 5800H with Radeon Graphics |
| cores | 16 logical, 8 physical |
| OS | windows/amd64 |
| Go | go1.27.1 |
| CPU budgets | GOMAXPROCS 1, 16; none: GOMAXPROCS only; other processes may share the CPUs |
| GOGC | default (100) |
| process memory | peak working set (Windows) |
| runs | 5 per job; batches of 16 images |
| commit | a2795c34aadb11f1ba597e49a8a0bf212625788f (modified) |
| command | `G:\golang\measure.exe -v -formats tiff,gif,webp -parts latency -budgets 1,16 -name tiff-gif-webp -out reports` |

- Every job runs in a fresh process with GOMAXPROCS set to its budget; no CPU affinity is set.
- Each implementation encodes once before timing (warm-up, output check); timings are of warmed, repeated encodes.
- Within a run the implementations take turns in a rotating order, so the machine's load falls on all alike.
- Latency: each encode is timed on its own (QueryPerformanceCounter on Windows, whose Go clock is too coarse for small images); a run is a loop of at least 200ms per implementation; its ratio is the median encode time over the reference's.
- CPU: process user+system time over the whole loop or batch, per image, over the reference's. Windows counts it in scheduler ticks (~15.6 ms), hence the long loops.
- Batch: a fixed number of independent copies of the image; configurations are workers×outer with workers×outer = budget; throughput is the reference's batch time over the configuration's.
- Memory: one encode per fresh process (cold) or after three encodes (warm); heap in use (objects, including those not yet collected) sampled every 1 ms during the encode (a sampled peak, not an exact one), bytes allocated (runtime.MemStats), and the process's high-water mark, which includes the input and the Go runtime. Output goes to a writer that keeps nothing. Median of several processes.
- Summary: the median of the per-run ratios, with their range; a range across 1.00 is marked ~ (inconclusive).
- Only ratios are reported: times say more about the machine and its load than about the encoders.

Dependencies: github.com/timzifer/cera v0.4.0, github.com/timzifer/figure v0.14.0, github.com/timzifer/figure/backend/gg v0.12.0, github.com/HugoSmits86/nativewebp v1.3.0, golang.org/x/image v0.46.0.

## One image: latency and CPU

### TIFF, relative to golang.org/x/image/tiff (Deflate)

Time to encode one image with as many workers as the budget P allows (one at P=1); in brackets the CPU time it took. bands: whether calamus split the image at the largest budget. size: output bytes relative to the reference's, with one worker and with the largest budget's.

| fixture | kind | P=1 | P=16 | bands | size |
|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.99 (0.81–1.15) ~ [1.10 (1.00–1.11)] | 1.07 (0.98–1.50) ~ [0.85 (0.58–1.33) ~] | - | 1.000, 1.000 |
| photo/medium/keong-macan-500 | real | 1.02 (1.01–1.02) [1.08 (0.92–1.09) ~] | 0.50 (0.47–0.62) [1.18 (0.96–1.58) ~] | - | 1.000, 1.003 |
| photo/large/flower | real | 1.01 (0.96–1.05) ~ [1.00 (0.93–1.07) ~] | 0.14 (0.14–0.15) [1.38 (1.23–1.82)] | - | 1.000, 1.005 |
| graphic/medium/p3-color-bars | real | 1.01 (0.98–1.02) ~ [1.00 (0.93–1.08) ~] | 0.20 (0.19–0.21) [1.54 (1.52–1.67)] | - | 1.000, 1.005 |
| document/medium/attention-p3-150dpi | real | 1.10 (1.08–1.12) [1.07 (1.07–1.15)] | 0.33 (0.29–0.34) [2.69 (2.04–3.51)] | - | 1.000, 1.016 |
| diagram/medium/lines-800x500 | real | 1.08 (1.05–1.08) [1.08 (1.00–1.17)] | 0.51 (0.50–0.56) [2.04 (1.78–2.19)] | - | 1.000, 1.011 |
| synthetic/medium/page-1240x1754 | synthetic | 1.05 (1.02–1.08) [1.07 (1.00–1.15)] | 0.25 (0.22–0.28) [2.51 (1.92–2.78)] | - | 1.000, 1.008 |
| synthetic/medium/noise-1240x1754 | synthetic | 1.34 (1.31–1.37) [1.44 (1.22–1.62)] | 1.48 (1.43–1.50) [11.44 (8.85–15.56)] | - | 1.000, 1.000 |

### GIF, relative to image/gif (Plan 9 palette, Floyd-Steinberg)

Time to encode one image with as many workers as the budget P allows (one at P=1); in brackets the CPU time it took. bands: whether calamus split the image at the largest budget. size: output bytes relative to the reference's, with one worker and with the largest budget's.

| fixture | kind | P=1 | P=16 | bands | size |
|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.99 (0.98–1.04) ~ [1.08 (0.93–1.17) ~] | 1.70 (1.57–1.73) [16.14 (13.80–19.83)] | - | 1.000, 1.000 |
| photo/medium/keong-macan-500 | real | 1.01 (1.00–1.15) ~ [1.06 (0.95–1.17) ~] | 0.23 (0.23–0.27) [2.88 (2.52–3.45)] | - | 1.000, 1.000 |
| photo/large/flower | real | 1.00 (0.97–1.04) ~ [0.99 (0.96–1.03) ~] | 0.12 (0.12–0.12) [1.64 (1.60–1.71)] | - | 1.000, 1.000 |
| graphic/medium/p3-color-bars | real | 0.99 (0.96–1.13) ~ [1.03 (0.90–1.16) ~] | 0.13 (0.13–0.15) [1.80 (1.61–1.96)] | - | 1.000, 1.000 |
| document/medium/attention-p3-150dpi | real | 1.05 (0.88–1.15) ~ [1.03 (0.89–1.14) ~] | 0.12 (0.12–0.13) [1.67 (1.52–1.73)] | - | 1.000, 1.006 |
| diagram/medium/lines-800x500 | real | 0.99 (0.98–1.01) ~ [1.00 (0.93–1.00)] | 0.17 (0.15–0.19) [2.24 (2.17–2.51)] | - | 1.000, 1.000 |
| synthetic/medium/page-1240x1754 | synthetic | 0.99 (0.94–1.01) ~ [1.00 (0.94–1.01) ~] | 0.12 (0.12–0.13) [1.70 (1.67–1.74)] | - | 1.000, 1.001 |
| synthetic/medium/noise-1240x1754 | synthetic | 0.99 (0.99–1.04) ~ [1.02 (0.97–1.08) ~] | 0.12 (0.12–0.14) [1.49 (1.47–1.71)] | - | 1.000, 1.000 |

### WEBP, relative to nativewebp v1.3.0 (lossless; nativewebp at its default level, calamus/webp at its fixed effort)

Time to encode one image with as many workers as the budget P allows (one at P=1); in brackets the CPU time it took. bands: whether calamus split the image at the largest budget. size: output bytes relative to the reference's, with one worker and with the largest budget's.

| fixture | kind | P=1 | P=16 | bands | size |
|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.44 (0.41–0.47) [0.45 (0.45–0.50)] | 0.35 (0.32–0.36) [0.59 (0.29–0.71)] | - | 0.968, 0.968 |
| photo/medium/keong-macan-500 | real | 0.61 (0.59–0.73) [0.60 (0.56–0.75)] | 0.19 (0.17–0.20) [0.84 (0.79–0.98)] | - | 0.973, 0.973 |
| photo/large/flower | real | 0.51 (0.49–0.53) [0.51 (0.50–0.54)] | 0.10 (0.09–0.10) [0.93 (0.91–1.06) ~] | - | 0.969, 0.969 |
| graphic/medium/p3-color-bars | real | 0.57 (0.57–0.59) [0.60 (0.57–0.62)] | 0.11 (0.10–0.13) [0.79 (0.73–1.21) ~] | - | 0.917, 0.917 |
| document/medium/attention-p3-150dpi | real | 0.30 (0.27–0.33) [0.30 (0.26–0.33)] | 0.07 (0.06–0.07) [0.48 (0.36–0.65)] | - | 1.011, 1.011 |
| diagram/medium/lines-800x500 | real | 0.26 (0.25–0.29) [0.26 (0.26–0.31)] | 0.07 (0.07–0.09) [0.49 (0.33–0.55)] | - | 0.976, 0.977 |
| synthetic/medium/page-1240x1754 | synthetic | 0.37 (0.37–0.40) [0.38 (0.37–0.39)] | 0.07 (0.06–0.08) [0.62 (0.59–0.79)] | - | 0.709, 0.709 |
| synthetic/medium/noise-1240x1754 | synthetic | 0.62 (0.59–0.69) [0.63 (0.60–0.70)] | 0.14 (0.14–0.15) [1.56 (1.40–1.58)] | - | 0.998, 0.998 |

## Corpus

| fixture | kind | file SHA-256 | pixel SHA-256 |
|---|---|---|---|
| photo/small/keong-macan-64 | real | 00b23ff5b144 | 4585b0751b8a |
| photo/medium/keong-macan-500 | real | eb9189de8004 | 3b197a6e7a7e |
| photo/large/flower | real | 77ea5436547c | 1060f0738d8b |
| graphic/medium/p3-color-bars | real | 55af57a76b2e | 7ee573104c38 |
| document/medium/attention-p3-150dpi | real | bdfaa68d8984 | 5eed691b8a1f |
| diagram/medium/lines-800x500 | real | - | 78c2f5344463 |
| synthetic/medium/page-1240x1754 | synthetic | - | f0370477fdfb |
| synthetic/medium/noise-1240x1754 | synthetic | - | 8ebd8fe2b32c |
