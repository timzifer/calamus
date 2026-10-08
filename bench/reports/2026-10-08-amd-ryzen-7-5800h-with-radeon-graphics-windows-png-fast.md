# calamus measurements, 2026-10-08

Ratios to the reference only, one machine, the short corpus. Time, CPU and memory: below 1.00 is less than the reference. Throughput: above 1.00 is more. `~` marks a range across 1.00 (inconclusive). These results hold for this machine and corpus; ratios can shift on other processors and with the machine's load. Raw per-run ratios: [2026-10-08-amd-ryzen-7-5800h-with-radeon-graphics-windows-png-fast.json](2026-10-08-amd-ryzen-7-5800h-with-radeon-graphics-windows-png-fast.json).

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
| commit | 4dd5fd82a0b9438ada651c88c9f429a832305b08 |
| command | `G:\golang\measure.exe -v -formats png-fast -parts latency,batch -name png-fast -out reports` |

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

### PNG-FAST, relative to image/png (image/png at BestSpeed, calamus at FastCompression)

Time to encode one image with as many workers as the budget P allows (one at P=1); in brackets the CPU time it took. bands: whether calamus split the image at the largest budget. size: output bytes relative to the reference's, with one worker and with the largest budget's.

| fixture | kind | P=1 | P=2 | P=4 | P=8 | P=16 | bands | size |
|---|---|---|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.80 (0.61–0.84) [0.62 (0.57–0.69)] | 0.63 (0.57–0.79) [0.71 (0.61–0.88)] | 0.66 (0.56–0.67) [0.50 (0.41–0.87)] | 0.64 (0.60–0.71) [0.53 (0.44–0.88)] | 0.60 (0.55–0.74) [0.53 (0.33–1.32) ~] | no | 0.653, 0.653 |
| photo/medium/keong-macan-500 | real | 0.45 (0.45–0.46) [0.44 (0.42–0.50)] | 0.48 (0.43–0.56) [0.48 (0.48–0.62)] | 0.47 (0.45–0.51) [0.50 (0.39–0.59)] | 0.45 (0.45–0.46) [0.48 (0.46–0.56)] | 0.45 (0.43–0.45) [0.42 (0.39–0.71)] | yes | 1.037, 1.037 |
| photo/large/flower | real | 0.38 (0.36–0.39) [0.40 (0.37–0.40)] | 0.22 (0.21–0.25) [0.44 (0.38–0.45)] | 0.13 (0.12–0.13) [0.46 (0.41–0.51)] | 0.10 (0.09–0.11) [0.60 (0.56–0.74)] | 0.09 (0.08–0.13) [0.67 (0.57–0.82)] | yes | 1.057, 1.057 |
| graphic/medium/p3-color-bars | real | 0.44 (0.41–0.46) [0.43 (0.41–0.50)] | 0.29 (0.26–0.34) [0.61 (0.39–0.69)] | 0.21 (0.18–0.21) [0.64 (0.40–0.80)] | 0.19 (0.16–0.20) [0.67 (0.41–0.68)] | 0.18 (0.15–0.20) [0.67 (0.51–0.75)] | yes | 0.891, 0.890 |
| document/medium/attention-p3-150dpi | real | 0.86 (0.85–0.88) [0.92 (0.83–1.00)] | 0.50 (0.49–0.51) [0.90 (0.87–1.00)] | 0.33 (0.32–0.34) [1.00 (0.86–1.12) ~] | 0.27 (0.22–0.29) [1.10 (1.06–1.17)] | 0.19 (0.17–0.25) [1.03 (0.85–1.56) ~] | yes | 1.432, 1.438 |
| diagram/medium/lines-800x500 | real | 0.42 (0.41–0.43) [0.42 (0.38–0.50)] | 0.29 (0.26–0.34) [0.51 (0.46–0.55)] | 0.27 (0.24–0.34) [0.48 (0.43–0.64)] | 0.27 (0.26–0.33) [0.50 (0.44–0.59)] | 0.27 (0.25–0.32) [0.53 (0.45–0.58)] | yes | 1.014, 1.015 |
| synthetic/medium/page-1240x1754 | synthetic | 0.80 (0.79–0.83) [0.76 (0.67–0.78)] | 0.46 (0.44–0.57) [0.87 (0.81–1.12) ~] | 0.31 (0.29–0.31) [1.02 (0.88–1.21) ~] | 0.25 (0.22–0.26) [1.16 (0.87–1.44) ~] | 0.18 (0.17–0.21) [1.28 (0.90–1.63) ~] | yes | 2.246, 2.245 |
| synthetic/medium/noise-1240x1754 | synthetic | 0.22 (0.21–0.24) [0.24 (0.22–0.26)] | 0.16 (0.12–0.17) [0.31 (0.22–0.33)] | 0.09 (0.08–0.10) [0.31 (0.13–0.33)] | 0.08 (0.07–0.09) [0.45 (0.31–0.54)] | 0.08 (0.07–0.09) [0.55 (0.15–0.64)] | yes | 1.000, 1.000 |

## A batch of images: throughput and CPU at equal budgets

Throughput of 16 independent images relative to the reference encoding them on P goroutines at once; in brackets CPU time per image, and the median time per image. Configurations are workers×outer: calamus-1xP encodes P images at a time with one worker each, calamus-Px1 one image at a time with P workers.

### PNG-FAST, P=1, relative to image/png on 1 goroutine

| fixture | kind | calamus-1x1 |
|---|---|---|
| photo/small/keong-macan-64 | real | 1.66 (1.52–1.69) [CPU 0.58 (0.58–0.64), image 0.58 (0.45–0.80)] |
| photo/medium/keong-macan-500 | real | 2.16 (1.86–2.25) [CPU 0.44 (0.44–0.60), image 0.46 (0.45–0.50)] |
| photo/large/flower | real | 2.64 (2.60–2.93) [CPU 0.38 (0.36–0.39), image 0.38 (0.37–0.38)] |
| graphic/medium/p3-color-bars | real | 1.75 (1.52–2.01) [CPU 0.53 (0.48–0.60), image 0.58 (0.50–0.68)] |
| document/medium/attention-p3-150dpi | real | 1.16 (1.11–1.17) [CPU 0.87 (0.85–0.89), image 0.86 (0.85–0.87)] |
| diagram/medium/lines-800x500 | real | 2.38 (2.35–2.64) [CPU 0.40 (0.36–0.42), image 0.42 (0.40–0.42)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.21 (1.14–1.24) [CPU 0.83 (0.81–0.87), image 0.82 (0.81–0.84)] |
| synthetic/medium/noise-1240x1754 | synthetic | 3.94 (3.31–4.19) [CPU 0.25 (0.24–0.30), image 0.25 (0.24–0.30)] |

### PNG-FAST, P=2, relative to image/png on 2 goroutines

| fixture | kind | calamus-1x2 | calamus-2x1 |
|---|---|---|---|
| photo/small/keong-macan-64 | real | 1.19 (1.16–1.20) [CPU 0.78 (0.60–0.89), image 0.80 (0.75–0.81)] | 1.08 (1.02–1.11) [CPU 0.65 (0.61–0.89), image 0.34 (0.33–0.36)] |
| photo/medium/keong-macan-500 | real | 1.97 (1.88–2.31) [CPU 0.47 (0.42–0.58), image 0.50 (0.44–0.52)] | 1.16 (1.08–1.18) [CPU 0.50 (0.38–0.55), image 0.43 (0.43–0.46)] |
| photo/large/flower | real | 2.58 (2.02–2.61) [CPU 0.39 (0.39–0.48), image 0.39 (0.38–0.48)] | 2.42 (2.35–2.48) [CPU 0.39 (0.38–0.41), image 0.21 (0.20–0.22)] |
| graphic/medium/p3-color-bars | real | 2.11 (1.88–2.17) [CPU 0.44 (0.43–0.53), image 0.47 (0.46–0.54)] | 1.89 (1.55–1.91) [CPU 0.53 (0.49–0.66), image 0.27 (0.26–0.32)] |
| document/medium/attention-p3-150dpi | real | 1.13 (1.09–1.14) [CPU 0.90 (0.85–0.92), image 0.90 (0.87–0.91)] | 1.03 (1.01–1.05) [CPU 0.90 (0.83–0.92), image 0.49 (0.48–0.50)] |
| diagram/medium/lines-800x500 | real | 2.37 (2.26–2.54) [CPU 0.45 (0.39–0.48), image 0.45 (0.41–0.46)] | 2.04 (1.95–2.09) [CPU 0.47 (0.39–0.50), image 0.26 (0.25–0.27)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.24 (1.14–1.54) [CPU 0.79 (0.63–0.86), image 0.80 (0.63–0.86)] | 1.12 (1.01–1.23) [CPU 0.84 (0.76–0.94), image 0.45 (0.42–0.51)] |
| synthetic/medium/noise-1240x1754 | synthetic | 3.87 (3.49–4.90) [CPU 0.24 (0.20–0.27), image 0.24 (0.20–0.28)] | 3.07 (2.91–3.84) [CPU 0.31 (0.24–0.34), image 0.16 (0.13–0.18)] |

### PNG-FAST, P=4, relative to image/png on 4 goroutines

| fixture | kind | calamus-1x4 | calamus-2x2 | calamus-4x1 |
|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 1.00 (0.97–1.08) ~ [CPU 0.92 (0.65–1.27) ~, image 0.98 (0.91–1.04) ~] | 0.83 (0.80–0.86) [CPU 1.00 (0.57–1.37) ~, image 0.59 (0.58–0.61)] | 0.63 (0.60–0.66) [CPU 1.00 (0.55–1.26) ~, image 0.32 (0.30–0.34)] |
| photo/medium/keong-macan-500 | real | 1.91 (1.90–2.03) [CPU 0.43 (0.40–0.54), image 0.52 (0.49–0.54)] | 1.13 (1.07–1.15) [CPU 0.49 (0.44–0.60), image 0.45 (0.44–0.46)] | 0.61 (0.58–0.63) [CPU 0.49 (0.42–0.54), image 0.42 (0.41–0.43)] |
| photo/large/flower | real | 2.42 (2.35–2.60) [CPU 0.40 (0.37–0.42), image 0.39 (0.38–0.42)] | 2.34 (1.83–2.45) [CPU 0.40 (0.37–0.50), image 0.21 (0.21–0.27)] | 2.10 (1.76–2.14) [CPU 0.47 (0.40–0.49), image 0.12 (0.12–0.14)] |
| graphic/medium/p3-color-bars | real | 2.06 (1.83–2.14) [CPU 0.51 (0.45–0.52), image 0.49 (0.45–0.55)] | 1.77 (1.47–2.00) [CPU 0.56 (0.46–0.61), image 0.29 (0.24–0.34)] | 1.59 (1.40–1.71) [CPU 0.57 (0.49–0.64), image 0.16 (0.14–0.19)] |
| document/medium/attention-p3-150dpi | real | 1.19 (1.15–1.38) [CPU 0.83 (0.67–0.85), image 0.83 (0.69–0.87)] | 1.04 (0.88–1.12) ~ [CPU 0.83 (0.74–1.09) ~, image 0.49 (0.44–0.58)] | 0.86 (0.83–0.90) [CPU 0.90 (0.81–0.99), image 0.29 (0.28–0.31)] |
| diagram/medium/lines-800x500 | real | 2.25 (2.00–2.43) [CPU 0.39 (0.38–0.44), image 0.45 (0.41–0.49)] | 1.88 (1.72–2.11) [CPU 0.49 (0.43–0.56), image 0.28 (0.24–0.29)] | 1.12 (1.07–1.25) [CPU 0.48 (0.42–0.49), image 0.24 (0.20–0.24)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.21 (1.18–1.26) [CPU 0.82 (0.80–0.85), image 0.81 (0.81–0.84)] | 1.12 (1.03–1.41) [CPU 0.86 (0.61–0.94), image 0.47 (0.36–0.47)] | 0.90 (0.82–1.03) ~ [CPU 0.84 (0.82–0.93), image 0.28 (0.25–0.31)] |
| synthetic/medium/noise-1240x1754 | synthetic | 4.24 (3.70–4.40) [CPU 0.22 (0.21–0.27), image 0.23 (0.22–0.27)] | 3.51 (3.39–3.59) [CPU 0.26 (0.24–0.28), image 0.14 (0.14–0.15)] | 3.02 (2.82–3.05) [CPU 0.31 (0.28–0.33), image 0.08 (0.08–0.08)] |

### PNG-FAST, P=8, relative to image/png on 8 goroutines

| fixture | kind | calamus-1x8 | calamus-2x4 | calamus-4x2 | calamus-8x1 |
|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.96 (0.83–1.12) ~ [CPU 0.72 (0.47–0.94), image 1.07 (0.91–1.26) ~] | 0.79 (0.60–0.95) [CPU 0.71 (0.48–0.86), image 0.63 (0.54–0.89)] | 0.62 (0.48–0.65) [CPU 0.70 (0.60–0.97), image 0.40 (0.39–0.53)] | 0.45 (0.43–0.48) [CPU 0.54 (0.38–0.70), image 0.23 (0.22–0.24)] |
| photo/medium/keong-macan-500 | real | 1.69 (1.62–1.71) [CPU 0.49 (0.45–0.54), image 0.59 (0.58–0.62)] | 1.13 (0.90–1.17) ~ [CPU 0.51 (0.39–0.61), image 0.45 (0.44–0.54)] | 0.64 (0.63–0.66) [CPU 0.47 (0.41–0.52), image 0.39 (0.39–0.40)] | 0.35 (0.34–0.36) [CPU 0.46 (0.42–0.53), image 0.36 (0.35–0.36)] |
| photo/large/flower | real | 2.25 (2.19–2.29) [CPU 0.45 (0.42–0.46), image 0.44 (0.43–0.44)] | 2.03 (1.93–2.15) [CPU 0.45 (0.44–0.47), image 0.25 (0.23–0.26)] | 1.83 (1.76–1.87) [CPU 0.45 (0.42–0.48), image 0.14 (0.13–0.14)] | 1.60 (1.50–1.61) [CPU 0.44 (0.40–0.58), image 0.08 (0.08–0.08)] |
| graphic/medium/p3-color-bars | real | 1.85 (1.78–1.94) [CPU 0.52 (0.50–0.57), image 0.53 (0.53–0.56)] | 1.54 (1.34–1.63) [CPU 0.54 (0.45–0.60), image 0.34 (0.31–0.37)] | 1.34 (1.30–1.59) [CPU 0.62 (0.54–0.66), image 0.19 (0.16–0.20)] | 0.96 (0.83–1.03) ~ [CPU 0.47 (0.44–0.61), image 0.14 (0.12–0.15)] |
| document/medium/attention-p3-150dpi | real | 1.21 (1.05–1.36) [CPU 0.81 (0.75–0.89), image 0.85 (0.81–0.97)] | 1.03 (0.89–1.52) ~ [CPU 0.83 (0.64–0.96), image 0.50 (0.38–0.58)] | 0.81 (0.57–1.17) ~ [CPU 0.94 (0.72–1.00), image 0.31 (0.24–0.38)] | 0.66 (0.52–0.98) [CPU 0.89 (0.64–0.92), image 0.20 (0.14–0.23)] |
| diagram/medium/lines-800x500 | real | 1.94 (1.91–2.26) [CPU 0.41 (0.31–0.45), image 0.50 (0.45–0.52)] | 1.67 (1.57–1.82) [CPU 0.45 (0.33–0.51), image 0.30 (0.27–0.31)] | 1.17 (0.99–1.41) ~ [CPU 0.39 (0.29–0.40), image 0.21 (0.17–0.25)] | 0.67 (0.61–0.76) [CPU 0.39 (0.35–0.41), image 0.19 (0.18–0.20)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.17 (1.15–1.27) [CPU 0.79 (0.77–0.83), image 0.86 (0.78–0.87)] | 1.06 (0.98–1.16) ~ [CPU 0.86 (0.80–0.94), image 0.48 (0.43–0.51)] | 0.89 (0.84–0.96) [CPU 0.87 (0.86–0.92), image 0.29 (0.27–0.29)] | 0.70 (0.69–0.76) [CPU 0.89 (0.81–1.03) ~, image 0.19 (0.17–0.19)] |
| synthetic/medium/noise-1240x1754 | synthetic | 3.63 (3.43–3.74) [CPU 0.25 (0.24–0.28), image 0.28 (0.25–0.28)] | 2.77 (2.70–2.84) [CPU 0.30 (0.29–0.33), image 0.18 (0.17–0.19)] | 2.33 (2.18–2.41) [CPU 0.35 (0.29–0.45), image 0.11 (0.10–0.12)] | 1.86 (1.80–1.97) [CPU 0.37 (0.28–0.43), image 0.07 (0.06–0.07)] |

### PNG-FAST, P=16, relative to image/png on 16 goroutines

| fixture | kind | calamus-1x16 | calamus-2x8 | calamus-4x4 | calamus-8x2 | calamus-16x1 |
|---|---|---|---|---|---|---|
| photo/small/keong-macan-64 | real | 0.90 (0.74–1.02) ~ [CPU 0.76 (0.66–1.42) ~, image 1.28 (1.21–1.98)] | 0.86 (0.72–1.02) ~ [CPU 0.85 (0.52–1.08) ~, image 0.72 (0.68–0.84)] | 0.70 (0.61–0.81) [CPU 0.86 (0.56–1.12) ~, image 0.43 (0.37–0.55)] | 0.68 (0.45–0.71) [CPU 0.54 (0.44–1.16) ~, image 0.23 (0.20–0.43)] | 0.40 (0.30–0.44) [CPU 0.76 (0.41–1.19) ~, image 0.16 (0.13–0.27)] |
| photo/medium/keong-macan-500 | real | 1.46 (1.38–1.51) [CPU 0.49 (0.41–0.70), image 0.72 (0.69–0.75)] | 1.14 (1.07–1.26) [CPU 0.44 (0.41–0.50), image 0.49 (0.46–0.51)] | 0.79 (0.77–0.86) [CPU 0.36 (0.31–0.39), image 0.35 (0.33–0.36)] | 0.47 (0.44–0.50) [CPU 0.38 (0.33–0.41), image 0.31 (0.29–0.31)] | 0.26 (0.25–0.27) [CPU 0.33 (0.30–0.35), image 0.28 (0.26–0.28)] |
| photo/large/flower | real | 2.05 (1.55–2.28) [CPU 0.45 (0.40–0.49), image 0.53 (0.45–0.58)] | 1.92 (1.40–2.06) [CPU 0.43 (0.40–0.46), image 0.27 (0.24–0.34)] | 1.79 (1.44–1.92) [CPU 0.47 (0.40–0.52), image 0.15 (0.14–0.17)] | 1.67 (1.52–1.75) [CPU 0.47 (0.44–0.56), image 0.09 (0.08–0.09)] | 1.30 (1.27–1.42) [CPU 0.50 (0.47–0.55), image 0.05 (0.05–0.06)] |
| graphic/medium/p3-color-bars | real | 1.83 (1.69–2.09) [CPU 0.53 (0.42–0.61), image 0.57 (0.54–0.61)] | 1.61 (1.48–1.67) [CPU 0.66 (0.51–0.70), image 0.37 (0.35–0.40)] | 1.52 (1.29–1.55) [CPU 0.61 (0.41–0.64), image 0.20 (0.20–0.24)] | 1.13 (1.02–1.20) [CPU 0.46 (0.38–0.59), image 0.14 (0.13–0.15)] | 0.79 (0.68–0.84) [CPU 0.48 (0.41–0.52), image 0.10 (0.10–0.12)] |
| document/medium/attention-p3-150dpi | real | 1.33 (1.28–1.38) [CPU 0.76 (0.71–0.85), image 0.80 (0.77–0.81)] | 1.15 (1.09–1.27) [CPU 0.83 (0.79–0.96), image 0.49 (0.47–0.52)] | 1.07 (1.04–1.17) [CPU 0.79 (0.67–0.96), image 0.28 (0.26–0.28)] | 0.88 (0.78–0.96) [CPU 0.79 (0.76–1.00), image 0.17 (0.16–0.19)] | 0.69 (0.64–0.77) [CPU 0.88 (0.52–0.99), image 0.11 (0.10–0.11)] |
| diagram/medium/lines-800x500 | real | 2.17 (1.76–2.34) [CPU 0.40 (0.34–0.51), image 0.47 (0.44–0.54)] | 1.89 (1.77–1.93) [CPU 0.42 (0.29–0.54), image 0.28 (0.28–0.31)] | 1.35 (1.15–1.37) [CPU 0.41 (0.39–0.52), image 0.20 (0.20–0.23)] | 0.95 (0.72–1.00) ~ [CPU 0.36 (0.32–0.43), image 0.14 (0.14–0.20)] | 0.58 (0.44–0.60) [CPU 0.31 (0.25–0.48), image 0.12 (0.12–0.16)] |
| synthetic/medium/page-1240x1754 | synthetic | 1.17 (1.15–1.26) [CPU 0.84 (0.77–0.95), image 0.82 (0.73–0.85)] | 1.09 (1.02–1.29) [CPU 0.80 (0.75–1.00), image 0.47 (0.43–0.48)] | 0.96 (0.92–1.14) ~ [CPU 0.90 (0.75–1.09) ~, image 0.28 (0.26–0.30)] | 0.82 (0.75–1.05) ~ [CPU 0.90 (0.79–1.16) ~, image 0.17 (0.14–0.19)] | 0.67 (0.67–0.87) [CPU 1.01 (0.85–1.12) ~, image 0.10 (0.09–0.11)] |
| synthetic/medium/noise-1240x1754 | synthetic | 2.49 (2.28–2.64) [CPU 0.34 (0.27–0.35), image 0.41 (0.35–0.43)] | 2.04 (1.81–2.40) [CPU 0.43 (0.33–0.44), image 0.26 (0.22–0.30)] | 1.78 (1.51–2.48) [CPU 0.45 (0.41–0.48), image 0.16 (0.12–0.19)] | 1.47 (1.38–2.14) [CPU 0.47 (0.33–0.58), image 0.10 (0.07–0.11)] | 1.30 (0.98–1.94) ~ [CPU 0.36 (0.32–0.59), image 0.06 (0.04–0.06)] |

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
