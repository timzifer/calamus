# calamus measurements, 2026-10-07

Ratios to the reference only, one machine, the short corpus. Time, CPU and memory: below 1.00 is less than the reference. Throughput: above 1.00 is more. `~` marks a range across 1.00 (inconclusive). These results hold for this machine and corpus; ratios can shift on other processors and with the machine's load. Raw per-run ratios: [2026-10-07-amd-ryzen-7-5800h-with-radeon-graphics-windows.json](2026-10-07-amd-ryzen-7-5800h-with-radeon-graphics-windows.json).

## Machine and method

| | |
|---|---|
| CPU | AMD Ryzen 7 5800H with Radeon Graphics |
| cores | 16 logical, 8 physical |
| OS | windows/amd64 |
| Go | go1.27.1 |
| CPU budgets | GOMAXPROCS 1, 2, 4, 8, 16; none: GOMAXPROCS only; other processes may share the CPUs |
| GOGC | default (100) |
| process memory | peak working set (Windows) |
| runs | 5 per job; batches of 16 images |
| commit | c957d288dc4e6a725b555fdbd0236edae9175f9c |
| command | `G:\golang\measure.exe -v -out reports` |

- Every job runs in a fresh process with GOMAXPROCS set to its budget; no CPU affinity is set.
- Each implementation encodes once before timing (warm-up, output check); timings are of warmed, repeated encodes.
- Within a run the implementations take turns in a rotating order, so the machine's load falls on all alike.
- Latency: each encode is timed on its own (QueryPerformanceCounter on Windows, whose Go clock is too coarse for small images); a run is a loop of at least 200ms per implementation; its ratio is the median encode time over the reference's.
- CPU: process user+system time over the whole loop or batch, per image, over the reference's. Windows counts it in scheduler ticks (~15.6 ms), hence the long loops.
- Batch: a fixed number of independent copies of the image; configurations are workers×outer with workers×outer = budget; throughput is the reference's batch time over the configuration's.
- Memory: one encode per fresh process (cold) or after three encodes and one collection, which keeps sync.Pools filled (warm); heap in use (objects, including those not yet collected) sampled every 1 ms during the encode (a sampled peak, not an exact one), bytes allocated (runtime.MemStats), and the process's high-water mark, which includes the input and the Go runtime. Output goes to a writer that keeps nothing. Median of several processes.
- Summary: the median of the per-run ratios, with their range; a range across 1.00 is marked ~ (inconclusive).
- Only ratios are reported: times say more about the machine and its load than about the encoders.

Dependencies: github.com/timzifer/cera v0.4.0, github.com/timzifer/figure v0.14.0, github.com/timzifer/figure/backend/gg v0.12.0, github.com/HugoSmits86/nativewebp v1.3.0, golang.org/x/image v0.46.0.

## One image: latency and CPU

### PNG, relative to image/png (BestSpeed)

Time to encode one image with as many workers as the budget P allows (one at P=1); in brackets the CPU time it took. bands: whether calamus split the image at the largest budget. size: output bytes relative to the reference's, with one worker and with the largest budget's.

| fixture | kind | P=1 | P=2 | P=4 | P=8 | P=16 | bands | size |
|---|---|---|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.58 (0.51–0.72) [0.50 (0.40–0.71)] | 0.57 (0.49–0.65) [0.47 (0.35–0.62)] | 0.57 (0.48–0.61) [0.47 (0.36–0.71)] | 0.66 (0.60–0.70) [0.41 (0.39–0.60)] | 0.61 (0.59–0.69) [0.53 (0.29–0.92)] | no | 1.001, 1.001 |
| photo/medium/keong-macan-500 | real | 0.91 (0.90–1.05) ~ [0.93 (0.85–1.00)] | 0.89 (0.78–0.96) [1.00 (0.80–1.00)] | 0.92 (0.81–0.99) [0.88 (0.81–1.00)] | 0.94 (0.90–0.95) [0.93 (0.83–0.93)] | 0.92 (0.73–1.12) ~ [1.00 (0.63–1.09) ~] | yes | 1.000, 1.000 |
| photo/large/flower | real | 0.94 (0.88–0.99) [0.93 (0.87–1.00)] | 0.51 (0.47–0.53) [0.91 (0.88–0.97)] | 0.28 (0.24–0.30) [1.04 (0.92–1.12) ~] | 0.16 (0.15–0.17) [1.08 (0.96–1.16) ~] | 0.15 (0.14–0.17) [1.29 (1.06–1.42)] | yes | 1.000, 1.000 |
| graphic/medium/p3-color-bars | real | 0.93 (0.92–0.95) [0.93 (0.80–1.00)] | 0.50 (0.47–0.57) [0.96 (0.88–1.08) ~] | 0.31 (0.27–0.36) [0.98 (0.61–1.14) ~] | 0.30 (0.29–0.32) [1.09 (0.89–1.29) ~] | 0.29 (0.18–0.32) [1.02 (0.86–1.20) ~] | yes | 1.000, 1.001 |
| document/medium/attention-p3-150dpi | real | 0.84 (0.82–0.85) [0.82 (0.78–0.88)] | 0.49 (0.48–0.54) [0.90 (0.86–1.06) ~] | 0.33 (0.29–0.36) [0.92 (0.78–1.12) ~] | 0.23 (0.20–0.24) [1.04 (0.87–1.13) ~] | 0.21 (0.21–0.25) [1.21 (1.00–1.23)] | yes | 1.000, 1.001 |
| diagram/medium/lines-800x500 | real | 0.83 (0.82–0.84) [0.80 (0.69–1.00)] | 0.58 (0.42–0.66) [1.00 (0.84–1.12) ~] | 0.53 (0.42–0.63) [0.88 (0.85–1.11) ~] | 0.48 (0.45–0.58) [0.96 (0.77–1.03) ~] | 0.48 (0.46–0.52) [0.85 (0.79–0.97)] | yes | 1.000, 0.999 |
| synthetic/medium/page-1240x1754 | synthetic | 0.86 (0.81–0.87) [0.88 (0.82–0.88)] | 0.47 (0.46–0.48) [0.92 (0.77–0.96)] | 0.31 (0.29–0.36) [0.96 (0.93–1.21) ~] | 0.24 (0.23–0.29) [1.24 (1.08–1.91)] | 0.17 (0.15–0.18) [1.36 (1.25–1.44)] | yes | 1.000, 0.999 |
| synthetic/medium/noise-1240x1754 | synthetic | 0.94 (0.87–0.95) [0.92 (0.82–0.96)] | 0.51 (0.49–0.53) [0.97 (0.90–1.05) ~] | 0.28 (0.27–0.29) [1.04 (0.96–1.06) ~] | 0.20 (0.20–0.21) [1.11 (0.96–1.12) ~] | 0.13 (0.12–0.18) [1.13 (1.07–1.27)] | yes | 1.000, 1.000 |

### JPEG, relative to image/jpeg (quality 75)

Time to encode one image with as many workers as the budget P allows (one at P=1); in brackets the CPU time it took. bands: whether calamus split the image at the largest budget. size: output bytes relative to the reference's, with one worker and with the largest budget's.

| fixture | kind | P=1 | P=2 | P=4 | P=8 | P=16 | bands | size |
|---|---|---|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 1.00 (0.97–1.06) ~ [1.00 (0.75–1.33) ~] | 1.01 (0.97–1.03) ~ [1.00 (0.75–1.33) ~] | 1.00 (0.99–1.00) ~ [1.00 (1.00–1.33)] | 1.00 (0.98–1.01) ~ [1.00 (1.00–1.33)] | 1.00 (0.98–1.01) ~ [0.75 (0.75–1.00)] | no | 1.000, 1.000 |
| photo/medium/keong-macan-500 | real | 1.01 (1.00–1.03) ~ [1.08 (0.93–1.08) ~] | 0.53 (0.51–0.73) [1.08 (1.00–1.21)] | 0.34 (0.29–0.36) [1.16 (1.00–1.33)] | 0.23 (0.21–0.24) [1.34 (1.11–1.66)] | 0.23 (0.22–0.23) [1.24 (1.08–1.75)] | yes | 1.000, 1.001 |
| photo/large/flower | real | 1.01 (0.98–1.05) ~ [1.00 (0.92–1.00)] | 0.54 (0.50–0.56) [1.00 (0.96–1.08) ~] | 0.30 (0.28–0.31) [1.05 (1.03–1.12)] | 0.17 (0.17–0.18) [1.24 (1.17–1.27)] | 0.14 (0.13–0.16) [1.26 (1.22–1.40)] | yes | 1.000, 1.000 |
| graphic/medium/p3-color-bars | real | 1.00 (0.92–1.04) ~ [1.00 (0.93–1.15) ~] | 0.53 (0.51–0.61) [1.04 (1.03–1.10)] | 0.35 (0.29–0.39) [1.07 (0.98–1.16) ~] | 0.22 (0.21–0.26) [1.34 (1.07–1.52)] | 0.16 (0.15–0.18) [1.28 (0.96–1.44) ~] | yes | 1.000, 1.001 |
| document/medium/attention-p3-150dpi | real | 1.00 (0.99–1.02) ~ [1.08 (1.00–1.08)] | 0.52 (0.50–0.64) [1.08 (0.96–1.22) ~] | 0.35 (0.26–0.37) [1.15 (0.85–1.23) ~] | 0.19 (0.18–0.20) [1.28 (1.20–1.32)] | 0.13 (0.12–0.15) [1.38 (1.14–1.81)] | yes | 1.000, 1.000 |
| diagram/medium/lines-800x500 | real | 1.01 (0.99–1.06) ~ [1.07 (0.87–1.08) ~] | 0.54 (0.53–0.81) [1.04 (0.92–1.44) ~] | 0.30 (0.30–0.31) [1.13 (0.85–1.18) ~] | 0.22 (0.19–0.25) [1.39 (1.14–1.42)] | 0.24 (0.21–0.37) [1.64 (0.82–1.93) ~] | yes | 1.000, 1.001 |
| synthetic/medium/page-1240x1754 | synthetic | 0.99 (0.98–1.01) ~ [1.00 (0.94–1.07) ~] | 0.51 (0.49–0.56) [1.05 (1.00–1.07)] | 0.31 (0.27–0.35) [1.12 (0.90–1.16) ~] | 0.20 (0.18–0.21) [1.39 (1.25–1.55)] | 0.16 (0.15–0.16) [1.47 (1.27–1.84)] | yes | 1.000, 1.000 |
| synthetic/medium/noise-1240x1754 | synthetic | 1.02 (0.97–1.19) ~ [1.00 (0.93–1.07) ~] | 0.54 (0.53–0.64) [1.12 (1.00–1.22)] | 0.32 (0.27–0.36) [1.04 (0.93–1.16) ~] | 0.17 (0.16–0.23) [1.34 (1.04–1.51)] | 0.12 (0.11–0.14) [1.25 (1.18–1.52)] | yes | 1.000, 1.000 |

## A batch of images: throughput and CPU at equal budgets

Throughput of 16 independent images relative to the reference encoding them on P goroutines at once; in brackets CPU time per image, and the median time per image. Configurations are workers×outer: calamus-1xP encodes P images at a time with one worker each, calamus-Px1 one image at a time with P workers.

### PNG, P=1, relative to image/png on 1 goroutine

| fixture | kind | calamus-1x1 |
|---|---|---|
| photo/small/keong-macan-64 | real | 2.04 (1.92–2.14) [CPU 0.45 (0.36–0.56), image 0.51 (0.50–0.59)] |
| photo/medium/keong-macan-500 | real | 1.09 (1.04–1.13) [CPU 0.89 (0.84–0.94), image 0.91 (0.88–0.96)] |
| photo/large/flower | real | 1.09 (1.02–1.14) [CPU 0.94 (0.88–0.96), image 0.92 (0.89–0.98)] |
| graphic/medium/p3-color-bars | real | 1.13 (1.07–1.15) [CPU 0.86 (0.82–0.96), image 0.89 (0.86–0.95)] |
| document/medium/attention-p3-150dpi | real | 1.20 (1.16–1.40) [CPU 0.85 (0.70–0.89), image 0.83 (0.73–0.85)] |
| diagram/medium/lines-800x500 | real | 1.22 (0.97–1.42) ~ [CPU 0.77 (0.76–0.95), image 0.82 (0.76–1.01) ~] |
| synthetic/medium/page-1240x1754 | synthetic | 1.22 (1.13–1.26) [CPU 0.83 (0.81–0.85), image 0.82 (0.78–0.86)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.05 (1.02–1.08) [CPU 0.95 (0.93–0.97), image 0.96 (0.92–0.98)] |

### PNG, P=2, relative to image/png on 2 goroutines

| fixture | kind | calamus-1x2 | calamus-2x1 |
|---|---|---|---|
| photo/small/keong-macan-64 | real | 2.28 (1.96–2.58) [CPU 0.47 (0.39–0.60), image 0.36 (0.30–0.40)] | 1.23 (1.17–1.35) [CPU 0.50 (0.43–0.63), image 0.34 (0.30–0.35)] |
| photo/medium/keong-macan-500 | real | 1.12 (1.08–1.15) [CPU 0.91 (0.82–0.95), image 0.89 (0.85–0.96)] | 0.57 (0.55–0.63) [CPU 0.85 (0.84–0.91), image 0.87 (0.79–0.92)] |
| photo/large/flower | real | 1.07 (0.93–1.12) ~ [CPU 0.94 (0.91–1.01) ~, image 0.93 (0.89–1.08) ~] | 0.97 (0.96–1.02) ~ [CPU 0.97 (0.95–0.99), image 0.51 (0.49–0.52)] |
| graphic/medium/p3-color-bars | real | 1.13 (1.05–1.29) [CPU 0.85 (0.79–0.96), image 0.88 (0.76–0.92)] | 1.02 (0.78–1.08) ~ [CPU 0.94 (0.91–1.12) ~, image 0.49 (0.46–0.66)] |
| document/medium/attention-p3-150dpi | real | 1.19 (1.10–1.24) [CPU 0.84 (0.80–0.90), image 0.83 (0.80–0.90)] | 1.04 (1.00–1.21) ~ [CPU 0.85 (0.74–0.98), image 0.48 (0.41–0.51)] |
| diagram/medium/lines-800x500 | real | 1.26 (1.03–1.45) [CPU 0.81 (0.65–0.91), image 0.80 (0.72–0.97)] | 1.08 (0.92–1.18) ~ [CPU 0.79 (0.77–1.00), image 0.46 (0.44–0.52)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.16 (1.11–1.39) [CPU 0.85 (0.78–0.89), image 0.85 (0.71–0.87)] | 1.10 (1.02–1.14) [CPU 0.93 (0.84–1.00), image 0.47 (0.43–0.51)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.07 (1.05–1.09) [CPU 0.93 (0.92–0.94), image 0.94 (0.92–0.95)] | 1.00 (0.96–1.02) ~ [CPU 0.98 (0.96–1.00), image 0.50 (0.49–0.52)] |

### PNG, P=4, relative to image/png on 4 goroutines

| fixture | kind | calamus-1x4 | calamus-2x2 | calamus-4x1 |
|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 2.38 (2.13–2.61) [CPU 0.44 (0.39–0.57), image 0.32 (0.31–0.38)] | 1.35 (1.28–1.48) [CPU 0.47 (0.32–0.57), image 0.32 (0.29–0.32)] | 0.73 (0.70–0.84) [CPU 0.53 (0.42–0.62), image 0.31 (0.29–0.31)] |
| photo/medium/keong-macan-500 | real | 1.13 (1.06–1.27) [CPU 0.93 (0.79–1.02) ~, image 0.89 (0.78–0.93)] | 0.60 (0.57–0.66) [CPU 0.85 (0.77–0.91), image 0.85 (0.76–0.86)] | 0.30 (0.29–0.33) [CPU 0.88 (0.77–0.94), image 0.82 (0.75–0.88)] |
| photo/large/flower | real | 1.07 (0.96–1.18) ~ [CPU 0.91 (0.85–0.97), image 0.89 (0.84–0.94)] | 1.00 (0.70–1.10) ~ [CPU 0.92 (0.88–1.15) ~, image 0.49 (0.45–0.69)] | 1.02 (0.56–1.03) ~ [CPU 0.94 (0.92–1.19) ~, image 0.25 (0.24–0.45)] |
| graphic/medium/p3-color-bars | real | 1.18 (1.04–1.81) [CPU 0.90 (0.70–0.94), image 0.82 (0.62–0.95)] | 0.88 (0.60–1.67) ~ [CPU 0.93 (0.83–1.04) ~, image 0.51 (0.30–0.97)] | 0.98 (0.84–1.58) ~ [CPU 0.92 (0.86–1.02) ~, image 0.26 (0.17–0.27)] |
| document/medium/attention-p3-150dpi | real | 1.25 (1.18–1.51) [CPU 0.78 (0.68–0.87), image 0.82 (0.62–0.84)] | 1.11 (0.83–1.35) ~ [CPU 0.83 (0.66–0.86), image 0.47 (0.35–0.54)] | 0.90 (0.86–1.00) ~ [CPU 0.78 (0.66–0.99), image 0.29 (0.24–0.30)] |
| diagram/medium/lines-800x500 | real | 1.35 (1.08–1.38) [CPU 0.73 (0.70–1.00), image 0.77 (0.71–0.95)] | 1.13 (1.09–1.35) [CPU 0.77 (0.70–0.84), image 0.44 (0.36–0.47)] | 0.59 (0.57–0.72) [CPU 0.78 (0.70–0.85), image 0.43 (0.34–0.47)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.17 (1.12–1.25) [CPU 0.88 (0.80–0.95), image 0.88 (0.83–0.91)] | 1.12 (1.04–1.20) [CPU 0.86 (0.83–0.95), image 0.45 (0.44–0.51)] | 0.97 (0.89–1.03) ~ [CPU 0.88 (0.51–1.04) ~, image 0.26 (0.26–0.29)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.05 (1.04–1.09) [CPU 0.93 (0.91–0.96), image 0.94 (0.91–0.98)] | 1.03 (0.99–1.07) ~ [CPU 0.96 (0.91–1.00), image 0.48 (0.47–0.51)] | 0.95 (0.90–1.02) ~ [CPU 0.98 (0.90–1.00) ~, image 0.26 (0.25–0.28)] |

### PNG, P=8, relative to image/png on 8 goroutines

| fixture | kind | calamus-1x8 | calamus-2x4 | calamus-4x2 | calamus-8x1 |
|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 2.67 (2.07–2.91) [CPU 0.56 (0.50–0.81), image 0.27 (0.27–0.35)] | 1.79 (1.45–2.04) [CPU 0.44 (0.36–0.69), image 0.23 (0.20–0.31)] | 1.06 (0.83–1.17) ~ [CPU 0.35 (0.29–0.62), image 0.20 (0.19–0.25)] | 0.59 (0.50–0.65) [CPU 0.41 (0.31–0.47), image 0.20 (0.18–0.23)] |
| photo/medium/keong-macan-500 | real | 1.16 (1.14–2.50) [CPU 0.87 (0.84–0.95), image 0.86 (0.55–0.88)] | 0.63 (0.57–1.24) ~ [CPU 0.81 (0.81–0.93), image 0.80 (0.56–0.88)] | 0.33 (0.32–0.67) [CPU 0.80 (0.75–0.86), image 0.76 (0.52–0.80)] | 0.17 (0.17–0.36) [CPU 0.78 (0.75–0.83), image 0.73 (0.48–0.76)] |
| photo/large/flower | real | 1.07 (1.03–1.11) [CPU 0.93 (0.86–0.99), image 0.93 (0.90–0.98)] | 1.05 (1.02–1.06) [CPU 0.91 (0.88–0.99), image 0.48 (0.47–0.49)] | 1.02 (0.95–1.03) ~ [CPU 0.95 (0.91–1.01) ~, image 0.25 (0.24–0.27)] | 0.95 (0.94–0.96) [CPU 0.93 (0.92–1.00) ~, image 0.13 (0.13–0.14)] |
| graphic/medium/p3-color-bars | real | 1.09 (1.03–1.42) [CPU 0.91 (0.86–0.94), image 0.89 (0.77–0.95)] | 0.94 (0.84–1.26) ~ [CPU 0.94 (0.88–1.00), image 0.54 (0.43–0.55)] | 0.93 (0.77–1.22) ~ [CPU 0.92 (0.80–1.07) ~, image 0.27 (0.23–0.32)] | 0.52 (0.49–0.62) [CPU 0.88 (0.82–0.94), image 0.24 (0.21–0.26)] |
| document/medium/attention-p3-150dpi | real | 1.28 (0.97–1.30) ~ [CPU 0.81 (0.70–0.86), image 0.80 (0.77–0.90)] | 1.11 (0.66–1.15) ~ [CPU 0.74 (0.68–0.83), image 0.47 (0.43–0.53)] | 0.89 (0.88–0.91) [CPU 0.85 (0.80–0.90), image 0.29 (0.28–0.30)] | 0.79 (0.76–0.86) [CPU 0.84 (0.74–0.88), image 0.17 (0.15–0.17)] |
| diagram/medium/lines-800x500 | real | 1.31 (1.21–1.51) [CPU 0.76 (0.73–0.87), image 0.76 (0.68–0.90)] | 1.07 (1.05–1.10) [CPU 0.86 (0.79–0.95), image 0.46 (0.45–0.52)] | 0.63 (0.60–0.76) [CPU 0.73 (0.64–0.89), image 0.40 (0.31–0.43)] | 0.34 (0.33–0.43) [CPU 0.74 (0.65–0.78), image 0.37 (0.27–0.40)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.23 (1.11–1.51) [CPU 0.82 (0.77–0.83), image 0.82 (0.71–0.90)] | 1.09 (1.06–1.42) [CPU 0.87 (0.76–0.92), image 0.46 (0.38–0.49)] | 0.94 (0.88–1.23) ~ [CPU 0.85 (0.84–0.92), image 0.27 (0.22–0.29)] | 0.71 (0.68–0.90) [CPU 0.96 (0.87–1.12) ~, image 0.18 (0.15–0.19)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.08 (1.05–1.12) [CPU 0.92 (0.91–1.00), image 0.94 (0.92–0.98)] | 0.99 (0.96–1.10) ~ [CPU 0.98 (0.97–0.98), image 0.50 (0.47–0.54)] | 0.95 (0.91–1.05) ~ [CPU 0.98 (0.98–1.04) ~, image 0.27 (0.25–0.28)] | 0.72 (0.69–0.81) [CPU 0.99 (0.96–1.10) ~, image 0.18 (0.16–0.18)] |

### PNG, P=16, relative to image/png on 16 goroutines

| fixture | kind | calamus-1x16 | calamus-2x8 | calamus-4x4 | calamus-8x2 | calamus-16x1 |
|---|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 2.89 (2.48–3.00) [CPU 0.45 (0.37–2.17) ~, image 0.19 (0.18–0.21)] | 2.65 (2.52–2.79) [CPU 0.46 (0.26–2.08) ~, image 0.18 (0.17–0.18)] | 1.69 (1.52–1.78) [CPU 0.42 (0.39–1.08) ~, image 0.14 (0.13–0.17)] | 0.97 (0.88–1.05) ~ [CPU 0.40 (0.33–1.42) ~, image 0.13 (0.12–0.14)] | 0.52 (0.50–0.57) [CPU 0.51 (0.37–1.58) ~, image 0.12 (0.12–0.13)] |
| photo/medium/keong-macan-500 | real | 1.12 (0.87–1.17) ~ [CPU 0.88 (0.75–0.94), image 0.90 (0.83–1.10) ~] | 0.81 (0.72–0.88) [CPU 0.89 (0.79–0.96), image 0.71 (0.64–0.87)] | 0.44 (0.43–0.47) [CPU 0.81 (0.73–0.86), image 0.67 (0.61–0.76)] | 0.24 (0.23–0.28) [CPU 0.75 (0.65–0.78), image 0.59 (0.55–0.64)] | 0.12 (0.12–0.13) [CPU 0.73 (0.67–0.76), image 0.59 (0.53–0.64)] |
| photo/large/flower | real | 1.14 (1.03–1.19) [CPU 0.89 (0.83–0.92), image 0.92 (0.85–1.02) ~] | 1.08 (1.05–1.27) [CPU 0.92 (0.89–0.96), image 0.46 (0.44–0.52)] | 1.04 (0.97–1.20) ~ [CPU 0.93 (0.88–0.93), image 0.26 (0.25–0.27)] | 0.99 (0.92–1.25) ~ [CPU 0.94 (0.87–0.98), image 0.14 (0.13–0.15)] | 0.79 (0.73–1.00) [CPU 0.93 (0.83–1.02) ~, image 0.09 (0.08–0.09)] |
| graphic/medium/p3-color-bars | real | 1.14 (0.81–1.16) ~ [CPU 0.91 (0.78–1.03) ~, image 0.94 (0.85–1.10) ~] | 0.91 (0.89–1.12) ~ [CPU 1.02 (0.91–1.07) ~, image 0.55 (0.49–0.60)] | 0.93 (0.86–1.08) ~ [CPU 1.06 (1.04–1.22), image 0.30 (0.25–0.31)] | 0.68 (0.58–0.78) [CPU 0.93 (0.91–0.98), image 0.22 (0.20–0.24)] | 0.38 (0.34–0.51) [CPU 0.82 (0.73–0.96), image 0.19 (0.16–0.21)] |
| document/medium/attention-p3-150dpi | real | 1.26 (1.15–1.37) [CPU 0.78 (0.69–0.84), image 0.80 (0.75–0.88)] | 1.14 (1.08–1.17) [CPU 0.81 (0.72–0.91), image 0.49 (0.45–0.54)] | 1.00 (0.96–1.17) ~ [CPU 0.85 (0.77–0.94), image 0.28 (0.26–0.29)] | 0.87 (0.79–1.10) ~ [CPU 0.84 (0.70–0.92), image 0.16 (0.14–0.17)] | 0.71 (0.57–0.74) [CPU 0.72 (0.68–0.84), image 0.11 (0.11–0.12)] |
| diagram/medium/lines-800x500 | real | 1.36 (1.29–1.57) [CPU 0.73 (0.70–0.79), image 0.72 (0.68–0.73)] | 1.16 (1.10–1.28) [CPU 0.74 (0.72–0.81), image 0.46 (0.44–0.50)] | 0.86 (0.80–0.91) [CPU 0.67 (0.62–0.73), image 0.34 (0.31–0.35)] | 0.48 (0.45–0.59) [CPU 0.64 (0.51–0.67), image 0.30 (0.23–0.32)] | 0.26 (0.24–0.29) [CPU 0.61 (0.48–0.64), image 0.29 (0.22–0.31)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.16 (1.03–1.39) [CPU 0.74 (0.72–1.00), image 0.82 (0.77–0.87)] | 1.06 (0.91–1.42) ~ [CPU 0.82 (0.77–0.93), image 0.48 (0.41–0.51)] | 1.05 (0.99–1.31) ~ [CPU 0.85 (0.70–1.04) ~, image 0.27 (0.25–0.29)] | 0.82 (0.80–1.14) ~ [CPU 0.88 (0.77–1.07) ~, image 0.17 (0.16–0.18)] | 0.70 (0.68–0.99) [CPU 0.90 (0.72–0.99), image 0.10 (0.10–0.11)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.22 (1.07–1.26) [CPU 0.91 (0.84–0.94), image 0.89 (0.80–0.92)] | 1.22 (1.04–1.47) [CPU 0.92 (0.88–0.99), image 0.44 (0.37–0.48)] | 1.13 (0.91–1.36) ~ [CPU 1.00 (0.96–1.03) ~, image 0.25 (0.22–0.28)] | 0.93 (0.85–1.05) ~ [CPU 0.98 (0.91–1.07) ~, image 0.15 (0.13–0.17)] | 0.83 (0.79–0.95) [CPU 0.99 (0.98–1.04) ~, image 0.08 (0.08–0.11)] |

### JPEG, P=1, relative to image/jpeg on 1 goroutine

| fixture | kind | calamus-1x1 |
|---|---|---|
| photo/small/keong-macan-64 | real | 1.00 (0.98–1.02) ~ [CPU 1.00, image 0.99 (0.98–1.00)] |
| photo/medium/keong-macan-500 | real | 1.04 (0.96–1.05) ~ [CPU 0.94 (0.94–1.00), image 0.98 (0.97–0.99)] |
| photo/large/flower | real | 1.00 (0.90–1.03) ~ [CPU 1.00 (0.90–1.12) ~, image 0.99 (0.97–1.09) ~] |
| graphic/medium/p3-color-bars | real | 1.03 (0.97–1.07) ~ [CPU 0.93 (0.93–1.00), image 0.99 (0.96–1.05) ~] |
| document/medium/attention-p3-150dpi | real | 1.03 (0.96–1.06) ~ [CPU 1.00 (0.95–1.03) ~, image 0.99 (0.86–1.08) ~] |
| diagram/medium/lines-800x500 | real | 1.03 (1.00–1.05) ~ [CPU 1.06 (0.94–1.13) ~, image 0.99 (0.98–0.99)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.06 (0.95–1.15) ~ [CPU 0.97 (0.91–1.02) ~, image 0.95 (0.89–1.01) ~] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.02 (0.98–1.09) ~ [CPU 0.97 (0.92–0.99), image 0.98 (0.93–1.01) ~] |

### JPEG, P=2, relative to image/jpeg on 2 goroutines

| fixture | kind | calamus-1x2 | calamus-2x1 |
|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.98 (0.92–1.05) ~ [CPU 1.00 (0.86–1.17) ~, image 1.00 (0.99–1.04) ~] | 0.52 (0.51–0.54) [CPU 1.00 (0.86–1.17) ~, image 0.99 (0.97–1.01) ~] |
| photo/medium/keong-macan-500 | real | 1.05 (1.00–1.18) [CPU 0.96 (0.90–1.00), image 0.99 (0.85–1.01) ~] | 0.98 (0.93–1.01) ~ [CPU 1.00 (0.93–1.12) ~, image 0.53 (0.47–0.54)] |
| photo/large/flower | real | 1.01 (0.98–1.04) ~ [CPU 1.00 (0.95–1.04) ~, image 0.98 (0.96–1.03) ~] | 1.00 (0.98–1.10) ~ [CPU 1.00 (0.92–1.01) ~, image 0.50 (0.45–0.52)] |
| graphic/medium/p3-color-bars | real | 1.01 (0.90–1.03) ~ [CPU 1.03 (1.00–1.07), image 1.00 (0.96–1.09) ~] | 0.89 (0.87–0.93) [CPU 1.04 (0.96–1.12) ~, image 0.54 (0.53–0.58)] |
| document/medium/attention-p3-150dpi | real | 0.99 (0.91–1.03) ~ [CPU 1.00 (0.97–1.03) ~, image 1.01 (0.98–1.11) ~] | 0.92 (0.87–0.97) [CPU 1.03 (0.84–1.07) ~, image 0.53 (0.52–0.62)] |
| diagram/medium/lines-800x500 | real | 0.99 (0.83–1.02) ~ [CPU 0.97 (0.93–1.07) ~, image 1.00 (0.97–1.04) ~] | 0.95 (0.93–0.99) [CPU 0.93 (0.86–1.07) ~, image 0.53 (0.52–0.55)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.03 (0.98–1.25) ~ [CPU 1.00 (0.81–1.02) ~, image 0.96 (0.79–1.01) ~] | 0.96 (0.91–1.00) [CPU 1.02 (0.98–1.09) ~, image 0.54 (0.50–0.57)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.03 (0.96–1.19) ~ [CPU 0.99 (0.84–1.01) ~, image 0.97 (0.85–1.04) ~] | 0.94 (0.90–0.97) [CPU 1.04 (0.95–1.08) ~, image 0.54 (0.50–0.57)] |

### JPEG, P=4, relative to image/jpeg on 4 goroutines

| fixture | kind | calamus-1x4 | calamus-2x2 | calamus-4x1 |
|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.98 (0.91–1.13) ~ [CPU 1.00 (0.88–1.00), image 1.00 (0.97–1.01) ~] | 0.54 (0.49–0.63) [CPU 0.88 (0.75–1.14) ~, image 0.99 (0.94–1.03) ~] | 0.28 (0.26–0.32) [CPU 0.75 (0.62–0.86), image 0.98 (0.93–0.99)] |
| photo/medium/keong-macan-500 | real | 1.03 (0.99–1.13) ~ [CPU 1.00 (0.88–1.10) ~, image 0.99 (0.83–1.01) ~] | 0.95 (0.91–0.97) [CPU 1.03 (0.96–1.06) ~, image 0.54 (0.52–0.55)] | 0.92 (0.85–1.13) ~ [CPU 1.00 (0.77–1.15) ~, image 0.29 (0.21–0.29)] |
| photo/large/flower | real | 1.00 (0.89–1.15) ~ [CPU 1.01 (0.85–1.08) ~, image 1.00 (0.83–1.13) ~] | 0.98 (0.93–1.01) ~ [CPU 1.05 (0.94–1.19) ~, image 0.52 (0.50–0.55)] | 0.96 (0.91–0.99) [CPU 1.06 (1.01–1.07), image 0.27 (0.25–0.28)] |
| graphic/medium/p3-color-bars | real | 0.95 (0.83–1.01) ~ [CPU 1.07 (0.96–1.17) ~, image 1.03 (0.99–1.23) ~] | 0.91 (0.87–0.97) [CPU 1.06 (1.04–1.09), image 0.55 (0.51–0.61)] | 0.79 (0.77–0.89) [CPU 1.07 (0.96–1.11) ~, image 0.30 (0.28–0.35)] |
| document/medium/attention-p3-150dpi | real | 1.02 (0.97–1.04) ~ [CPU 1.00 (0.94–1.06) ~, image 0.98 (0.95–1.07) ~] | 1.00 (0.88–1.10) ~ [CPU 0.96 (0.88–1.09) ~, image 0.52 (0.44–0.60)] | 0.87 (0.82–0.96) [CPU 1.00 (0.95–1.10) ~, image 0.31 (0.26–0.32)] |
| diagram/medium/lines-800x500 | real | 1.01 (1.00–1.06) [CPU 0.97 (0.87–1.12) ~, image 0.97 (0.93–1.02) ~] | 0.94 (0.87–1.02) ~ [CPU 1.00 (0.92–1.12) ~, image 0.53 (0.50–0.64)] | 0.89 (0.83–0.96) [CPU 0.96 (0.85–1.08) ~, image 0.29 (0.26–0.34)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.01 (1.00–1.56) ~ [CPU 0.95 (0.67–1.03) ~, image 0.94 (0.65–1.01) ~] | 0.89 (0.65–1.32) ~ [CPU 1.09 (0.76–1.35) ~, image 0.56 (0.37–0.75)] | 0.93 (0.70–0.96) [CPU 0.95 (0.92–1.29) ~, image 0.28 (0.26–0.36)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.06 (0.90–1.32) ~ [CPU 1.05 (0.91–1.13) ~, image 0.99 (0.78–1.11) ~] | 1.03 (0.33–1.39) ~ [CPU 1.08 (0.89–1.37) ~, image 0.53 (0.38–0.92)] | 0.98 (0.90–1.30) ~ [CPU 0.96 (0.87–1.14) ~, image 0.27 (0.20–0.30)] |

### JPEG, P=8, relative to image/jpeg on 8 goroutines

| fixture | kind | calamus-1x8 | calamus-2x4 | calamus-4x2 | calamus-8x1 |
|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.92 (0.90–1.06) ~ [CPU 0.53 (0.44–2.00) ~, image 1.21 (0.96–1.45) ~] | 0.65 (0.62–0.71) [CPU 0.53 (0.44–8.00) ~, image 0.82 (0.63–0.94)] | 0.35 (0.30–0.43) [CPU 0.40 (0.38–6.00) ~, image 0.82 (0.62–0.93)] | 0.20 (0.19–0.22) [CPU 0.40 (0.38–7.00) ~, image 0.81 (0.61–0.92)] |
| photo/medium/keong-macan-500 | real | 1.02 (0.94–1.05) ~ [CPU 0.98 (0.94–1.03) ~, image 0.99 (0.93–1.01) ~] | 0.92 (0.74–1.02) ~ [CPU 1.00 (0.94–1.13) ~, image 0.54 (0.49–0.56)] | 0.87 (0.85–0.96) [CPU 1.02 (0.86–1.16) ~, image 0.28 (0.26–0.29)] | 0.73 (0.69–0.76) [CPU 1.09 (0.98–1.25) ~, image 0.17 (0.16–0.17)] |
| photo/large/flower | real | 1.00 (0.96–1.03) ~ [CPU 0.98 (0.97–0.98), image 1.01 (0.96–1.02) ~] | 1.00 (0.95–1.04) ~ [CPU 0.98 (0.92–1.01) ~, image 0.52 (0.48–0.52)] | 0.95 (0.94–0.98) [CPU 1.01 (0.95–1.04) ~, image 0.27 (0.26–0.27)] | 0.91 (0.90–0.93) [CPU 1.02 (0.97–1.05) ~, image 0.14 (0.14–0.14)] |
| graphic/medium/p3-color-bars | real | 1.01 (0.98–1.04) ~ [CPU 1.04 (0.99–1.09) ~, image 0.99 (0.97–1.03) ~] | 0.97 (0.93–0.99) [CPU 1.05 (0.91–1.14) ~, image 0.53 (0.53–0.54)] | 0.90 (0.83–0.92) [CPU 1.03 (0.94–1.09) ~, image 0.29 (0.28–0.30)] | 0.68 (0.64–0.74) [CPU 1.05 (1.00–1.30), image 0.18 (0.17–0.20)] |
| document/medium/attention-p3-150dpi | real | 0.99 (0.95–1.11) ~ [CPU 0.97 (0.92–1.05) ~, image 1.00 (0.92–1.05) ~] | 0.92 (0.90–1.05) ~ [CPU 1.02 (0.85–1.12) ~, image 0.55 (0.48–0.56)] | 0.88 (0.86–1.02) ~ [CPU 1.01 (0.89–1.08) ~, image 0.29 (0.25–0.30)] | 0.79 (0.77–0.94) [CPU 0.98 (0.86–1.14) ~, image 0.17 (0.14–0.17)] |
| diagram/medium/lines-800x500 | real | 1.03 (1.00–1.13) ~ [CPU 0.95 (0.95–1.02) ~, image 0.99 (0.95–1.02) ~] | 0.93 (0.86–1.12) ~ [CPU 1.05 (0.93–1.08) ~, image 0.55 (0.48–0.60)] | 0.88 (0.79–1.01) ~ [CPU 1.07 (0.94–1.12) ~, image 0.28 (0.26–0.31)] | 0.68 (0.66–0.95) [CPU 1.05 (0.93–1.18) ~, image 0.17 (0.14–0.18)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.06 (1.00–1.11) ~ [CPU 0.98 (0.97–0.99), image 0.93 (0.89–1.05) ~] | 1.04 (0.88–1.05) ~ [CPU 0.98 (0.96–1.12) ~, image 0.51 (0.47–0.56)] | 0.94 (0.90–1.07) ~ [CPU 1.02 (0.97–1.09) ~, image 0.27 (0.23–0.29)] | 0.80 (0.74–0.93) [CPU 1.11 (0.86–1.20) ~, image 0.15 (0.14–0.17)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.01 (0.86–1.03) ~ [CPU 1.09 (1.01–1.22), image 1.00 (0.96–1.16) ~] | 0.96 (0.80–0.98) [CPU 1.11 (1.00–1.15), image 0.52 (0.52–0.59)] | 0.95 (0.88–0.99) [CPU 1.12 (1.05–1.28), image 0.27 (0.25–0.29)] | 0.89 (0.88–0.91) [CPU 1.18 (0.81–1.21) ~, image 0.14 (0.14–0.15)] |

### JPEG, P=16, relative to image/jpeg on 16 goroutines

| fixture | kind | calamus-1x16 | calamus-2x8 | calamus-4x4 | calamus-8x2 | calamus-16x1 |
|---|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 1.03 (0.89–1.07) ~ [CPU 1.00 (0.48–1.40) ~, image 1.02 (0.98–1.03) ~] | 0.88 (0.66–0.91) [CPU 1.00 (0.53–8.00) ~, image 0.96 (0.80–0.99)] | 0.69 (0.56–0.71) [CPU 0.53 (0.43–8.00) ~, image 0.55 (0.54–0.55)] | 0.38 (0.34–0.41) [CPU 0.64 (0.29–6.00) ~, image 0.54 (0.53–0.55)] | 0.19 (0.17–0.22) [CPU 0.64 (0.33–7.00) ~, image 0.54 (0.53–0.54)] |
| photo/medium/keong-macan-500 | real | 1.02 (0.90–1.15) ~ [CPU 1.03 (0.98–1.16) ~, image 1.00 (0.97–1.03) ~] | 1.00 (0.92–1.19) ~ [CPU 1.00 (0.89–1.09) ~, image 0.57 (0.55–0.60)] | 0.97 (0.73–1.17) ~ [CPU 1.03 (0.97–1.10) ~, image 0.29 (0.28–0.38)] | 0.88 (0.83–1.00) [CPU 1.01 (0.99–1.02) ~, image 0.16 (0.16–0.17)] | 0.53 (0.47–0.57) [CPU 1.03 (0.79–1.04) ~, image 0.15 (0.14–0.15)] |
| photo/large/flower | real | 0.98 (0.79–1.09) ~ [CPU 0.97 (0.91–1.02) ~, image 0.99 (0.95–1.26) ~] | 0.94 (0.89–1.09) ~ [CPU 0.97 (0.92–1.03) ~, image 0.52 (0.47–0.56)] | 0.97 (0.89–1.07) ~ [CPU 1.00 (0.95–1.06) ~, image 0.27 (0.26–0.30)] | 0.99 (0.93–1.01) ~ [CPU 1.01 (0.94–1.04) ~, image 0.14 (0.14–0.15)] | 0.78 (0.74–0.85) [CPU 0.90 (0.85–1.05) ~, image 0.09 (0.08–0.09)] |
| graphic/medium/p3-color-bars | real | 0.98 (0.97–1.02) ~ [CPU 1.02 (0.90–1.19) ~, image 1.00 (0.97–1.03) ~] | 1.00 (0.97–1.09) ~ [CPU 1.04 (0.99–1.15) ~, image 0.53 (0.51–0.55)] | 0.96 (0.86–1.06) ~ [CPU 0.99 (0.90–1.29) ~, image 0.28 (0.27–0.29)] | 0.79 (0.78–0.91) [CPU 0.97 (0.86–1.09) ~, image 0.18 (0.17–0.18)] | 0.65 (0.63–0.73) [CPU 0.95 (0.90–1.23) ~, image 0.11 (0.10–0.11)] |
| document/medium/attention-p3-150dpi | real | 1.08 (0.86–1.25) ~ [CPU 1.00 (0.88–1.07) ~, image 1.01 (0.79–1.13) ~] | 1.05 (0.90–1.28) ~ [CPU 0.99 (0.95–1.04) ~, image 0.50 (0.42–0.60)] | 0.97 (0.84–1.23) ~ [CPU 1.00 (0.90–1.11) ~, image 0.29 (0.22–0.32)] | 0.98 (0.87–1.25) ~ [CPU 1.06 (0.91–1.14) ~, image 0.15 (0.12–0.17)] | 0.87 (0.84–1.09) ~ [CPU 0.99 (0.87–1.11) ~, image 0.08 (0.07–0.09)] |
| diagram/medium/lines-800x500 | real | 0.98 (0.95–1.11) ~ [CPU 1.01 (0.94–1.03) ~, image 1.01 (0.95–1.04) ~] | 0.95 (0.91–0.99) [CPU 0.96 (0.91–1.09) ~, image 0.60 (0.57–0.61)] | 0.96 (0.82–1.04) ~ [CPU 1.00 (0.97–1.03) ~, image 0.29 (0.29–0.35)] | 0.89 (0.76–0.99) [CPU 1.01 (0.92–1.05) ~, image 0.16 (0.15–0.19)] | 0.50 (0.43–0.59) [CPU 0.93 (0.72–1.16) ~, image 0.15 (0.13–0.16)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.02 (0.89–1.08) ~ [CPU 1.01 (1.00–1.06), image 0.98 (0.93–1.06) ~] | 1.04 (0.96–1.15) ~ [CPU 1.06 (0.91–1.17) ~, image 0.52 (0.50–0.53)] | 1.04 (0.93–1.09) ~ [CPU 1.07 (0.98–1.12) ~, image 0.28 (0.25–0.30)] | 1.04 (0.95–1.08) ~ [CPU 1.04 (0.98–1.11) ~, image 0.14 (0.14–0.15)] | 0.87 (0.82–0.91) [CPU 1.11 (0.95–1.28) ~, image 0.09 (0.08–0.09)] |
| synthetic/medium/noise-1240x1754 | synthetic | 0.99 (0.91–1.21) ~ [CPU 0.99 (0.91–0.99), image 1.03 (0.90–1.06) ~] | 1.04 (0.94–1.08) ~ [CPU 1.00 (0.92–1.07) ~, image 0.52 (0.50–0.56)] | 0.94 (0.84–1.01) ~ [CPU 0.99 (0.96–1.02) ~, image 0.29 (0.28–0.31)] | 1.00 (0.86–1.16) ~ [CPU 1.02 (0.96–1.05) ~, image 0.14 (0.12–0.16)] | 0.87 (0.77–1.02) ~ [CPU 0.95 (0.92–1.03) ~, image 0.08 (0.07–0.09)] |

## Memory

Each cell: sampled peak heap in use / bytes allocated / process high-water mark (peak working set (Windows)), relative to the baseline named; n/a where the baseline's is too small to sample. The heap in use counts objects not yet collected too, and its peak is sampled every millisecond, not exact. control holds the input and encodes nothing: its process ratio shows how much of the high-water mark is input and runtime.

### PNG

| fixture | kind | calamus-1 | calamus-16 | control | calamus-1/warm | calamus-16/warm |
|---|---|---|---|---|---|---|
| photo/medium/keong-macan-500 | real | 1.19 / 1.20 / 1.00 of ref | 1.19 / 1.20 / 0.99 of ref | process 0.99 of ref | 1.20 / 1.20 / 0.94 of ref/warm | 1.20 / 1.20 / 0.94 of ref/warm |
| photo/large/flower | real | 1.25 / 1.20 / 1.00 of ref | 40.76 / 38.14 / 1.36 of ref | process 1.00 of ref | 1.25 / 1.20 / 0.98 of ref/warm | 39.96 / 37.63 / 1.73 of ref/warm |
| graphic/medium/p3-color-bars | real | 1.22 / 1.20 / 1.00 of ref | 5.91 / 5.85 / 1.08 of ref | process 0.99 of ref | 1.23 / 1.20 / 0.98 of ref/warm | 5.42 / 5.60 / 1.21 of ref/warm |
| document/medium/attention-p3-150dpi | real | 1.20 / 1.20 / 1.00 of ref | 14.67 / 13.98 / 1.00 of ref | process 1.00 of ref | 0.18 / 0.20 / 1.00 of ref/warm | 14.23 / 13.93 / 1.01 of ref/warm |
| diagram/medium/lines-800x500 | real | 1.20 / 1.20 / 1.01 of ref | 2.61 / 2.60 / 1.01 of ref | process 1.01 of ref | 1.20 / 1.20 / 0.99 of ref/warm | 2.62 / 2.59 / 1.04 of ref/warm |
| synthetic/medium/page-1240x1754 | synthetic | 1.20 / 1.20 / 1.00 of ref | 15.89 / 15.14 / 1.16 of ref | process 1.00 of ref | 1.20 / 1.20 / 1.00 of ref/warm | 13.98 / 14.95 / 1.61 of ref/warm |
| synthetic/medium/noise-1240x1754 | synthetic | 1.20 / 1.20 / 1.00 of ref | 37.67 / 39.69 / 1.74 of ref | process 1.00 of ref | 1.21 / 1.20 / 0.99 of ref/warm | 37.00 / 39.85 / 1.94 of ref/warm |

### JPEG

| fixture | kind | calamus-1 | calamus-16 | control | calamus-1/warm | calamus-16/warm |
|---|---|---|---|---|---|---|
| photo/medium/keong-macan-500 | real | n/a / 0.99 / 1.00 of ref | n/a / 6.51 / 1.00 of ref | process 0.99 of ref | n/a / 0.99 / 0.99 of ref/warm | n/a / 2.85 / 1.03 of ref/warm |
| photo/large/flower | real | n/a / 1.00 / 1.00 of ref | n/a / 52.04 / 1.00 of ref | process 1.00 of ref | n/a / 1.00 / 1.00 of ref/warm | n/a / 2.41 / 1.03 of ref/warm |
| graphic/medium/p3-color-bars | real | n/a / 1.00 / 1.00 of ref | n/a / 13.02 / 1.00 of ref | process 1.00 of ref | n/a / 1.00 / 1.00 of ref/warm | n/a / 2.32 / 1.02 of ref/warm |
| document/medium/attention-p3-150dpi | real | n/a / 1.00 / 1.00 of ref | n/a / 36.66 / 1.00 of ref | process 1.00 of ref | n/a / 1.00 / 1.00 of ref/warm | n/a / 7.05 / 1.00 of ref/warm |
| diagram/medium/lines-800x500 | real | n/a / 0.99 / 1.00 of ref | n/a / 6.76 / 1.01 of ref | process 1.00 of ref | n/a / 1.00 / 1.00 of ref/warm | n/a / 1.60 / 1.00 of ref/warm |
| synthetic/medium/page-1240x1754 | synthetic | n/a / 1.01 / 1.00 of ref | n/a / 152.78 / 1.00 of ref | process 1.00 of ref | n/a / 1.00 / 1.00 of ref/warm | n/a / 2.52 / 1.06 of ref/warm |
| synthetic/medium/noise-1240x1754 | synthetic | n/a / 1.01 / 1.00 of ref | n/a / 255.00 / 1.00 of ref | process 1.00 of ref | n/a / 0.99 / 1.00 of ref/warm | n/a / 2.21 / 1.08 of ref/warm |

## Animations with a slow writer

A generated animation of 60 frames of 640×480, written through a writer limited to 8 MB/s, against a writer that keeps nothing: sampled peak live heap and process high-water mark, slow over fast for the same encoder; and the slow writer's heap peak against the baseline's with the same slow writer.

| format | encoder | heap slow/fast | process slow/fast | heap against baseline (slow) |
|---|---|---|---|---|
| gif-anim | calamus-1 | 0.99 | 1.00 | 2.58 of ref |
| gif-anim | calamus-16 | 1.02 | 1.00 | 2.50 of ref |
| gif-anim | ref | 1.00 | 1.00 | 1.00 of ref |
| apng-anim | calamus-1 | 1.05 | 1.01 | 1.00 of calamus-1 |
| apng-anim | calamus-16 | 0.91 | 0.99 | 1.55 of calamus-1 |

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
