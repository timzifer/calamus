package corpus

import (
	"bytes"
	"image"
	"image/png"
)

// The libjxl test data, a git submodule at testdata/libjxl. The repository
// is CC-BY-4.0 (The JPEG XL Project Authors); its external/ directories
// carry the licences of their sources, given per fixture.
const (
	libjxlSource  = "github.com/libjxl/testdata@73695d3"
	libjxlLicense = "CC-BY-4.0, The JPEG XL Project Authors"
	wesaturate    = "CC0-1.0, wesaturate.com, via " + libjxlSource
	rawPixls      = "CC0-1.0, raw.pixls.us, via " + libjxlSource
	wideGamut     = "Apache-2.0, github.com/codelogic/wide-gamut-tests, via " + libjxlSource
)

func libjxl() []Fixture {
	png := func(name, category, size string, short bool, file, license, fileSHA string) Fixture {
		return Fixture{
			Name: name, Category: category, Size: size, Real: true, Short: short,
			Source: libjxlSource + "/" + file, License: license,
			File: "libjxl/" + file, FileSHA256: fileSHA,
			load: decodePNG,
		}
	}
	return []Fixture{
		png("photo/small/keong-macan-64", "photo", Small, true,
			"external/wesaturate/64px/cvo9xd_keong_macan_srgb8.png", wesaturate,
			"00b23ff5b1444bb3572304a7d229f091033c669c96fa7627c865d040e69631fa"),
		png("photo/small/alfann24-64-paletted", "photo", Small, false,
			"external/wesaturate/64px/phu1or_alfann24_srgb8.png", wesaturate,
			"b2c065ea6c82965850523a5dd0f328e3bccdfeed16433da7354ba8a2be3cd96b"),
		png("photo/small/dji-64-16bit", "photo", Small, false,
			"external/raw.pixls/DJI-FC6310-16bit_709_v4_krita.png", rawPixls,
			"689dcd2bac65263cfa61e501d19caec856e47c92baf12d88809cf14080c28052"),
		png("photo/medium/keong-macan-500", "photo", Medium, true,
			"external/wesaturate/500px/cvo9xd_keong_macan_srgb8.png", wesaturate,
			"eb9189de8004dd0e56c5ccd8312567ae325bcef42861e2bfc86b064f4c4861b6"),
		png("photo/medium/keong-macan-500-gray", "photo", Medium, false,
			"external/wesaturate/500px/cvo9xd_keong_macan_grayscale.png", wesaturate,
			"6c133157d191e70622872db3511178ff9ca11f34a689f743a65aaf4be4035ed0"),
		png("photo/medium/riaphotographs-500-alpha", "photo", Medium, false,
			"external/wesaturate/500px/tmshre_riaphotographs_alpha.png", wesaturate,
			"71139b97f4d1f76a8fa911cd191daa9434c43f9817f18fb2216d2a0b6a598df7"),
		png("photo/large/flower", "photo", Large, true,
			"jxl/flower/flower.png", libjxlLicense,
			"77ea5436547c82c9be2f72308f13b64de3dc889d8c9f3708be3638e6e17cff41"),
		png("photo/large/flower-alpha", "photo", Large, false,
			"jxl/flower/flower_alpha.png", libjxlLicense,
			"66e8943231e548b22069badcca8a97e2348fd09470392a103edaa93168d430ce"),
		png("graphic/medium/p3-color-ring-alpha", "graphic", Medium, false,
			"external/wide-gamut-tests/P3-sRGB-color-ring.png", wideGamut,
			"0f3f3ea157d3c01a37e209e8a5b9f953866c74f20f2cd82d117252e7309b8612"),
		png("graphic/medium/p3-color-bars", "graphic", Medium, true,
			"external/wide-gamut-tests/P3-sRGB-color-bars.png", wideGamut,
			"55af57a76b2e6e5560facc7f80b9dcb63c0a08bf71b4658070d580d5529557e8"),
	}
}

func decodePNG(_ string, data []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(data))
}
