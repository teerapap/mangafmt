//
// color.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package imgutil

import (
	"fmt"
	"image/color"
)

const ColorRange = 0xffff // 16-bit color
const Epsilon = 1.0e-12

func ParseColorHex(str string) (color.Color, error) {
	if len(str) != 7 {
		return nil, fmt.Errorf("color(%s) must be in hex format #ffffff or #FFFFFF: invalid length", str)
	}
	c := color.RGBA{A: 0xFF}
	if _, err := fmt.Sscanf(str, "#%02x%02x%02x", &c.R, &c.G, &c.B); err != nil {
		return nil, fmt.Errorf("color(%s) must be in hex format #ffffff or #FFFFFF: %w", str, err)
	}
	return c, nil
}

func ToHexString(c color.Color) string {
	rgb := color.RGBAModel.Convert(c).(color.RGBA)
	return fmt.Sprintf("#%02x%02x%02x", rgb.R, rgb.G, rgb.B)
}

func FuzzFromPercent(fp float64) float64 {
	return fp * float64(ColorRange)
}

// This function compares two colors within certain distance in a linear 3D color space.
// Two colors are similar when
//
//	fuzz >= sqrt(color_distance^2 * (u.a/qr) * (v.a/qr)  + (u.a - v.a)^2)
//
// where
//
//	color_distance^2  = ((u.r-v.r)^2 + (u.g-v.g)^2 + (u.b-v.b)^2) / 3
//
// See https://imagemagick.org/Usage/bugs/fuzz_distance/
func IsColorSimilar(u color.Color, v color.Color, fuzzP float64) bool {
	uc := color.NRGBA64Model.Convert(u).(color.NRGBA64)
	vc := color.NRGBA64Model.Convert(v).(color.NRGBA64)

	if fuzzP == 0 {
		return uc == vc
	}

	const qr = ColorRange

	fuzz := fuzzP * float64(qr)
	fuzz = fuzz * fuzz

	scale := 1.0
	distance := 0.0

	if uc.A != qr || vc.A != qr {
		// some colors have transparencies
		distance = sqDiff(uc.A, vc.A)
		if distance > fuzz {
			return false
		}

		if uc.A != qr {
			scale *= float64(uc.A) / float64(qr)
		}
		if vc.A != qr {
			scale *= float64(vc.A) / float64(qr)
		}
		if scale <= Epsilon { // near zero
			return true
		}
	}

	distance *= 3.0
	fuzz *= 3.0

	distance += sqDiff(uc.R, vc.R) * scale
	if distance > fuzz {
		return false
	}
	distance += sqDiff(uc.G, vc.G) * scale
	if distance > fuzz {
		return false
	}
	distance += sqDiff(uc.B, vc.B) * scale
	return distance <= fuzz
}

func sqDiff(x uint16, y uint16) float64 {
	return sqDiffF(float64(x), float64(y))
}

func sqDiffF(x float64, y float64) float64 {
	d := x - y
	return d * d
}

func SquaredDistortion(u color.Color, v color.Color) float64 {
	const qr = ColorRange

	ur, ug, ub, ua := u.RGBA()
	vr, vg, vb, va := v.RGBA()

	distortion := 0.0
	channels := 4
	distortion += sqDiffF(float64(ur)/float64(qr), float64(vr)/float64(qr))
	distortion += sqDiffF(float64(ug)/float64(qr), float64(vg)/float64(qr))
	distortion += sqDiffF(float64(ub)/float64(qr), float64(vb)/float64(qr))
	distortion += sqDiffF(float64(ua)/float64(qr), float64(va)/float64(qr))

	return distortion / float64(channels)
}
