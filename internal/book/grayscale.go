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

	"github.com/teerapap/mangafmt/internal/log"
	"gopkg.in/gographics/imagick.v2/imagick"
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
	return fmt.Errorf("Unsupported color depth: %d-bits", depth)
}

func (p *Page) ConvertToGrayscale(cfg GrayscaleConfig) error {
	pr := cfg.PageRange
	if pr == nil {
		return nil
	} else if !pr.Contains(p.PageNo) && !(p.OtherPageNo > 0 && pr.Contains(p.OtherPageNo)) {
		return nil
	}
	log.Printf("[Grayscale] Converting to grayscale %d-bit colors", cfg.ColorDepth)
	if err := p.mw.TransformImageColorspace(imagick.COLORSPACE_GRAY); err != nil {
		return fmt.Errorf("converting to grayscale: %w", err)
	}
	if cfg.ColorDepth != 16 { // default is 16-bits color
		numColor := uint(math.Pow(2, float64(cfg.ColorDepth)))
		if err := p.mw.QuantizeImage(numColor, imagick.COLORSPACE_GRAY, 0, true, false); err != nil {
			return fmt.Errorf("reducing number of colors to %d colors: %w", numColor, err)
		}
		if err := p.mw.SetImageChannelDepth(imagick.CHANNELS_ALL, cfg.ColorDepth); err != nil {
			return fmt.Errorf("setting output image channel depth to %d bits: %w", cfg.ColorDepth, err)
		}
	}
	return nil
}
