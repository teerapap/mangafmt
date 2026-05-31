//
// resize.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package volume

import (
	"image"

	"github.com/teerapap/mangafmt/internal/imgutil"
	"github.com/teerapap/mangafmt/internal/log"
)

type ResizeConfig struct {
	Enabled    bool
	ScreenSize Size
}

func (p *Page) ResizeToFit(cfg ResizeConfig, keepOrientation bool, logger log.Logger) error {
	logger = logger.Indent("> Resize ")
	if !cfg.Enabled {
		return nil
	}
	screen := cfg.ScreenSize
	pageSize := p.Size()
	pgOrient := pageSize.Orientation()
	scrOrient := screen.Orientation()
	if pgOrient != OrientationSquare && pgOrient != scrOrient && !keepOrientation {
		// rotate counter-clockwise
		logger.Info("Rotating page because page orientation does not match screen orientation -", "page_size", pageSize, "page_orientation", pgOrient, "screen_orientation", scrOrient)
		p.img = imgutil.Rotate(p.img, 270, logger)

		pageSize = p.Size()
		//lint:ignore SA4006,SA4017 for correctness
		pgOrient = pageSize.Orientation()
	}

	if pageSize.CanFitIn(screen) {
		logger.Info("Skip resizing becasue page size can fit in screen size -", "page_size", pageSize, "screen_size", screen)
		return nil
	}
	fittedSize := pageSize.AspectFitIn(screen, false)

	logger.Info("Resizing page size to fit in screen size", "page_size", pageSize, "fitted_size", fittedSize, "screen_size", screen)
	p.img = imgutil.Resize(p.img, image.Pt(int(fittedSize.Width), int(fittedSize.Height)), logger)

	return nil
}
