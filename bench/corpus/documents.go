package corpus

import (
	"context"
	"fmt"
	"image"
	"image/color"

	"github.com/timzifer/cera"
)

// Download is a file fetched into the corpus directory, verified by its
// SHA-256. These PDFs are the ones cera's own corpus pins.
type Download struct {
	File, URL, SHA256, License string
}

const pdfjsRev = "18e8a26a3813a319b38c806076f0b0ef9baf1bf4"

// Downloads returns the files cmd/corpus fetch downloads.
func Downloads() []Download {
	pdfjs := func(name, sha string) Download {
		return Download{
			File: "pdf/pdfjs/" + name, SHA256: sha,
			URL:     "https://raw.githubusercontent.com/mozilla/pdf.js/" + pdfjsRev + "/test/pdfs/" + name,
			License: "pdf.js test suite (github.com/mozilla/pdf.js@" + pdfjsRev[:12] + "); fetched, not redistributed",
		}
	}
	arxiv := func(id, sha string) Download {
		return Download{
			File: "pdf/arxiv/" + id + ".pdf", SHA256: sha,
			URL:     "https://arxiv.org/pdf/" + id,
			License: "terms as stated on arxiv.org/abs/" + id + "; fetched, not redistributed",
		}
	}
	return []Download{
		pdfjs("tracemonkey.pdf", "3662ff519e485810520552bf301d8c3b2b917fd2f83303f4965d7abed367e113"),
		pdfjs("bitmap-composite-and-xnor-text.pdf", "aba6249b83158532ad87ccf47ba69737c9ba2b787c2f815767d239e910564235"),
		arxiv("1706.03762v7", "bdfaa68d8984f0dc02beaca527b76f207d99b666d31d1da728ee0728182df697"),
		arxiv("1512.03385v1", "1e0651b6810ecba34a3dbc5b5b0209226f889004607c1f203540a48d64e5a93a"),
	}
}

// documents are pages rendered with cera on one goroutine (bands may
// round antialiasing differently where they meet) on white paper.
func documents() []Fixture {
	dl := map[string]Download{}
	for _, d := range Downloads() {
		dl[d.File] = d
	}
	page := func(name, size string, short bool, file string, index int, dpi float64) Fixture {
		d := dl[file]
		return Fixture{
			Name: name, Category: "document", Size: size, Real: true, Short: short,
			Source:  fmt.Sprintf("%s, page %d, rendered by cera at %g dpi", d.URL, index+1, dpi),
			License: d.License, File: file, FileSHA256: d.SHA256,
			load: func(_ string, data []byte) (image.Image, error) { return render(data, index, dpi) },
		}
	}
	return []Fixture{
		page("document/small/attention-p3-72dpi", Small, false, "pdf/arxiv/1706.03762v7.pdf", 2, 72),
		page("document/medium/attention-p3-150dpi", Medium, true, "pdf/arxiv/1706.03762v7.pdf", 2, 150),
		page("document/large/attention-p3-300dpi", Large, false, "pdf/arxiv/1706.03762v7.pdf", 2, 300),
		page("document/medium/tracemonkey-p1-150dpi", Medium, false, "pdf/pdfjs/tracemonkey.pdf", 0, 150),
		page("document/medium/resnet-p1-150dpi", Medium, false, "pdf/arxiv/1512.03385v1.pdf", 0, 150),
		page("document/medium/scan-p1-150dpi", Medium, false, "pdf/pdfjs/bitmap-composite-and-xnor-text.pdf", 0, 150),
	}
}

func render(data []byte, index int, dpi float64) (image.Image, error) {
	doc, err := cera.Open(data)
	if err != nil {
		return nil, err
	}
	p, err := doc.Page(index)
	if err != nil {
		return nil, err
	}
	defer p.Release()
	scale := dpi / 72
	dst := image.NewRGBA(p.Bounds(scale))
	err = p.Render(context.Background(), dst, cera.RenderOptions{
		Scale:      scale,
		Background: color.RGBA{255, 255, 255, 255},
		Workers:    1,
	})
	return dst, err
}
