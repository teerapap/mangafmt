//
// ops.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package main

import (
	"fmt"
	"os"

	"gopkg.in/gographics/imagick.v2/imagick"
)

type Page = imagick.MagickWand

func size(p *Page) Size {
	return Size{p.GetImageWidth(), p.GetImageHeight()}
}

func process(itr **Page, inFile string, inPage *int, inTotal int, outPage *int) error {
	current := *itr
	processed := 0
	if current != nil {
		defer current.Destroy()
	} else {
		current = imagick.NewMagickWand()
		defer current.Destroy()
		inFilePage := fmt.Sprintf("%s[%d]", inFile, *inPage-1)

		// Read page
		if err := current.SetResolution(float64(density), float64(density)); err != nil {
			return fmt.Errorf("setting page resolution: %w", err)
		}
		if err := current.ReadImage(inFilePage); err != nil {
			return fmt.Errorf("opening input file page: %w", err)
		}
	}
	processed += 1

	// Look ahead next page
	var next *Page
	if *inPage+1 <= inTotal { // has next page
		// Read next page
		next = imagick.NewMagickWand()
		defer next.Destroy()
		inFilePage := fmt.Sprintf("%s[%d]", inFile, *inPage)

		// Read page
		if err := next.SetResolution(float64(density), float64(density)); err != nil {
			return fmt.Errorf("setting next page resolution: %w", err)
		}
		if err := next.ReadImage(inFilePage); err != nil {
			return fmt.Errorf("opening input file next page: %w", err)
		}

		// Check if the next page can merge with current page
		var left *Page = current
		var right *Page = next
		if isRTL {
			left = next
			right = current
		}
		connected, err := ifConnect(left, right)
		if err != nil {
			return fmt.Errorf("checking if two pages are connected: %w", err)
		}
		if connected {
			olog.Printf("Connect page %d and %d together\n", *inPage, *inPage+1)
			// connect two pages
			if current, err = concatPages(left, right); err != nil {
				return fmt.Errorf("connecting two pages: %w", err)
			}
			defer current.Destroy()
			processed += 1
			next = nil
		}
	}

	// Prepare output file
	err := os.MkdirAll(fmt.Sprintf("%s/Images", workDir), 0750)
	if err != nil {
		return fmt.Errorf("creating images directory: %w", err)
	}
	outputFilePage := fmt.Sprintf("%s/Images/page-%02d.png", workDir, *outPage)

	if err := postProcessingPage(current, outputFilePage); err != nil {
		return err
	}

	*outPage += 1
	*inPage += processed
	if next != nil {
		*itr = next.Clone() // next page will become current in the next process
	} else {
		*itr = nil
	}

	return nil
}

var firstPage = true

func ifConnect(left *Page, right *Page) (bool, error) {
	// TODO: Implement connectness checking
	if firstPage {
		firstPage = false
		return false, nil
	}
	return true, nil
}

func concatPages(left *Page, right *Page) (*Page, error) {
	canvas := imagick.NewMagickWand()
	defer canvas.Destroy()

	if err := canvas.AddImage(left); err != nil {
		return nil, fmt.Errorf("connecting left page: %w", err)
	}
	if err := canvas.AddImage(right); err != nil {
		return nil, fmt.Errorf("connecting rigt page: %w", err)
	}
	canvas.ResetIterator()
	return canvas.AppendImages(false), nil
}

func postProcessingPage(p *Page, outputFilePage string) error {
	// Trim image with fuzz
	if err := trimPage(p, 1.0-trimMax, trimFuzz); err != nil {
		return fmt.Errorf("trimming page: %w", err)
	}

	// Resize page to aspect fit screen
	if err := resizePage(p, targetSize); err != nil {
		return fmt.Errorf("resizing page to fit to screen: %w", err)
	}

	// TODO: Convert to grayscale

	// Save as raw image
	if err := p.WriteImage(outputFilePage); err != nil {
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
	// TODO: Print trim percentage
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
