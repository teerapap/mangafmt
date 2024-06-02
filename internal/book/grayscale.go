//
// grayscale.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"

	"github.com/teerapap/mangafmt/internal/log"
	"gopkg.in/gographics/imagick.v2/imagick"
)

func (p *Page) ConvertToGrayscale() error {
	log.Printf("[Grayscale] Converting to grayscale")
	if err := p.mw.TransformImageColorspace(imagick.COLORSPACE_GRAY); err != nil {
		return fmt.Errorf("converting to grayscale: %w", err)
	}
	if err := p.mw.QuantizeImage(16, imagick.COLORSPACE_GRAY, 0, true, false); err != nil {
		return fmt.Errorf("reducing number of colors to 16 colors: %w", err)
	}
	if err := p.mw.SetImageChannelDepth(imagick.CHANNELS_ALL, 4); err != nil {
		return fmt.Errorf("setting output image channel depth to 4 bits: %w", err)
	}
	return nil
}
