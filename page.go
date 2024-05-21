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

func fuzzFromPercent(fp float64) float64 {
	_, colorRange := imagick.GetQuantumRange()
	return fp * float64(colorRange)
}

func numDigits(total int) int {
	d := 1
	for ; total >= 10; total = total / 10 {
		d += 1
	}
	return d
}

func process(itr **Page, bookFile string, pageCount int, curPage *int, lastPage int, outPage *int) error {
	current := *itr
	processed := 0
	if current != nil {
		defer current.Destroy()
	} else {
		current = imagick.NewMagickWand()
		defer current.Destroy()
		inFilePage := fmt.Sprintf("%s[%d]", bookFile, *curPage-1)

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
	if *curPage+1 <= lastPage { // has next page
		// Read next page
		next = imagick.NewMagickWand()
		defer next.Destroy()
		inFilePage := fmt.Sprintf("%s[%d]", bookFile, *curPage)

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
			olog.Printf("Connect page %d and %d together\n", *curPage, *curPage+1)
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
	fileFmt := fmt.Sprintf("%%s/Images/page-%%0%dd", numDigits(pageCount))
	outFilePage := fmt.Sprintf(fileFmt, workDir, *outPage)

	if err := postProcessingPage(current, outFilePage); err != nil {
		return err
	}

	*curPage += processed
	*outPage += 1
	if next != nil {
		*itr = next.Clone() // next page will become current in the next process
	} else {
		*itr = nil
	}

	return nil
}

func ifConnect(left *Page, right *Page) (bool, error) {
	// TODO: Implement connectness checking
	return false, nil
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

func postProcessingPage(p *Page, outFilePage string) error {
	// Trim image with fuzz
	if err := trimPage(p, 1.0-trimMaxP, fuzzP, bgColor, outFilePage); err != nil {
		return fmt.Errorf("trimming page: %w", err)
	}

	// Resize page to aspect fit screen
	if err := resizePage(p, targetSize); err != nil {
		return fmt.Errorf("resizing page to fit to screen: %w", err)
	}

	// TODO: Convert to grayscale

	// Save as raw image
	outFile := fmt.Sprintf("%s.png", outFilePage)
	if err := p.WriteImage(outFile); err != nil {
		return fmt.Errorf("writing page to image file: %w", err)
	}
	return nil
}

func trimPage(p *Page, minSizeP float64, fuzzP float64, bgColor string, outFile string) error {
	// Trim image with fuzz
	pageSize := size(p)
	minSize := pageSize.ScaleBy(minSizeP)

	// write tmp file
	tmpFile := fmt.Sprintf("%s.trimming.png", outFile)
	if err := p.WriteImage(tmpFile); err != nil {
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
		"info:",
	})
	if err != nil {
		return fmt.Errorf("finding trim box: %w", err)
	}

	var tSize, tOffset Size
	if _, err := fmt.Sscanf(ret.Meta, "%dx%d+%d+%d", &(tSize.width), &(tSize.height), &(tOffset.width), &(tOffset.height)); err != nil {
		return fmt.Errorf("parsing trim box %s: %w", ret.Meta, err)
	}
	tOffset.TranslateBy(-1, -1) // compensate trim guide border
	// TODO: Add margin
	vlog.Printf("trim box: %s+%d+%d\n", tSize, tOffset.width, tOffset.height)

	if tSize == pageSize { // trim box equals page size
		vlog.Printf("No trimming needed\n")
		return nil
	}

	if !minSize.CanFitIn(tSize) {
		// smaller than min size after trimmed
		otSize := tSize

		// expand trim size in each dimension to reach minimum size
		expand := func(cur uint, off uint, mn uint, mx uint) (uint, uint) {
			if cur >= mn {
				return cur, off
			}
			gap := mn - cur
			leftover := uint(max(0, int(gap/2)-int(mx-(off+cur))))
			return mn, off - min(off, (gap/2)+leftover)
		}
		tSize.width, tOffset.width = expand(tSize.width, tOffset.width, minSize.width, pageSize.width)
		tSize.height, tOffset.height = expand(tSize.height, tOffset.height, minSize.height, pageSize.height)

		olog.Printf("Page size %s is trimmed to %s but it is smaller than minimum size %s - expanding trim box to minimum %s\n", pageSize, otSize, minSize, tSize)
	}

	// Crop based on trim size
	if err := p.CropImage(tSize.width, tSize.height, int(tOffset.width), int(tOffset.height)); err != nil {
		return fmt.Errorf("trimming page with %s+%d+%d: %w", tSize, tOffset.width, tOffset.height, err)
	}

	// Print trim info
	tWidthP := float64(tSize.width) * 100.0 / float64(pageSize.width)
	tHeightP := float64(tSize.height) * 100.0 / float64(pageSize.height)
	olog.Printf("Page size %s is trimmed to %s (%.2f%% | %.2f%%)\n", pageSize, tSize, tWidthP, tHeightP)

	return nil
}

func resizePage(p *Page, screen Size) error {
	// Trim image with fuzz
	pageSize := size(p)
	pgOrient := pageSize.Orientation()
	scrOrient := screen.Orientation()
	if pgOrient != Square && pgOrient != scrOrient {
		// rotate counter-clockwise
		olog.Printf("Rotating page because page orientation %s (%s) does not match screen orientation (%s)\n", pageSize, pgOrient, scrOrient)
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
