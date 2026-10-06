// Package calamus is a family of image encoders in pure Go that encode on
// all cores: each cuts the image into bands, encodes them concurrently and
// joins them into one ordinary file of its format, readable by any decoder.
//
//   - calamus/png: PNG and animated PNG, a drop-in for image/png (bands of a zlib stream, frames)
//   - calamus/jpeg: baseline JPEG, a drop-in for image/jpeg (bands between restart markers)
//   - calamus/tiff: TIFF, a drop-in for golang.org/x/image/tiff's encoder (strips), with LZW and the predictor
//   - calamus/gif: GIF and animated GIF, a drop-in for image/gif (frames, LZW bands, dithering as a wavefront)
//   - calamus/webp: lossless WebP, still and animated (transforms per pixel, LZ77 per band, spliced bit streams)
//
// This package itself holds no code.
package calamus
