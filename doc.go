// Package calamus is a family of image encoders in pure Go that encode on
// all cores: each cuts the image into bands, encodes them concurrently and
// joins them into one ordinary file of its format, readable by any decoder.
//
//   - calamus/png: PNG, a drop-in for image/png (bands of a zlib stream)
//   - calamus/jpeg: baseline JPEG (bands between restart markers), planned
//   - calamus/tiff: TIFF (strips), planned
//   - calamus/gif: GIF (LZW bands), planned
//   - calamus/webp: lossless WebP, planned
//
// This package itself holds no code.
package calamus
