package main

import (
	"image"
	"image/color"
	"image/color/palette"
	stdgif "image/gif"
	stdjpeg "image/jpeg"
	stdpng "image/png"
	"io"
	"math"
	"time"

	cgif "github.com/timzifer/calamus/gif"
	cjpeg "github.com/timzifer/calamus/jpeg"
	cpng "github.com/timzifer/calamus/png"
)

// format is one output format and configuration: the reference encoder
// and calamus's, with the same settings.
type format struct {
	name, ref, settings string
	refEncode           func(w io.Writer, m any) error
	encode              func(w io.Writer, m any, workers int) error
}

var formats = map[string]format{
	"png": {
		name: "png", ref: "image/png", settings: "BestSpeed",
		refEncode: func(w io.Writer, m any) error {
			return (&stdpng.Encoder{CompressionLevel: stdpng.BestSpeed}).Encode(w, m.(image.Image))
		},
		encode: func(w io.Writer, m any, n int) error {
			return (&cpng.Encoder{CompressionLevel: cpng.BestSpeed, Workers: n}).Encode(w, m.(image.Image))
		},
	},
	"jpeg": {
		name: "jpeg", ref: "image/jpeg", settings: "quality 75",
		refEncode: func(w io.Writer, m any) error {
			return stdjpeg.Encode(w, m.(image.Image), &stdjpeg.Options{Quality: 75})
		},
		encode: func(w io.Writer, m any, n int) error {
			return (&cjpeg.Encoder{Quality: 75, Workers: n}).Encode(w, m.(image.Image))
		},
	},
	// Animations, for memory: image/gif against calamus/gif, and APNG,
	// which has no reference and is compared with itself.
	"gif-anim": {
		name: "gif-anim", ref: "image/gif", settings: "pre-quantised frames",
		refEncode: func(w io.Writer, m any) error { return stdgif.EncodeAll(w, m.(*anim).gif) },
		encode: func(w io.Writer, m any, n int) error {
			return (&cgif.Encoder{Workers: n}).EncodeAll(w, m.(*anim).gif)
		},
	},
	"apng-anim": {
		name: "apng-anim", ref: "", settings: "BestSpeed",
		encode: func(w io.Writer, m any, n int) error {
			return (&cpng.Encoder{CompressionLevel: cpng.BestSpeed, Workers: n}).EncodeAll(w, m.(*anim).apng)
		},
	},
}

// anim is a generated animation, as GIF frames in the web-safe palette
// and as APNG frames: a moving gradient with a bouncing square.
type anim struct {
	gif  *stdgif.GIF
	apng *cpng.Animation
}

const animFrames, animW, animH = 60, 640, 480

func newAnim() *anim {
	a := &anim{gif: &stdgif.GIF{}, apng: &cpng.Animation{}}
	for f := range animFrames {
		m := image.NewNRGBA(image.Rect(0, 0, animW, animH))
		cx := int(float64(animW-80) * (0.5 + 0.5*math.Sin(float64(f)/9)))
		cy := int(float64(animH-80) * (0.5 + 0.5*math.Cos(float64(f)/7)))
		for y := range animH {
			for x := range animW {
				c := color.NRGBA{uint8(x + 3*f), uint8(y + 2*f), uint8((x + y) / 4), 0xff}
				if x >= cx && x < cx+80 && y >= cy && y < cy+80 {
					c = color.NRGBA{0xff, 0xff, 0xff, 0xff}
				}
				m.SetNRGBA(x, y, c)
			}
		}
		// The web-safe palette is a 6×6×6 cube: the index is computed,
		// not searched for.
		pm := image.NewPaletted(m.Rect, palette.WebSafe)
		for i := range pm.Pix {
			p := m.Pix[4*i : 4*i+3 : 4*i+3]
			pm.Pix[i] = uint8(36*((int(p[0])+25)/51) + 6*((int(p[1])+25)/51) + (int(p[2])+25)/51)
		}
		a.gif.Image = append(a.gif.Image, pm)
		a.gif.Delay = append(a.gif.Delay, 4)
		a.apng.Frames = append(a.apng.Frames, cpng.Frame{Image: m, Delay: 40 * time.Millisecond})
	}
	return a
}

// slowRate is the slow writer's bandwidth: a slow disk or network.
const slowRate = 8 << 20

// slowWriter stands for a slow disk or network of a fixed bandwidth, so
// that encoded bands wait for the output. It sleeps once the bytes it
// has taken owe at least a millisecond, as sleeps are coarse.
type slowWriter struct {
	bytesPerSecond float64
	owed           time.Duration
}

func (s *slowWriter) Write(p []byte) (int, error) {
	s.owed += time.Duration(float64(len(p)) / s.bytesPerSecond * float64(time.Second))
	if s.owed >= time.Millisecond {
		time.Sleep(s.owed)
		s.owed = 0
	}
	return len(p), nil
}
