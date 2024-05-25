//
// trim.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"
	"os"

	"github.com/teerapap/mangafmt/internal/log"
	"gopkg.in/gographics/imagick.v2/imagick"
)

type TrimConfig struct {
	Enabled  bool
	MinSizeP float64
	Margin   int
}

func (p *Page) Trim(cfg TrimConfig, fuzzP float64, workDir string) error {
	if !cfg.Enabled {
		return nil
	}
	// Trim image with fuzz
	pageRect := p.Rect()
	minSize := pageRect.size.ScaleBy(cfg.MinSizeP)
	bgColor := p.book.Config.BgColor

	// write tmp file
	tmpFile := p.Filepath(workDir, ".trimming.png")
	if err := p.mw.WriteImage(tmpFile); err != nil {
		return fmt.Errorf("writing tmp file for trimming: %w", err)
	}
	defer os.Remove(tmpFile)

	// finding trim box
	ret, err := imagick.ConvertImageCommand([]string{
		"convert",
		tmpFile,
		"-bordercolor", bgColor, // guiding border so that it trims only specified background color
		"-border", "1",
		"-fuzz", fmt.Sprintf("%.0f%%", fuzzP),
		"-format", "%@",
		"null:",
	})
	if err != nil {
		return fmt.Errorf("finding trim box: %w", err)
	}
	ret.Info.Destroy()

	var trimRect Rect
	if _, err := fmt.Sscanf(ret.Meta, "%dx%d+%d+%d", &(trimRect.size.Width), &(trimRect.size.Height), &(trimRect.origin.X), &(trimRect.origin.Y)); err != nil {
		return fmt.Errorf("parsing trim box %s: %w", ret.Meta, err)
	}
	trimRect = trimRect.
		TranslateBy(-1, -1).               // remove trim guiding border
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

	// Crop based on trim size
	if err := p.mw.CropImage(trimRect.size.Width, trimRect.size.Height, trimRect.origin.X, trimRect.origin.Y); err != nil {
		return fmt.Errorf("trimming page with %s: %w", trimRect, err)
	}

	// Print trim info
	tWidthP := float64(trimRect.size.Width) * 100.0 / float64(pageRect.size.Width)
	tHeightP := float64(trimRect.size.Height) * 100.0 / float64(pageRect.size.Height)
	log.Printf("[Trim] Page size %s is trimmed by %s (%.2f%% | %.2f%%)", pageRect.size, trimRect, tWidthP, tHeightP)

	return nil
}
