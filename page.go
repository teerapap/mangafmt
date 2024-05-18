//
// ops.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package main

import (
	"fmt"
	"gopkg.in/gographics/imagick.v2/imagick"
)

type Page = imagick.MagickWand

func size(p *Page) Size {
	return Size{p.GetImageWidth(), p.GetImageHeight()}
}

func processPage(inputFile string, page int) error {
	inputFilePage := fmt.Sprintf("%s[%d]", inputFile, page-1)
	outputFilePage := fmt.Sprintf("%s/page-%02d.png", workDir, page)

	current := imagick.NewMagickWand()
	defer current.Destroy()

	// Read page
	if err := current.SetResolution(float64(density), float64(density)); err != nil {
		return fmt.Errorf("setting page resolution: %w", err)
	}
	if err := current.ReadImage(inputFilePage); err != nil {
		return fmt.Errorf("opening input file page: %w", err)
	}

	// TODO: Check if the next page can merge with current page
	// TODO: Merge

	// Trim image with fuzz
	if err := trimPage(current, 1.0-trimMax, trimFuzz); err != nil {
		return fmt.Errorf("trimming page: %w", err)
	}

	// Resize page to aspect fit screen
	if err := resizePage(current, targetSize); err != nil {
		return fmt.Errorf("resizing page to fit to screen: %w", err)
	}

	// TODO: Convert to grayscale

	// Save as raw image
	if err := current.WriteImage(outputFilePage); err != nil {
		return fmt.Errorf("writing page to image file: %w", err)
	}
	return nil
}

func trimPage(p *Page, minSizeFactor float64, fuzz float64) error {
	// Trim image with fuzz

	pageSize := size(p)
	minSize := pageSize.ScaleBy(minSizeFactor)

	trim := p.Clone()
	defer trim.Destroy()

	if err := trim.SetGravity(imagick.GRAVITY_CENTER); err != nil {
		return fmt.Errorf("setting trim gravity: %w", err)
	}
	if err := trim.TrimImage(fuzz); err != nil {
		return fmt.Errorf("trimming page: %w", err)
	}

	trimSize := size(trim)
	if minSize.CanFitIn(trimSize) {
		vlog.Printf("Page size %s is trimmed to %s\n", pageSize, trimSize)
		p.SetImage(trim)
		return nil
	}
	olog.Printf("Page size %s is trimmed to %s but it is smaller than minimum size %s - skip trimming\n", pageSize, trimSize, minSize)
	return nil
}

func resizePage(p *Page, screen Size) error {
	// Trim image with fuzz
	pageSize := size(p)
	pgOrient := pageSize.Orientation()
	scrOrient := screen.Orientation()
	if pgOrient != Square && pgOrient != scrOrient {
		// rotate counter-clockwise
		olog.Printf("Rotating page because page orientation (%s) does not match screen orientation (%s)\n", pgOrient, scrOrient)
		pw := imagick.NewPixelWand()
		defer pw.Destroy()
		if err := p.RotateImage(pw, 270); err != nil {
			return fmt.Errorf("Re-orient page: %w", err)
		}
		pageSize = size(p)
		pgOrient = pageSize.Orientation()
	}

	if pageSize.CanFitIn(screen) {
		vlog.Printf("Page size %s can fit in screen size %s - skip resizing\n", pageSize, screen)
		return nil
	}
	fittedSize := pageSize.AspectFitIn(screen, false)

	vlog.Printf("Resizing page size %s to size %s fit in screen size %s \n", pageSize, fittedSize, screen)
	if err := p.ResizeImage(fittedSize.width, fittedSize.height, imagick.FILTER_LANCZOS, 1); err != nil {
		return fmt.Errorf("Resizing page: %w", err)
	}

	return nil
}
