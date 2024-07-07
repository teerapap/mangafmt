//
// resize.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"image"

	"github.com/teerapap/mangafmt/internal/imgutil"
	"github.com/teerapap/mangafmt/internal/log"
)

func (p *Page) ResizeToFit(screen Size) error {
	pageSize := p.Size()
	pgOrient := pageSize.Orientation()
	scrOrient := screen.Orientation()
	if pgOrient != Square && pgOrient != scrOrient {
		// rotate counter-clockwise
		log.Printf("[Resize] Rotating page because page orientation %s (%s) does not match screen orientation (%s)", pageSize, pgOrient, scrOrient)
		p.img = imgutil.Rotate(p.img, 270)

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
	p.img = imgutil.Resize(p.img, image.Pt(int(fittedSize.Width), int(fittedSize.Height)))

	return nil
}
