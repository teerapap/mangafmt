//
// resize.go
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

func (p *Page) ResizeToFit(screen Size) error {
	pageSize := p.Size()
	pgOrient := pageSize.Orientation()
	scrOrient := screen.Orientation()
	if pgOrient != Square && pgOrient != scrOrient {
		// rotate counter-clockwise
		log.Printf("[Resize] Rotating page because page orientation %s (%s) does not match screen orientation (%s)", pageSize, pgOrient, scrOrient)
		pw := imagick.NewPixelWand()
		defer pw.Destroy()
		if err := p.mw.RotateImage(pw, 270); err != nil {
			return fmt.Errorf("re-orient page: %w", err)
		}
		pageSize = p.Size()
		//lint:ignore SA4006,SA4017 for correctness
		pgOrient = pageSize.Orientation()
	}

	if pageSize.CanFitIn(screen) {
		log.Printf("[Resize] Page size %s can fit in screen size %s - skip resizing", pageSize, screen)
		return nil
	}
	fittedSize := pageSize.AspectFitIn(screen, false)

	log.Printf("[Resize] Resizing page size %s to size %s fit in screen size %s", pageSize, fittedSize, screen)
	if err := p.mw.ResizeImage(fittedSize.Width, fittedSize.Height, imagick.FILTER_LANCZOS, 1); err != nil {
		return fmt.Errorf("resizing page: %w", err)
	}

	return nil
}
