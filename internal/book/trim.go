//
// trim.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"

	"github.com/teerapap/mangafmt/internal/imgutil"
	"github.com/teerapap/mangafmt/internal/log"
)

type TrimConfig struct {
	Enabled  bool
	MinSizeP float64
	Margin   int
}

func (p *Page) Trim(cfg TrimConfig, fuzzP float64) error {
	if !cfg.Enabled {
		return nil
	}
	// Trim image with fuzz
	pageRect := p.Rect()
	minSize := pageRect.size.ScaleBy(cfg.MinSizeP)
	bgColor := p.book.Config.BgColor

	tr, err := imgutil.TrimRect(p.img, bgColor[0], fuzzP)
	if err != nil {
		return fmt.Errorf("finding trim box: %w", err)
	}

	trimRect := FromRectangle(tr).
		InsetBy(-cfg.Margin, -cfg.Margin). // add safety margin
		BoundBy(pageRect)                  // bound by page rect

	log.Verbosef("[Trim] trim box: %s", trimRect)

	if trimRect == pageRect { // trim box equals page rect
		log.Printf("[Trim] No trimming needed")
		return nil
	}

	if !minSize.CanFitIn(trimRect.size) {
		// smaller than min size after trimmed
		oldRect := trimRect

		gapX := max(0, int(minSize.Width)-int(trimRect.size.Width))
		gapY := max(0, int(minSize.Height)-int(trimRect.size.Height))
		trimRect = trimRect.
			InsetBy(-gapX/2, -gapY/2). // expand each side to minimum size
			MoveInside(pageRect)       // Move the rect to fit inside page rect frame as much as possible

		log.Printf("[Trim] Page size %s is trimmed by %s but it is smaller than minimum size %s - expanding trim box to minimum %s", pageRect.size, oldRect, minSize, trimRect)
	}

	// Crop to trim rectangle
	p.img = imgutil.CropImage(p.img, trimRect.ToRectangle())

	// Print trim info
	tWidthP := float64(trimRect.size.Width) * 100.0 / float64(pageRect.size.Width)
	tHeightP := float64(trimRect.size.Height) * 100.0 / float64(pageRect.size.Height)
	log.Printf("[Trim] Page size %s is trimmed by %s (%.2f%% | %.2f%%)", pageRect.size, trimRect, tWidthP, tHeightP)

	return nil
}
