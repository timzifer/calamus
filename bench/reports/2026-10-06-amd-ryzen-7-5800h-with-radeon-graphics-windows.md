# calamus measurements, 2026-10-06

Ratios to the reference only, one machine, the short corpus. Time, CPU and memory: below 1.00 is less than the reference. Throughput: above 1.00 is more. `~` marks a range across 1.00 (inconclusive). These results hold for this machine and corpus; ratios can shift on other processors and with the machine's load. Raw per-run ratios: [2026-10-06-amd-ryzen-7-5800h-with-radeon-graphics-windows.json](2026-10-06-amd-ryzen-7-5800h-with-radeon-graphics-windows.json).

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
| commit | cf08aedbe2a3606f48e7cf39de99d6d86a26841d |
| command | `G:\golang\measure.exe -v -out reports` |

- Every job runs in a fresh process with GOMAXPROCS set to its budget; no CPU affinity is set.
- Each implementation encodes once before timing (warm-up, output check); timings are of warmed, repeated encodes.
- Within a run the implementations take turns in a rotating order, so the machine's load falls on all alike.
- Latency: each encode is timed on its own (QueryPerformanceCounter on Windows, whose Go clock is too coarse for small images); a run is a loop of at least 200ms per implementation; its ratio is the median encode time over the reference's.
- CPU: process user+system time over the whole loop or batch, per image, over the reference's. Windows counts it in scheduler ticks (~15.6 ms), hence the long loops.
- Batch: a fixed number of independent copies of the image; configurations are workers×outer with workers×outer = budget; throughput is the reference's batch time over the configuration's.
- Memory: one encode per fresh process (cold) or after three encodes (warm); heap in use (objects, including those not yet collected) sampled every 1 ms during the encode (a sampled peak, not an exact one), bytes allocated (runtime.MemStats), and the process's high-water mark, which includes the input and the Go runtime. Output goes to a writer that keeps nothing. Median of several processes.
- Summary: the median of the per-run ratios, with their range; a range across 1.00 is marked ~ (inconclusive).
- Only ratios are reported: times say more about the machine and its load than about the encoders.

Dependencies: github.com/timzifer/cera v0.4.0, github.com/timzifer/figure v0.14.0, github.com/timzifer/figure/backend/gg v0.12.0, golang.org/x/image v0.46.0.

## One image: latency and CPU

### PNG, relative to image/png (BestSpeed)

Time to encode one image with as many workers as the budget P allows (one at P=1); in brackets the CPU time it took. bands: whether calamus split the image at the largest budget. size: output bytes relative to the reference's, with one worker and with the largest budget's.

| fixture | kind | P=1 | P=2 | P=4 | P=8 | P=16 | bands | size |
|---|---|---|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.93 (0.88–1.14) ~ [0.92 (0.91–1.00)] | 1.00 (0.89–1.16) ~ [1.00 (0.70–1.20) ~] | 1.11 (0.81–1.13) ~ [1.00 (0.67–1.06) ~] | 1.01 (0.97–1.08) ~ [1.05 (0.83–1.12) ~] | 1.02 (0.96–1.08) ~ [1.20 (0.62–1.67) ~] | no | 1.001, 1.001 |
| photo/medium/keong-macan-500 | real | 0.93 (0.92–0.96) [0.93 (0.87–0.93)] | 0.93 (0.80–0.94) [0.93 (0.75–1.00)] | 0.93 (0.91–0.94) [1.00 (0.87–1.06) ~] | 0.91 (0.87–0.96) [0.93 (0.75–1.00)] | 0.93 (0.84–0.94) [0.93 (0.88–1.00)] | no | 1.000, 1.000 |
| photo/large/flower | real | 0.93 (0.91–0.94) [0.93 (0.87–1.00)] | 0.49 (0.46–0.54) [0.97 (0.88–1.03) ~] | 0.28 (0.27–0.29) [1.02 (1.00–1.08)] | 0.16 (0.16–0.18) [1.11 (1.08–1.32)] | 0.14 (0.14–0.16) [1.13 (0.97–1.34) ~] | yes | 1.000, 1.000 |
| graphic/medium/p3-color-bars | real | 0.90 (0.88–0.93) [0.88 (0.88–0.94)] | 0.50 (0.48–0.59) [0.93 (0.89–1.06) ~] | 0.31 (0.28–0.31) [0.98 (0.92–1.04) ~] | 0.29 (0.25–0.32) [0.95 (0.95–1.12) ~] | 0.30 (0.26–0.31) [1.08 (0.93–1.23) ~] | yes | 1.000, 1.001 |
| document/medium/attention-p3-150dpi | real | 0.85 [0.82 (0.76–0.88)] | 0.52 (0.49–0.52) [0.92 (0.91–1.00)] | 0.32 (0.31–0.35) [0.90 (0.74–1.00)] | 0.22 (0.19–0.23) [1.19 (0.85–1.25) ~] | 0.22 (0.18–0.25) [1.18 (0.91–1.69) ~] | yes | 1.000, 1.001 |
| diagram/medium/lines-800x500 | real | 0.86 (0.84–0.87) [0.87 (0.80–0.93)] | 0.49 (0.47–0.68) [1.00 (0.80–1.06) ~] | 0.50 (0.46–0.70) [0.97 (0.80–1.07) ~] | 0.52 (0.47–0.67) [0.97 (0.79–1.08) ~] | 0.48 (0.47–0.65) [0.95 (0.83–1.06) ~] | yes | 1.000, 0.999 |
| synthetic/medium/page-1240x1754 | synthetic | 0.88 (0.85–0.88) [0.88 (0.81–0.93)] | 0.48 (0.45–0.54) [0.89 (0.86–1.09) ~] | 0.30 (0.27–0.34) [1.00 (0.79–1.18) ~] | 0.22 (0.20–0.24) [1.20 (0.96–1.40) ~] | 0.17 (0.16–0.20) [1.26 (1.19–1.57)] | yes | 1.000, 0.999 |
| synthetic/medium/noise-1240x1754 | synthetic | 0.94 (0.88–0.94) [0.92 (0.88–0.96)] | 0.51 (0.46–0.52) [1.00 (0.84–1.03) ~] | 0.27 (0.26–0.29) [1.04 (0.98–1.06) ~] | 0.19 (0.18–0.19) [1.03 (0.90–1.19) ~] | 0.12 (0.12–0.14) [1.20 (1.13–1.32)] | yes | 1.000, 1.000 |

### JPEG, relative to image/jpeg (quality 75)

Time to encode one image with as many workers as the budget P allows (one at P=1); in brackets the CPU time it took. bands: whether calamus split the image at the largest budget. size: output bytes relative to the reference's, with one worker and with the largest budget's.

| fixture | kind | P=1 | P=2 | P=4 | P=8 | P=16 | bands | size |
|---|---|---|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.99 (0.97–1.00) ~ [1.00 (1.00–1.33)] | 0.99 (0.98–1.00) [1.00 (0.75–1.00)] | 1.00 (0.98–1.00) [1.00 (0.80–1.00)] | 0.99 (0.98–1.00) [1.00 (0.75–1.33) ~] | 1.00 (0.98–1.00) ~ [1.00 (0.50–1.00)] | no | 1.000, 1.000 |
| photo/medium/keong-macan-500 | real | 0.99 (0.98–1.00) ~ [1.00 (0.93–1.00)] | 0.54 (0.54–0.75) [1.13 (1.00–1.24)] | 0.37 (0.30–0.39) [1.29 (1.05–1.33)] | 0.24 (0.23–0.26) [1.39 (1.28–1.76)] | 0.24 (0.23–0.26) [1.42 (1.07–1.68)] | yes | 1.000, 1.001 |
| photo/large/flower | real | 0.99 (0.97–1.00) [1.00] | 0.51 (0.48–0.58) [1.00 (0.96–1.07) ~] | 0.32 (0.29–0.32) [1.07 (1.04–1.21)] | 0.17 (0.17–0.19) [1.29 (1.19–1.42)] | 0.14 (0.14–0.16) [1.37 (1.31–1.58)] | yes | 1.000, 1.000 |
| graphic/medium/p3-color-bars | real | 0.98 (0.98–0.99) [1.00 (0.93–1.17) ~] | 0.51 (0.51–0.67) [1.10 (1.00–1.14)] | 0.29 (0.28–0.38) [1.25 (0.84–1.29) ~] | 0.24 (0.21–0.25) [1.24 (1.15–1.52)] | 0.18 (0.17–0.18) [1.35 (1.29–1.53)] | yes | 1.000, 1.001 |
| document/medium/attention-p3-150dpi | real | 0.98 (0.93–0.98) [0.93 (0.86–1.00)] | 0.51 (0.49–0.64) [1.04 (0.96–1.10) ~] | 0.36 (0.25–0.39) [1.20 (0.90–1.24) ~] | 0.21 (0.18–0.22) [1.39 (1.09–1.55)] | 0.13 (0.13–0.15) [1.54 (1.28–1.66)] | yes | 1.000, 1.000 |
| diagram/medium/lines-800x500 | real | 0.97 (0.97–0.99) [1.00 (0.93–1.08) ~] | 0.55 (0.53–0.79) [1.08 (0.92–1.31) ~] | 0.31 (0.29–0.41) [1.23 (1.03–1.45)] | 0.27 (0.24–0.29) [1.56 (0.93–1.96) ~] | 0.27 (0.23–0.29) [1.59 (1.32–1.79)] | yes | 1.000, 1.001 |
| synthetic/medium/page-1240x1754 | synthetic | 1.01 (1.00–1.04) [1.00 (0.94–1.07) ~] | 0.53 (0.52–0.58) [1.07 (1.00–1.14)] | 0.32 (0.28–0.33) [1.19 (1.02–1.28)] | 0.19 (0.18–0.19) [1.28 (1.08–1.42)] | 0.14 (0.14–0.16) [1.74 (1.44–2.08)] | yes | 1.000, 1.000 |
| synthetic/medium/noise-1240x1754 | synthetic | 1.01 (0.99–1.02) ~ [1.00 (0.86–1.08) ~] | 0.55 (0.53–0.60) [1.06 (1.00–1.13)] | 0.31 (0.27–0.34) [1.13 (1.06–1.24)] | 0.18 (0.17–0.19) [1.32 (1.21–1.34)] | 0.13 (0.12–0.14) [1.31 (1.29–1.36)] | yes | 1.000, 1.000 |

## A batch of images: throughput and CPU at equal budgets

Throughput of 16 independent images relative to the reference encoding them on P goroutines at once; in brackets CPU time per image, and the median time per image. Configurations are workers×outer: calamus-1xP encodes P images at a time with one worker each, calamus-Px1 one image at a time with P workers.

### PNG, P=1, relative to image/png on 1 goroutine

| fixture | kind | calamus-1x1 |
|---|---|---|
| photo/small/keong-macan-64 | real | 1.10 (1.05–1.18) [CPU 1.00 (0.91–1.00), image 0.90 (0.82–0.96)] |
| photo/medium/keong-macan-500 | real | 1.05 (1.04–1.09) [CPU 0.93 (0.88–1.00), image 0.93 (0.93–0.95)] |
| photo/large/flower | real | 1.09 (1.08–1.11) [CPU 0.92 (0.92–0.94), image 0.91 (0.91–0.93)] |
| graphic/medium/p3-color-bars | real | 1.11 (1.09–1.20) [CPU 0.94 (0.83–0.96), image 0.89 (0.83–0.91)] |
| document/medium/attention-p3-150dpi | real | 1.22 (1.12–1.38) [CPU 0.84 (0.70–0.91), image 0.83 (0.71–0.90)] |
| diagram/medium/lines-800x500 | real | 1.13 (1.12–1.17) [CPU 0.89 (0.80–0.94), image 0.86 (0.85–0.88)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.21 (1.14–1.22) [CPU 0.83 (0.79–0.90), image 0.85 (0.81–0.88)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.08 (1.08–1.09) [CPU 0.93 (0.90–0.93), image 0.93 (0.92–0.93)] |

### PNG, P=2, relative to image/png on 2 goroutines

| fixture | kind | calamus-1x2 | calamus-2x1 |
|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.92 (0.89–0.93) [CPU 1.00 (1.00–1.22), image 1.06 (1.05–1.10)] | 0.77 (0.74–0.80) [CPU 0.95 (0.75–1.15) ~, image 0.56 (0.52–0.59)] |
| photo/medium/keong-macan-500 | real | 1.05 (1.00–1.09) ~ [CPU 0.94 (0.89–0.97), image 0.95 (0.89–1.01) ~] | 0.55 (0.54–0.60) [CPU 0.91 (0.83–0.94), image 0.92 (0.83–0.92)] |
| photo/large/flower | real | 1.10 (1.08–1.19) [CPU 0.91 (0.85–0.93), image 0.92 (0.85–0.93)] | 1.06 (1.03–1.15) [CPU 0.94 (0.87–0.96), image 0.48 (0.44–0.49)] |
| graphic/medium/p3-color-bars | real | 1.10 (1.06–1.17) [CPU 0.90 (0.81–1.00), image 0.91 (0.88–0.96)] | 1.02 (0.97–1.08) ~ [CPU 0.98 (0.92–1.02) ~, image 0.49 (0.47–0.53)] |
| document/medium/attention-p3-150dpi | real | 1.19 (1.07–1.31) [CPU 0.85 (0.75–0.92), image 0.85 (0.75–0.92)] | 1.10 (1.01–1.24) [CPU 0.86 (0.69–0.89), image 0.46 (0.40–0.50)] |
| diagram/medium/lines-800x500 | real | 1.18 (1.10–1.46) [CPU 0.80 (0.72–0.96), image 0.85 (0.68–0.88)] | 1.05 (0.99–1.12) ~ [CPU 0.90 (0.80–1.00), image 0.47 (0.45–0.53)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.19 (1.10–1.36) [CPU 0.88 (0.75–0.92), image 0.86 (0.77–0.89)] | 1.05 (0.99–1.24) ~ [CPU 0.89 (0.82–1.02) ~, image 0.47 (0.42–0.52)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.07 (1.06–1.08) [CPU 0.94 (0.92–0.95), image 0.94 (0.92–0.95)] | 1.03 (1.01–1.04) [CPU 0.95 (0.93–0.96), image 0.49 (0.48–0.50)] |

### PNG, P=4, relative to image/png on 4 goroutines

| fixture | kind | calamus-1x4 | calamus-2x2 | calamus-4x1 |
|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.83 (0.75–0.87) [CPU 1.28 (0.94–1.32) ~, image 1.19 (1.15–1.37)] | 0.61 (0.58–0.65) [CPU 1.27 (0.97–1.52) ~, image 0.86 (0.80–0.94)] | 0.43 (0.41–0.49) [CPU 1.32 (1.06–1.50), image 0.54 (0.46–0.57)] |
| photo/medium/keong-macan-500 | real | 1.11 (1.06–1.27) [CPU 0.93 (0.87–0.95), image 0.91 (0.82–0.97)] | 0.58 (0.55–0.67) [CPU 0.88 (0.82–0.96), image 0.86 (0.78–0.93)] | 0.32 (0.30–0.34) [CPU 0.84 (0.83–0.88), image 0.79 (0.78–0.90)] |
| photo/large/flower | real | 1.09 (1.05–1.12) [CPU 0.91 (0.90–0.96), image 0.91 (0.90–0.94)] | 1.05 (1.04–1.07) [CPU 0.93 (0.91–0.94), image 0.48 (0.47–0.48)] | 0.99 (0.96–1.01) ~ [CPU 0.94 (0.90–0.98), image 0.26 (0.25–0.26)] |
| graphic/medium/p3-color-bars | real | 1.14 (1.13–1.28) [CPU 0.90 (0.78–0.95), image 0.89 (0.75–0.90)] | 1.02 (0.99–1.18) ~ [CPU 0.95 (0.80–1.00), image 0.50 (0.42–0.52)] | 0.92 (0.91–0.96) [CPU 0.98 (0.86–1.04) ~, image 0.28 (0.27–0.28)] |
| document/medium/attention-p3-150dpi | real | 1.20 (1.17–1.53) [CPU 0.84 (0.70–0.85), image 0.84 (0.70–0.85)] | 1.06 (1.01–1.38) [CPU 0.85 (0.70–0.92), image 0.48 (0.38–0.54)] | 0.86 (0.82–1.05) ~ [CPU 0.86 (0.78–1.05) ~, image 0.30 (0.25–0.33)] |
| diagram/medium/lines-800x500 | real | 1.23 (1.00–1.27) [CPU 0.82 (0.75–1.04) ~, image 0.81 (0.77–1.04) ~] | 1.08 (0.97–1.10) ~ [CPU 0.86 (0.77–0.96), image 0.49 (0.48–0.51)] | 0.59 (0.52–0.61) [CPU 0.84 (0.80–1.04) ~, image 0.44 (0.43–0.49)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.20 (1.04–1.30) [CPU 0.84 (0.75–0.93), image 0.87 (0.78–0.94)] | 1.15 (1.01–1.37) [CPU 0.83 (0.67–0.95), image 0.47 (0.36–0.49)] | 0.97 (0.85–1.06) ~ [CPU 0.89 (0.67–1.00), image 0.28 (0.23–0.30)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.07 (1.04–1.08) [CPU 0.93 (0.89–0.97), image 0.93 (0.92–0.96)] | 1.02 (1.00–1.04) ~ [CPU 0.95 (0.92–0.98), image 0.49 (0.48–0.50)] | 0.96 (0.93–0.97) [CPU 0.99 (0.96–1.04) ~, image 0.26 (0.26–0.27)] |

### PNG, P=8, relative to image/png on 8 goroutines

| fixture | kind | calamus-1x8 | calamus-2x4 | calamus-4x2 | calamus-8x1 |
|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.82 (0.66–0.83) [CPU 1.43 (0.63–1.77) ~, image 1.24 (1.22–1.53)] | 0.65 (0.60–0.67) [CPU 1.14 (0.93–2.06) ~, image 0.85 (0.78–0.88)] | 0.46 (0.42–0.48) [CPU 1.10 (0.72–2.88) ~, image 0.57 (0.54–0.62)] | 0.31 (0.28–0.32) [CPU 1.02 (0.76–2.35) ~, image 0.38 (0.36–0.40)] |
| photo/medium/keong-macan-500 | real | 1.06 (1.04–1.09) [CPU 0.95 (0.86–1.01) ~, image 0.93 (0.92–0.97)] | 0.61 (0.59–0.61) [CPU 0.85 (0.77–0.98), image 0.84 (0.80–0.85)] | 0.33 (0.32–0.33) [CPU 0.81 (0.76–0.85), image 0.76 (0.75–0.79)] | 0.17 (0.17–0.18) [CPU 0.80 (0.71–0.84), image 0.73 (0.71–0.77)] |
| photo/large/flower | real | 1.10 (1.09–1.11) [CPU 0.90 (0.86–0.94), image 0.90 (0.90–0.92)] | 1.07 (1.00–1.09) [CPU 0.92 (0.90–0.97), image 0.47 (0.46–0.50)] | 1.02 (0.98–1.03) ~ [CPU 0.93 (0.91–0.96), image 0.25 (0.25–0.25)] | 0.94 (0.93–0.95) [CPU 0.96 (0.92–1.00), image 0.14 (0.13–0.14)] |
| graphic/medium/p3-color-bars | real | 1.13 (1.12–1.21) [CPU 0.88 (0.81–0.95), image 0.89 (0.80–0.89)] | 1.05 (1.04–1.18) [CPU 0.90 (0.75–1.02) ~, image 0.49 (0.42–0.49)] | 0.97 (0.95–1.08) ~ [CPU 0.95 (0.82–1.11) ~, image 0.27 (0.24–0.27)] | 0.52 (0.51–0.61) [CPU 0.92 (0.73–0.97), image 0.24 (0.20–0.26)] |
| document/medium/attention-p3-150dpi | real | 1.18 (1.13–1.30) [CPU 0.82 (0.79–0.88), image 0.81 (0.78–0.85)] | 1.12 (1.10–1.37) [CPU 0.79 (0.65–0.81), image 0.46 (0.37–0.46)] | 0.94 (0.91–0.99) [CPU 0.83 (0.79–0.84), image 0.28 (0.27–0.29)] | 0.78 (0.74–0.80) [CPU 0.89 (0.77–1.05) ~, image 0.17 (0.17–0.17)] |
| diagram/medium/lines-800x500 | real | 1.25 (1.18–1.51) [CPU 0.76 (0.74–0.86), image 0.77 (0.64–0.89)] | 1.03 (0.98–1.32) ~ [CPU 0.86 (0.76–0.96), image 0.47 (0.37–0.50)] | 0.58 (0.57–0.81) [CPU 0.78 (0.62–0.94), image 0.42 (0.29–0.43)] | 0.31 (0.30–0.45) [CPU 0.80 (0.58–0.86), image 0.39 (0.26–0.41)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.18 (1.15–1.27) [CPU 0.86 (0.81–0.89), image 0.86 (0.76–0.86)] | 1.11 (1.10–1.21) [CPU 0.85 (0.74–0.90), image 0.46 (0.39–0.47)] | 0.93 (0.92–1.03) ~ [CPU 0.89 (0.74–0.95), image 0.28 (0.24–0.28)] | 0.72 (0.69–0.73) [CPU 0.92 (0.85–0.98), image 0.18 (0.18–0.19)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.02 (0.98–1.04) ~ [CPU 0.98 (0.93–0.99), image 0.97 (0.96–1.00) ~] | 1.03 (0.95–1.03) ~ [CPU 0.94 (0.91–1.00) ~, image 0.49 (0.48–0.53)] | 0.96 (0.89–0.98) [CPU 0.98 (0.87–1.09) ~, image 0.26 (0.26–0.28)] | 0.74 (0.70–0.75) [CPU 0.96 (0.87–0.98), image 0.17 (0.17–0.18)] |

### PNG, P=16, relative to image/png on 16 goroutines

| fixture | kind | calamus-1x16 | calamus-2x8 | calamus-4x4 | calamus-8x2 | calamus-16x1 |
|---|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.81 (0.44–0.94) [CPU 1.07 (0.77–1.65) ~, image 1.30 (1.23–3.00)] | 0.79 (0.40–0.89) [CPU 0.83 (0.62–1.22) ~, image 0.72 (0.62–1.71) ~] | 0.64 (0.29–0.73) [CPU 0.91 (0.64–1.27) ~, image 0.48 (0.40–1.19) ~] | 0.46 (0.18–0.54) [CPU 1.04 (0.70–1.43) ~, image 0.32 (0.26–0.99)] | 0.28 (0.12–0.28) [CPU 0.93 (0.70–1.38) ~, image 0.24 (0.20–0.63)] |
| photo/medium/keong-macan-500 | real | 1.09 (0.99–1.13) ~ [CPU 0.91 (0.77–0.95), image 0.91 (0.91–0.94)] | 0.71 (0.69–0.77) [CPU 0.83 (0.75–0.90), image 0.78 (0.74–0.79)] | 0.39 (0.38–0.46) [CPU 0.78 (0.69–0.80), image 0.69 (0.61–0.74)] | 0.21 (0.21–0.25) [CPU 0.76 (0.67–0.77), image 0.66 (0.58–0.67)] | 0.11 (0.11–0.12) [CPU 0.72 (0.64–0.78), image 0.60 (0.57–0.61)] |
| photo/large/flower | real | 1.11 (0.95–1.23) ~ [CPU 0.93 (0.86–0.98), image 0.91 (0.88–1.02) ~] | 1.10 (0.96–1.22) ~ [CPU 0.93 (0.89–0.95), image 0.47 (0.45–0.50)] | 1.05 (0.99–1.15) ~ [CPU 0.93 (0.89–0.97), image 0.25 (0.24–0.26)] | 0.97 (0.85–1.04) ~ [CPU 0.97 (0.85–0.99), image 0.14 (0.14–0.15)] | 0.73 (0.70–0.83) [CPU 0.95 (0.86–1.02) ~, image 0.09 (0.09–0.09)] |
| graphic/medium/p3-color-bars | real | 1.10 (1.02–1.20) [CPU 0.84 (0.84–0.93), image 0.88 (0.87–0.91)] | 1.10 (1.01–1.16) [CPU 0.88 (0.86–0.93), image 0.48 (0.48–0.51)] | 1.06 (1.01–1.14) [CPU 0.84 (0.83–0.91), image 0.26 (0.26–0.27)] | 0.64 (0.62–0.76) [CPU 0.72 (0.69–0.89), image 0.23 (0.20–0.23)] | 0.35 (0.34–0.44) [CPU 0.84 (0.65–0.89), image 0.20 (0.17–0.21)] |
| document/medium/attention-p3-150dpi | real | 1.21 (1.19–1.25) [CPU 0.80 (0.79–0.89), image 0.82 (0.79–0.84)] | 1.22 (1.15–1.29) [CPU 0.78 (0.75–0.89), image 0.43 (0.43–0.45)] | 1.10 (1.01–1.11) [CPU 0.78 (0.75–0.82), image 0.26 (0.24–0.27)] | 0.86 (0.85–0.97) [CPU 0.78 (0.75–0.86), image 0.16 (0.15–0.17)] | 0.56 (0.55–0.65) [CPU 0.79 (0.66–0.85), image 0.12 (0.11–0.13)] |
| diagram/medium/lines-800x500 | real | 1.29 (1.27–1.33) [CPU 0.76 (0.60–0.85), image 0.77 (0.76–0.78)] | 1.17 (1.09–1.22) [CPU 0.78 (0.75–0.88), image 0.46 (0.44–0.48)] | 0.76 (0.70–0.87) [CPU 0.59 (0.57–0.75), image 0.37 (0.32–0.40)] | 0.42 (0.36–0.60) [CPU 0.67 (0.53–0.74), image 0.34 (0.23–0.34)] | 0.23 (0.22–0.32) [CPU 0.63 (0.50–0.70), image 0.31 (0.21–0.33)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.19 (1.15–1.26) [CPU 0.78 (0.78–0.85), image 0.84 (0.77–0.87)] | 1.25 (1.19–1.27) [CPU 0.79 (0.77–0.84), image 0.42 (0.40–0.44)] | 1.19 (1.12–1.24) [CPU 0.81 (0.78–0.86), image 0.24 (0.22–0.25)] | 0.93 (0.82–0.96) [CPU 0.80 (0.55–1.24) ~, image 0.16 (0.15–0.17)] | 0.73 (0.68–0.75) [CPU 0.77 (0.73–0.87), image 0.10 (0.09–0.11)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.05 (1.00–1.23) ~ [CPU 0.95 (0.93–0.98), image 0.96 (0.85–0.99)] | 1.03 (0.99–1.16) ~ [CPU 0.90 (0.88–0.92), image 0.48 (0.46–0.50)] | 1.04 (0.96–1.18) ~ [CPU 0.91 (0.88–0.99), image 0.26 (0.23–0.27)] | 0.89 (0.82–1.04) ~ [CPU 0.88 (0.87–0.98), image 0.16 (0.14–0.16)] | 0.78 (0.75–0.94) [CPU 0.99 (0.93–1.02) ~, image 0.09 (0.08–0.09)] |

### JPEG, P=1, relative to image/jpeg on 1 goroutine

| fixture | kind | calamus-1x1 |
|---|---|---|
| photo/small/keong-macan-64 | real | 0.99 (0.95–1.00) ~ [CPU 1.00 (1.00–1.33), image 0.99 (0.99–1.01) ~] |
| photo/medium/keong-macan-500 | real | 1.01 (1.00–1.06) [CPU 1.00 (0.94–1.07) ~, image 0.99 (0.98–1.00) ~] |
| photo/large/flower | real | 1.00 (0.94–1.03) ~ [CPU 0.99 (0.97–1.04) ~, image 1.00 (0.95–1.10) ~] |
| graphic/medium/p3-color-bars | real | 1.03 (1.01–1.04) [CPU 1.00 (0.93–1.08) ~, image 0.98 (0.97–0.99)] |
| document/medium/attention-p3-150dpi | real | 1.00 (0.98–1.06) ~ [CPU 0.97 (0.94–1.00), image 0.98 (0.94–1.02) ~] |
| diagram/medium/lines-800x500 | real | 1.00 (0.99–1.02) ~ [CPU 1.00 (0.94–1.00), image 0.98 (0.98–0.99)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.00 (0.98–1.09) ~ [CPU 1.00 (0.90–1.02) ~, image 1.00 (0.92–1.04) ~] |
| synthetic/medium/noise-1240x1754 | synthetic | 0.98 (0.92–0.99) [CPU 1.04 (1.00–1.09), image 1.02 (1.00–1.11)] |

### JPEG, P=2, relative to image/jpeg on 2 goroutines

| fixture | kind | calamus-1x2 | calamus-2x1 |
|---|---|---|---|
| photo/small/keong-macan-64 | real | 1.04 (0.92–1.12) ~ [CPU 0.86 (0.75–1.20) ~, image 1.01 (1.00–1.02) ~] | 0.54 (0.46–0.57) [CPU 1.00 (0.75–1.20) ~, image 0.99 (0.99–1.00) ~] |
| photo/medium/keong-macan-500 | real | 1.00 (0.95–1.03) ~ [CPU 1.00 (1.00–1.07), image 0.99 (0.96–1.01) ~] | 0.91 (0.80–0.92) [CPU 1.08 (1.00–1.13), image 0.54 (0.53–0.69)] |
| photo/large/flower | real | 1.01 (0.97–1.08) ~ [CPU 0.97 (0.93–1.03) ~, image 1.00 (0.93–1.05) ~] | 0.92 (0.86–1.01) ~ [CPU 1.04 (0.98–1.09) ~, image 0.55 (0.48–0.57)] |
| graphic/medium/p3-color-bars | real | 1.03 (0.99–1.05) ~ [CPU 0.96 (0.91–1.00), image 0.98 (0.93–0.99)] | 0.98 (0.87–0.99) [CPU 1.11 (0.86–1.12) ~, image 0.51 (0.51–0.60)] |
| document/medium/attention-p3-150dpi | real | 1.02 (0.92–1.08) ~ [CPU 1.00 (0.88–1.10) ~, image 0.99 (0.91–1.03) ~] | 0.94 (0.88–1.10) ~ [CPU 1.05 (0.88–1.20) ~, image 0.51 (0.47–0.58)] |
| diagram/medium/lines-800x500 | real | 1.04 (0.91–1.08) ~ [CPU 0.93 (0.91–1.03) ~, image 0.98 (0.91–1.11) ~] | 0.92 (0.78–1.03) ~ [CPU 0.96 (0.85–1.12) ~, image 0.55 (0.53–0.69)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.04 (1.00–1.12) [CPU 0.98 (0.89–1.02) ~, image 0.97 (0.90–0.99)] | 0.96 (0.88–0.97) [CPU 1.07 (1.00–1.07), image 0.54 (0.52–0.57)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.01 (0.98–1.03) ~ [CPU 0.97 (0.94–1.03) ~, image 0.97 (0.96–1.00) ~] | 0.91 (0.85–0.98) [CPU 1.07 (1.00–1.17), image 0.55 (0.51–0.60)] |

### JPEG, P=4, relative to image/jpeg on 4 goroutines

| fixture | kind | calamus-1x4 | calamus-2x2 | calamus-4x1 |
|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 1.04 (0.96–1.09) ~ [CPU 1.00 (0.57–1.67) ~, image 1.00 (0.99–1.01) ~] | 0.58 (0.54–0.59) [CPU 1.14 (0.75–2.00) ~, image 0.99 (0.99–1.00)] | 0.30 (0.29–0.32) [CPU 1.00 (0.71–2.67) ~, image 0.98 (0.98–0.99)] |
| photo/medium/keong-macan-500 | real | 0.98 (0.98–1.05) ~ [CPU 0.93 (0.92–1.07) ~, image 1.01 (0.98–1.09) ~] | 0.91 (0.81–0.96) [CPU 1.00 (0.92–1.15) ~, image 0.55 (0.52–0.63)] | 0.84 (0.71–0.87) [CPU 1.03 (0.95–1.26) ~, image 0.30 (0.29–0.33)] |
| photo/large/flower | real | 1.05 (0.96–1.18) ~ [CPU 0.94 (0.83–1.02) ~, image 0.98 (0.83–1.01) ~] | 0.99 (0.95–1.05) ~ [CPU 1.01 (0.94–1.02) ~, image 0.51 (0.47–0.53)] | 0.91 (0.87–1.01) ~ [CPU 1.00 (0.90–1.10) ~, image 0.29 (0.25–0.29)] |
| graphic/medium/p3-color-bars | real | 0.97 (0.88–1.02) ~ [CPU 1.00 (0.97–1.15) ~, image 1.04 (0.99–1.17) ~] | 0.96 (0.87–0.99) [CPU 1.07 (0.93–1.09) ~, image 0.52 (0.48–0.61)] | 0.88 (0.75–0.97) [CPU 1.03 (0.90–1.15) ~, image 0.29 (0.26–0.36)] |
| document/medium/attention-p3-150dpi | real | 1.02 (1.00–1.08) ~ [CPU 0.96 (0.94–1.06) ~, image 1.01 (0.93–1.06) ~] | 0.98 (0.94–1.05) ~ [CPU 1.00 (0.88–1.02) ~, image 0.54 (0.50–0.57)] | 0.85 (0.80–0.93) [CPU 1.04 (0.97–1.06) ~, image 0.31 (0.27–0.33)] |
| diagram/medium/lines-800x500 | real | 1.04 (1.02–1.09) [CPU 0.96 (0.88–1.00), image 0.98 (0.90–0.98)] | 0.93 (0.81–0.96) [CPU 1.09 (0.94–1.16) ~, image 0.55 (0.53–0.65)] | 0.90 (0.75–1.07) ~ [CPU 0.98 (0.81–1.11) ~, image 0.29 (0.23–0.33)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.01 (0.99–1.05) ~ [CPU 0.97 (0.88–1.00), image 0.99 (0.97–1.02) ~] | 0.95 (0.91–1.08) ~ [CPU 1.04 (0.90–1.06) ~, image 0.54 (0.47–0.55)] | 0.88 (0.85–0.95) [CPU 1.11 (0.92–1.15) ~, image 0.28 (0.27–0.31)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.02 (1.00–1.09) [CPU 0.98 (0.89–1.04) ~, image 0.98 (0.86–1.04) ~] | 0.94 (0.90–1.00) [CPU 1.04 (1.00–1.10), image 0.54 (0.53–0.56)] | 0.91 (0.87–0.93) [CPU 1.06 (0.96–1.10) ~, image 0.29 (0.28–0.29)] |

### JPEG, P=8, relative to image/jpeg on 8 goroutines

| fixture | kind | calamus-1x8 | calamus-2x4 | calamus-4x2 | calamus-8x1 |
|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 1.03 (0.87–1.06) ~ [CPU 1.00 (0.38–2.00) ~, image 1.00 (0.94–1.24) ~] | 0.63 (0.61–0.69) [CPU 0.88 (0.25–1.00), image 0.95 (0.92–0.98)] | 0.35 (0.33–0.37) [CPU 0.75 (0.38–1.00), image 0.95 (0.92–0.97)] | 0.19 (0.18–0.19) [CPU 0.75 (0.38–1.00), image 0.94 (0.90–0.96)] |
| photo/medium/keong-macan-500 | real | 1.01 (0.98–1.07) ~ [CPU 0.95 (0.94–1.01) ~, image 0.97 (0.93–1.01) ~] | 0.87 (0.80–0.96) [CPU 0.96 (0.87–1.17) ~, image 0.56 (0.52–0.63)] | 0.74 (0.70–0.86) [CPU 1.03 (0.98–1.30) ~, image 0.31 (0.28–0.36)] | 0.63 (0.57–0.66) [CPU 1.17 (0.89–1.26) ~, image 0.18 (0.16–0.21)] |
| photo/large/flower | real | 1.03 (0.98–1.06) ~ [CPU 0.98 (0.93–1.01) ~, image 0.97 (0.93–1.02) ~] | 0.97 (0.95–1.01) ~ [CPU 0.99 (0.96–1.02) ~, image 0.52 (0.49–0.54)] | 0.94 (0.92–0.97) [CPU 1.01 (0.97–1.05) ~, image 0.27 (0.27–0.28)] | 0.90 (0.88–0.92) [CPU 1.04 (0.96–1.09) ~, image 0.14 (0.14–0.15)] |
| graphic/medium/p3-color-bars | real | 1.02 (0.98–1.06) ~ [CPU 0.97 (0.95–1.00), image 0.99 (0.96–1.03) ~] | 1.01 (0.92–1.10) ~ [CPU 0.96 (0.94–1.10) ~, image 0.52 (0.46–0.54)] | 0.91 (0.80–1.01) ~ [CPU 1.03 (0.82–1.14) ~, image 0.29 (0.26–0.30)] | 0.68 (0.65–0.75) [CPU 0.89 (0.80–1.17) ~, image 0.18 (0.16–0.18)] |
| document/medium/attention-p3-150dpi | real | 1.04 (1.01–1.06) [CPU 1.02 (0.93–1.04) ~, image 0.98 (0.98–1.04) ~] | 0.95 (0.95–0.99) [CPU 1.03 (0.96–1.12) ~, image 0.54 (0.53–0.58)] | 0.92 (0.86–0.95) [CPU 1.04 (0.94–1.09) ~, image 0.29 (0.28–0.30)] | 0.80 (0.79–0.92) [CPU 1.06 (0.97–1.14) ~, image 0.16 (0.14–0.17)] |
| diagram/medium/lines-800x500 | real | 0.99 (0.97–1.10) ~ [CPU 0.94 (0.93–1.09) ~, image 1.04 (0.88–1.09) ~] | 0.89 (0.80–1.01) ~ [CPU 1.03 (0.98–1.08) ~, image 0.57 (0.49–0.64)] | 0.80 (0.72–0.84) [CPU 1.07 (0.91–1.10) ~, image 0.30 (0.28–0.36)] | 0.63 (0.61–0.70) [CPU 1.11 (1.00–1.30), image 0.19 (0.16–0.20)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.03 (0.98–1.14) ~ [CPU 1.01 (0.90–1.03) ~, image 1.01 (0.86–1.03) ~] | 0.94 (0.92–1.12) ~ [CPU 0.99 (0.88–1.04) ~, image 0.54 (0.47–0.56)] | 0.93 (0.87–1.09) ~ [CPU 0.99 (0.89–1.07) ~, image 0.29 (0.24–0.29)] | 0.86 (0.82–0.93) [CPU 1.05 (0.95–1.07) ~, image 0.15 (0.15–0.15)] |
| synthetic/medium/noise-1240x1754 | synthetic | 0.98 (0.96–1.00) ~ [CPU 1.03 (0.97–1.06) ~, image 1.02 (1.01–1.03)] | 0.93 (0.92–0.97) [CPU 1.08 (1.00–1.13), image 0.54 (0.53–0.55)] | 0.90 (0.89–0.92) [CPU 1.02 (0.96–1.07) ~, image 0.28 (0.28–0.29)] | 0.86 (0.85–0.87) [CPU 1.09 (1.03–1.12), image 0.15 (0.15–0.15)] |

### JPEG, P=16, relative to image/jpeg on 16 goroutines

| fixture | kind | calamus-1x16 | calamus-2x8 | calamus-4x4 | calamus-8x2 | calamus-16x1 |
|---|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.98 (0.86–1.12) ~ [CPU 1.07 (0.07–3.50) ~, image 1.03 (0.99–1.04) ~] | 0.85 (0.79–0.90) [CPU 0.57 (0.07–4.00) ~, image 0.68 (0.57–0.84)] | 0.58 (0.55–0.65) [CPU 0.80 (0.53–4.00) ~, image 0.55 (0.54–0.56)] | 0.34 (0.29–0.37) [CPU 0.60 (0.40–3.00) ~, image 0.54 (0.53–0.55)] | 0.18 (0.15–0.19) [CPU 0.71 (0.40–5.50) ~, image 0.53 (0.53–0.54)] |
| photo/medium/keong-macan-500 | real | 0.99 (0.99–1.07) ~ [CPU 1.02 (0.98–1.08) ~, image 1.00 (0.98–1.01) ~] | 0.85 (0.81–0.87) [CPU 1.10 (1.03–1.24), image 0.66 (0.64–0.67)] | 0.75 (0.70–0.76) [CPU 1.09 (0.94–1.23) ~, image 0.36 (0.35–0.38)] | 0.66 (0.63–0.70) [CPU 1.06 (0.92–1.14) ~, image 0.17 (0.17–0.18)] | 0.40 (0.37–0.45) [CPU 1.11 (0.66–1.19) ~, image 0.16 (0.14–0.17)] |
| photo/large/flower | real | 0.94 (0.89–1.03) ~ [CPU 1.03 (0.96–1.16) ~, image 0.99 (0.96–1.01) ~] | 0.96 (0.90–0.99) [CPU 1.03 (0.98–1.13) ~, image 0.52 (0.52–0.55)] | 0.99 (0.95–1.08) ~ [CPU 0.99 (0.95–1.07) ~, image 0.27 (0.27–0.27)] | 0.93 (0.91–1.02) ~ [CPU 1.14 (0.92–1.20) ~, image 0.14 (0.14–0.15)] | 0.71 (0.70–0.79) [CPU 0.91 (0.90–1.09) ~, image 0.10 (0.10–0.10)] |
| graphic/medium/p3-color-bars | real | 0.99 (0.98–1.07) ~ [CPU 1.00 (0.94–1.17) ~, image 1.00 (0.96–1.04) ~] | 0.93 (0.92–0.97) [CPU 0.97 (0.92–1.13) ~, image 0.56 (0.55–0.58)] | 0.88 (0.88–0.90) [CPU 0.90 (0.87–1.17) ~, image 0.31 (0.29–0.31)] | 0.72 (0.70–0.74) [CPU 1.05 (0.89–1.17) ~, image 0.19 (0.19–0.19)] | 0.59 (0.56–0.59) [CPU 1.04 (0.91–1.13) ~, image 0.12 (0.11–0.12)] |
| document/medium/attention-p3-150dpi | real | 0.99 (0.95–1.16) ~ [CPU 0.99 (0.97–1.05) ~, image 1.01 (0.92–1.04) ~] | 1.00 (0.94–1.13) ~ [CPU 1.02 (0.96–1.08) ~, image 0.52 (0.48–0.56)] | 0.99 (0.89–1.14) ~ [CPU 1.01 (0.95–1.14) ~, image 0.28 (0.24–0.30)] | 0.92 (0.88–1.04) ~ [CPU 1.04 (0.91–1.08) ~, image 0.15 (0.14–0.16)] | 0.86 (0.79–0.96) [CPU 0.98 (0.85–1.05) ~, image 0.08 (0.07–0.08)] |
| diagram/medium/lines-800x500 | real | 1.00 (0.98–1.08) ~ [CPU 1.07 (0.94–1.24) ~, image 1.00 (0.99–1.01) ~] | 0.90 (0.85–0.91) [CPU 1.09 (0.94–1.30) ~, image 0.61 (0.61–0.62)] | 0.83 (0.77–0.83) [CPU 1.14 (0.76–1.21) ~, image 0.34 (0.33–0.34)] | 0.76 (0.70–0.79) [CPU 1.01 (0.71–1.20) ~, image 0.16 (0.16–0.17)] | 0.41 (0.39–0.51) [CPU 0.98 (0.77–1.47) ~, image 0.16 (0.13–0.17)] |
| synthetic/medium/page-1240x1754 | synthetic | 0.99 (0.97–1.00) ~ [CPU 1.04 (0.96–1.21) ~, image 1.03 (0.99–1.05) ~] | 0.97 (0.95–1.00) ~ [CPU 1.02 (0.99–1.22) ~, image 0.52 (0.51–0.55)] | 0.96 (0.93–0.98) [CPU 1.04 (0.94–1.19) ~, image 0.29 (0.28–0.30)] | 0.98 (0.88–1.00) [CPU 1.09 (0.98–1.15) ~, image 0.14 (0.14–0.16)] | 0.78 (0.77–0.82) [CPU 1.06 (0.94–1.20) ~, image 0.09 (0.08–0.09)] |
| synthetic/medium/noise-1240x1754 | synthetic | 1.02 (0.95–1.09) ~ [CPU 1.06 (0.98–1.09) ~, image 0.99 (0.95–1.07) ~] | 0.92 (0.88–1.06) ~ [CPU 1.10 (0.98–1.16) ~, image 0.55 (0.49–0.59)] | 0.97 (0.91–1.02) ~ [CPU 1.08 (0.96–1.17) ~, image 0.28 (0.26–0.30)] | 0.95 (0.84–1.00) ~ [CPU 1.06 (0.93–1.15) ~, image 0.15 (0.14–0.15)] | 0.83 (0.77–0.92) [CPU 1.06 (0.93–1.16) ~, image 0.08 (0.08–0.09)] |

## Memory

Each cell: sampled peak heap in use / bytes allocated / process high-water mark (peak working set (Windows)), relative to the baseline named; n/a where the baseline's is too small to sample. The heap in use counts objects not yet collected too, and its peak is sampled every millisecond, not exact. control holds the input and encodes nothing: its process ratio shows how much of the high-water mark is input and runtime.

### PNG

| fixture | kind | calamus-1 | calamus-16 | control | calamus-1/warm | calamus-16/warm |
|---|---|---|---|---|---|---|
| photo/medium/keong-macan-500 | real | 2.80 / 3.20 / 1.08 of ref | 2.80 / 3.20 / 1.08 of ref | process 1.01 of ref | 2.80 / 3.20 / 1.08 of ref/warm | 2.80 / 3.20 / 1.09 of ref/warm |
| photo/large/flower | real | 31.20 / 28.88 / 1.17 of ref | 49.45 / 47.23 / 1.50 of ref | process 1.00 of ref | 31.20 / 29.03 / 1.47 of ref/warm | 49.75 / 47.44 / 1.94 of ref/warm |
| graphic/medium/p3-color-bars | real | 5.57 / 5.35 / 1.05 of ref | 8.41 / 8.31 / 1.16 of ref | process 1.00 of ref | 5.57 / 5.35 / 1.16 of ref/warm | 8.40 / 8.30 / 1.26 of ref/warm |
| document/medium/attention-p3-150dpi | real | 10.33 / 9.87 / 1.00 of ref | 18.99 / 20.32 / 1.00 of ref | process 1.00 of ref | 10.33 / 10.00 / 1.00 of ref/warm | 20.02 / 20.56 / 1.01 of ref/warm |
| diagram/medium/lines-800x500 | real | 2.82 / 2.77 / 0.99 of ref | 3.89 / 3.83 / 0.99 of ref | process 0.99 of ref | 2.82 / 2.77 / 1.04 of ref/warm | 3.89 / 3.83 / 1.06 of ref/warm |
| synthetic/medium/page-1240x1754 | synthetic | 10.63 / 10.16 / 1.01 of ref | 21.55 / 21.63 / 1.26 of ref | process 1.00 of ref | 10.63 / 10.29 / 1.17 of ref/warm | 22.52 / 21.89 / 1.72 of ref/warm |
| synthetic/medium/noise-1240x1754 | synthetic | 37.84 / 45.04 / 1.75 of ref | 36.96 / 46.20 / 1.76 of ref | process 1.00 of ref | 37.83 / 45.43 / 2.12 of ref/warm | 36.90 / 46.56 / 2.35 of ref/warm |

### JPEG

| fixture | kind | calamus-1 | calamus-16 | control | calamus-1/warm | calamus-16/warm |
|---|---|---|---|---|---|---|
| photo/medium/keong-macan-500 | real | n/a / 0.99 / 1.00 of ref | n/a / 36.93 / 1.00 of ref | process 1.00 of ref | n/a / 1.00 / 1.00 of ref/warm | n/a / 36.24 / 1.09 of ref/warm |
| photo/large/flower | real | n/a / 1.00 / 1.00 of ref | n/a / 85.65 / 1.00 of ref | process 1.00 of ref | n/a / 1.00 / 1.00 of ref/warm | n/a / 83.16 / 1.11 of ref/warm |
| graphic/medium/p3-color-bars | real | n/a / 1.01 / 1.00 of ref | n/a / 45.91 / 1.00 of ref | process 1.00 of ref | n/a / 0.99 / 1.00 of ref/warm | n/a / 44.17 / 1.12 of ref/warm |
| document/medium/attention-p3-150dpi | real | n/a / 0.99 / 1.00 of ref | n/a / 114.91 / 1.00 of ref | process 1.00 of ref | n/a / 1.01 / 1.00 of ref/warm | n/a / 114.62 / 1.00 of ref/warm |
| diagram/medium/lines-800x500 | real | n/a / 1.00 / 0.98 of ref | n/a / 36.53 / 1.00 of ref | process 0.98 of ref | n/a / 1.00 / 1.00 of ref/warm | n/a / 35.80 / 1.01 of ref/warm |
| synthetic/medium/page-1240x1754 | synthetic | n/a / 1.00 / 1.00 of ref | n/a / 99.10 / 1.00 of ref | process 1.00 of ref | n/a / 1.00 / 1.00 of ref/warm | n/a / 96.04 / 1.11 of ref/warm |
| synthetic/medium/noise-1240x1754 | synthetic | n/a / 1.01 / 1.00 of ref | n/a / 99.42 / 1.01 of ref | process 1.00 of ref | n/a / 1.01 / 1.00 of ref/warm | n/a / 96.70 / 1.14 of ref/warm |

## Animations with a slow writer

A generated animation of 60 frames of 640×480, written through a writer limited to 8 MB/s, against a writer that keeps nothing: sampled peak live heap and process high-water mark, slow over fast for the same encoder; and the slow writer's heap peak against the baseline's with the same slow writer.

| format | encoder | heap slow/fast | process slow/fast | heap against baseline (slow) |
|---|---|---|---|---|
| gif-anim | calamus-1 | 0.99 | 1.00 | 2.58 of ref |
| gif-anim | calamus-16 | 1.03 | 1.00 | 2.49 of ref |
| gif-anim | ref | 1.00 | 1.00 | 1.00 of ref |
| apng-anim | calamus-1 | 1.02 | 1.01 | 1.00 of calamus-1 |
| apng-anim | calamus-16 | 1.02 | 1.00 | 1.19 of calamus-1 |

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
