//
// distortion.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package imgutil

import (
	"image"
	"math"
)

func GetRMSEDistortion(img1 image.Image, r image.Rectangle, img2 image.Image, sp image.Point) float64 {
	bounds := r.Bounds()
	distortion := 0.0
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {

			c1 := img1.At(x+bounds.Min.X, y+bounds.Min.Y)
			c2 := img2.At(x+sp.X, y+sp.Y)

			distortion += SquaredDistortion(c1, c2)
		}
	}

	distortion = distortion / float64(bounds.Dx()*bounds.Dy())
	return math.Sqrt(distortion)
}
