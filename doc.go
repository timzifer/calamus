// Package calamus writes PNG images fast, on all cores: the image is
// filtered and deflated in bands concurrently, as pigz does for gzip, and
// the bands are joined into one ordinary PNG that any decoder reads.
//
// It is meant as a drop-in for image/png's Encoder where encoding time
// matters: rendered pages, screenshots, map tiles.
package calamus
