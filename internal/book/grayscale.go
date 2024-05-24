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
	if err := p.mw.SetImageDepth(8); err != nil {
		return fmt.Errorf("setting output image depth to 8 bits: %w", err)
	}
	return nil
}
