//
// image.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package imgutil

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/ericpauley/go-quantize/quantize"
	"github.com/teerapap/mangafmt/internal/log"
	drawx "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

func NewCanvasSameColor(src image.Image, r image.Rectangle) draw.Image {
	switch v := src.(type) {
	case *image.Alpha:
		return image.NewAlpha(r)
	case *image.Alpha16:
		return image.NewAlpha16(r)
	case *image.CMYK:
		return image.NewCMYK(r)
	case *image.Gray:
		return image.NewGray(r)
	case *image.Gray16:
		return image.NewGray16(r)
	case *image.NRGBA:
		return image.NewNRGBA(r)
	case *image.NRGBA64:
		return image.NewNRGBA64(r)
	case *image.NYCbCrA:
		return image.NewRGBA(r)
	case *image.Paletted:
		return image.NewPaletted(r, v.Palette)
	case *image.RGBA:
		return image.NewRGBA(r)
	case *image.RGBA64:
		return image.NewRGBA64(r)
	case image.Rectangle:
		return image.NewRGBA(r)
	case *image.Uniform:
		return image.NewRGBA(r)
	case *image.YCbCr:
		return image.NewRGBA(r)
	default:
		log.Printf("unexpected image.Image: %#v", src)
		return image.NewRGBA(r)
	}
}

func TransformToGrayColorModel(img image.Image) *image.Gray {
	dst := image.NewGray(img.Bounds())
	draw.Draw(dst, dst.Bounds(), img, img.Bounds().Min, draw.Src)
	return dst
}

func QuantizeAndDither(img image.Image, numColor int) *image.Paletted {
	q := quantize.MedianCutQuantizer{}
	pal := q.Quantize(make(color.Palette, 0, numColor), img)

	dst := image.NewPaletted(img.Bounds(), pal)
	draw.FloydSteinberg.Draw(dst, dst.Bounds(), img, img.Bounds().Min)
	return dst
}

func Resize(src image.Image, size image.Point) image.Image {
	canvas := NewCanvasSameColor(src, image.Rect(0, 0, size.X, size.Y))

	// Resize
	drawx.CatmullRom.Scale(canvas, canvas.Bounds(), src, src.Bounds(), draw.Src, nil)
	return canvas
}

func Rotate(src image.Image, degree float64) image.Image {
	rad := degree * math.Pi / float64(180.0)

	// Rotation matrix
	mm := f64.Aff3{
		math.Cos(rad), -math.Sin(rad), 0,
		math.Sin(rad), math.Cos(rad), 0,
	}
	size := src.Bounds().Size()
	width := int((mm[0] * float64(size.X)) + (mm[1] * float64(size.Y)))
	height := int((mm[3] * float64(size.X)) + (mm[4] * float64(size.Y)))

	canvas := NewCanvasSameColor(src, image.Rect(0, 0, width, height))

	// Rotation Transform
	drawx.CatmullRom.Transform(canvas, mm, src, src.Bounds(), draw.Src, nil)
	return canvas
}

type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

func CropImage(src image.Image, rect image.Rectangle) image.Image {
	if img, ok := src.(subImager); ok {
		return img.SubImage(rect)
	}

	dst := NewCanvasSameColor(src, image.Rectangle{
		Min: image.Pt(0, 0),
		Max: rect.Size(),
	})

	draw.Draw(dst, dst.Bounds(), src, rect.Min, draw.Src)
	return dst
}

func AppendHorizontally(img1 image.Image, img2 image.Image) image.Image {
	r1 := img1.Bounds().Size()
	r2 := img2.Bounds().Size()

	r := image.Rectangle{
		Min: image.Pt(0, 0),
		Max: image.Pt(r1.X+r2.X, max(r1.Y, r2.Y)),
	}

	canvas := NewCanvasSameColor(img1, r)
	draw.Draw(canvas, image.Rectangle{
		Min: r.Min,
		Max: r1,
	}, img1, img1.Bounds().Min, draw.Src)
	draw.Draw(canvas, image.Rectangle{
		Min: r.Min.Add(image.Pt(r1.X, 0)),
		Max: r2.Add(image.Pt(r1.X, 0)),
	}, img2, img2.Bounds().Min, draw.Src)
	return canvas
}

func TrimRect(img image.Image, bgColor color.Color, fuzzP float64) (image.Rectangle, error) {
	bounds := img.Bounds()
	if bounds.Empty() {
		return image.Rectangle{}, nil
	}

	width, height := bounds.Dx(), bounds.Dy()

	top, left, bottom, right := -1, -1, -1, -1
	// start from top
topSearch:
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.At(x, y)
			if !IsColorSimilar(c, bgColor, fuzzP) {
				top = y
				bottom = y
				left = x
				right = x
				break topSearch
			}
		}
	}
	if top < 0 {
		// blank page
		return image.Rectangle{}, nil
	}

	// start from bottom
bottomSearch:
	for y := height - 1; y > bottom; y-- {
		for x := width - 1; x >= 0; x-- {
			c := img.At(x, y)
			if !IsColorSimilar(c, bgColor, fuzzP) {
				bottom = y
				left = min(left, x)
				right = max(right, x)
				break bottomSearch
			}
		}
	}

	// start from left
leftSearch:
	for x := 0; x < left; x++ {
		for y := top + 1; y <= bottom; y++ {
			c := img.At(x, y)
			if !IsColorSimilar(c, bgColor, fuzzP) {
				left = x
				right = max(right, x)
				break leftSearch
			}
		}
	}

	// start from right
rightSearch:
	for x := width - 1; x > right; x-- {
		for y := bottom - 1; y >= top; y-- {
			c := img.At(x, y)
			if !IsColorSimilar(c, bgColor, fuzzP) {
				right = x
				break rightSearch
			}
		}
	}

	trimRect := image.Rect(left, top, right+1, bottom+1)
	return trimRect, nil
}
