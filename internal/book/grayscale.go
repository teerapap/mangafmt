//
// grayscale.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"
	"math"

	"github.com/teerapap/mangafmt/internal/imgutil"
	"github.com/teerapap/mangafmt/internal/log"
)

type GrayscaleConfig struct {
	PageRange  *PageRange
	ColorDepth uint
}

func IsSupportedColorDepth(depth uint) error {
	switch depth {
	case 1:
		return nil
	case 2:
		return nil
	case 4:
		return nil
	case 8:
		return nil
	case 16:
		return nil
	}
	return fmt.Errorf("unsupported color depth: %d-bits", depth)
}

func (p *Page) ConvertToGrayscale(cfg GrayscaleConfig) error {
	pr := cfg.PageRange
	if pr == nil {
		return nil
	} else if !pr.Contains(p.PageNo) && !(p.OtherPageNo > 0 && pr.Contains(p.OtherPageNo)) {
		return nil
	}
	srcColorDepth := imgutil.ColorDepth(p.img)
	if cfg.ColorDepth < srcColorDepth {
		log.Printf("[Grayscale] Converting to grayscale %d-bit colors from %d-bit colors", cfg.ColorDepth, srcColorDepth)
	} else {
		log.Printf("[Grayscale] Converting to grayscale while keeping %d-bit colors", srcColorDepth)
	}
	p.img = imgutil.TransformToGrayColorModel(p.img)
	if cfg.ColorDepth < srcColorDepth { // need quantize and dither
		numColor := uint(math.Pow(2, float64(cfg.ColorDepth)))
		p.img = imgutil.QuantizeAndDither(p.img, int(numColor))
	}

	return nil
}
