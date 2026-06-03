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

func NewCanvasSameColor(src image.Image, r image.Rectangle, logger log.Logger) draw.Image {
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
		logger.Warn("unexpected image.Image -", "img", src)
		return image.NewRGBA(r)
	}
}

func ColorDepth(src image.Image) uint {
	switch src.(type) {
	case *image.Alpha16:
		return 16
	case *image.Gray16:
		return 16
	case *image.NRGBA64:
		return 16
	case *image.RGBA64:
		return 16
	case image.Rectangle:
		return 16
	case *image.Uniform:
		return 16
	default:
		return 8
	}
}

func TransformToGrayColorModel(img image.Image, logger log.Logger) image.Image {
	switch img.(type) {
	case *image.Gray16, *image.Gray:
		return img
	}
	srcDepth := ColorDepth(img)
	var dst draw.Image
	if srcDepth == 16 {
		dst = image.NewGray16(img.Bounds())
	} else {
		dst = image.NewGray(img.Bounds())
	}
	draw.Draw(dst, dst.Bounds(), img, img.Bounds().Min, draw.Src)
	return dst
}

func QuantizeAndDither(img image.Image, numColor int, logger log.Logger) *image.Paletted {
	q := quantize.MedianCutQuantizer{}
	pal := q.Quantize(make(color.Palette, 0, numColor), img)

	dst := image.NewPaletted(img.Bounds(), pal)
	draw.FloydSteinberg.Draw(dst, dst.Bounds(), img, img.Bounds().Min)
	return dst
}

func Resize(src image.Image, size image.Point, logger log.Logger) image.Image {
	canvas := NewCanvasSameColor(src, image.Rect(0, 0, size.X, size.Y), logger)

	// Resize
	drawx.CatmullRom.Scale(canvas, canvas.Bounds(), src, src.Bounds(), draw.Src, nil)
	return canvas
}

func Rotate(src image.Image, degree float64, logger log.Logger) image.Image {
	rad := degree * math.Pi / float64(180.0)

	// Rotation matrix
	mm := f64.Aff3{
		math.Cos(rad), -math.Sin(rad), 0,
		math.Sin(rad), math.Cos(rad), 0,
	}
	size := src.Bounds().Size()
	width := int((mm[0] * float64(size.X)) + (mm[1] * float64(size.Y)))
	height := int((mm[3] * float64(size.X)) + (mm[4] * float64(size.Y)))

	canvas := NewCanvasSameColor(src, image.Rect(0, 0, width, height), logger)

	// Rotation Transform
	drawx.CatmullRom.Transform(canvas, mm, src, src.Bounds(), draw.Src, nil)
	return canvas
}

type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

func CropImage(src image.Image, rect image.Rectangle, logger log.Logger) image.Image {
	if img, ok := src.(subImager); ok {
		return img.SubImage(rect)
	}

	dst := NewCanvasSameColor(src, image.Rectangle{
		Min: image.Pt(0, 0),
		Max: rect.Size(),
	}, logger)

	draw.Draw(dst, dst.Bounds(), src, rect.Min, draw.Src)
	return dst
}

// ConcatHorizontally returns an image.Image that is the horizontal concatenation of left and right.
// No pixel data is copied; At() reads are delegated to the underlying images.
// The returned image has bounds with origin (0, 0); height is max(left.h, right.h),
// and rows past either source's height read as transparent.
func ConcatHorizontally(left, right image.Image) image.Image {
	lb, rb := left.Bounds(), right.Bounds()
	h := lb.Dy()
	if rb.Dy() > h {
		h = rb.Dy()
	}
	return &hConcat{
		left: left, right: right,
		leftW:    lb.Dx(),
		leftOff:  lb.Min,
		rightOff: rb.Min,
		leftH:    lb.Dy(),
		rightH:   rb.Dy(),
		bounds:   image.Rect(0, 0, lb.Dx()+rb.Dx(), h),
	}
}

type hConcat struct {
	left, right       image.Image
	leftOff, rightOff image.Point
	leftW, leftH      int
	rightH            int
	bounds            image.Rectangle
}

func (h *hConcat) ColorModel() color.Model { return h.left.ColorModel() }
func (h *hConcat) Bounds() image.Rectangle { return h.bounds }

func (h *hConcat) At(x, y int) color.Color {
	if x < h.leftW {
		if y >= h.leftH {
			return color.Transparent
		}
		return h.left.At(h.leftOff.X+x, h.leftOff.Y+y)
	}
	if y >= h.rightH {
		return color.Transparent
	}
	return h.right.At(h.rightOff.X+(x-h.leftW), h.rightOff.Y+y)
}

// detectBackgroundColor estimates the margin color of img as the most frequent
// color within a band along its outer edge, and returns it together with that
// color's share of all sampled band pixels. A low share means the edge has no
// solid border, so the color should not be trusted as a margin. Manga pages
// almost always have a solid-color border, so the outer band is normally
// dominated by the margin color.
func detectBackgroundColor(img image.Image) (color.Color, float64) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	minX, minY := bounds.Min.X, bounds.Min.Y

	// How many pixels deep to scan inward from each edge.
	const depth = 20

	counts := make(map[color.NRGBA64]int)
	sample := func(x, y int) {
		c := color.NRGBA64Model.Convert(img.At(minX+x, minY+y)).(color.NRGBA64)
		counts[c]++
	}

	// Clamp band depths so the top/bottom and left/right bands never overlap
	// each other or read out of bounds on small images.
	topD := min(depth, height)
	botD := min(depth, height-topD)
	leftD := min(depth, width)
	rightD := min(depth, width-leftD)

	// Top and bottom bands span the full width.
	for x := 0; x < width; x++ {
		for y := 0; y < topD; y++ {
			sample(x, y)
		}
		for y := height - botD; y < height; y++ {
			sample(x, y)
		}
	}
	// Left and right bands cover only the rows not already scanned above.
	for y := topD; y < height-botD; y++ {
		for x := 0; x < leftD; x++ {
			sample(x, y)
		}
		for x := width - rightD; x < width; x++ {
			sample(x, y)
		}
	}

	var bg color.NRGBA64
	best, total := 0, 0
	for c, n := range counts {
		total += n
		if n > best {
			best = n
			bg = c
		}
	}
	if total == 0 {
		return bg, 0
	}
	return bg, float64(best) / float64(total)
}

// minBgDominance is the minimum fraction of edge-band pixels that must share
// the detected background color for TrimRect to treat it as a real margin.
// Below this, img is assumed to have no solid border and is left untrimmed.
const minBgDominance = 0.35

// TrimRect computes the rectangle remaining after trimming the solid-color
// margin from all sides of img. The margin color is detected automatically via
// detectBackgroundColor; if no dominant edge color is found, img is left
// untrimmed and its full bounds are returned. All coordinates are in img's own
// coordinate space, so sub-images (non-zero Bounds().Min) are handled correctly.
func TrimRect(img image.Image, fuzzP float64, logger log.Logger) (image.Rectangle, error) {
	bounds := img.Bounds()
	if bounds.Empty() {
		return image.Rectangle{}, nil
	}

	bgColor, bgDominance := detectBackgroundColor(img)
	if bgDominance < minBgDominance {
		// No dominant edge color, so there is no reliable margin to trim.
		return bounds, nil
	}

	minX, minY := bounds.Min.X, bounds.Min.Y
	maxX, maxY := bounds.Max.X, bounds.Max.Y

	top, left, bottom, right := 0, 0, 0, 0
	found := false
	// start from top
topSearch:
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			c := img.At(x, y)
			if !IsColorSimilar(c, bgColor, fuzzP) {
				top = y
				bottom = y
				left = x
				right = x
				found = true
				break topSearch
			}
		}
	}
	if !found {
		// blank page
		return image.Rectangle{}, nil
	}

	// start from bottom
bottomSearch:
	for y := maxY - 1; y > bottom; y-- {
		for x := maxX - 1; x >= minX; x-- {
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
	for x := minX; x < left; x++ {
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
	for x := maxX - 1; x > right; x-- {
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
