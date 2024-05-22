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

func rect(p *Page) Rect {
	return Rect{Point{}, size(p)}
}

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
		var leftNum = *curPage
		var right *Page = next
		var rightNum = *curPage + 1
		if isRTL {
			left, right = right, left
			leftNum, rightNum = rightNum, leftNum
		}
		connected, err := ifConnect(left, leftNum, right, rightNum)
		if err != nil {
			return fmt.Errorf("checking if two pages are connected: %w", err)
		}
		if connected {
			olog.Printf("Connect page %d and %d together\n", leftNum, rightNum)
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

func ifConnect(left *Page, leftNum int, right *Page, rightNum int) (bool, error) {
	lpEdge := rect(left).RightEdge(edgeWidth, edgeMargin)
	rpEdge := rect(right).LeftEdge(edgeWidth, edgeMargin)

	if lpEdge.size != rpEdge.size {
		olog.Printf("Two pages (%d <-> %d) are not connected because edge are not the same size - left(%s) != right(%s)\n", leftNum, rightNum, lpEdge.size, rpEdge.size)
		return false, nil
	} else if lpEdge.size.width == 0 {
		olog.Printf("Two pages (%d <-> %d) are not connected because page is not wide enough - left(%s), right(%s)\n", leftNum, rightNum, lpEdge.size, rpEdge.size)
		return false, nil
	}

	edge := lpEdge.size

	// Prepare background canvas for comparison
	bgCanvas := imagick.NewMagickWand()
	defer bgCanvas.Destroy()
	if err := bgCanvas.SetSize(edge.width, edge.height); err != nil {
		return false, fmt.Errorf("setting background canvas size %s: %w", edge, err)
	}
	if err := bgCanvas.ReadImage(fmt.Sprintf("canvas:%s", bgColor)); err != nil {
		return false, fmt.Errorf("creating background canvas: %w", err)
	}

	// Create left edge
	left = left.Clone()
	defer left.Destroy()
	if err := left.CropImage(lpEdge.size.width, lpEdge.size.height, lpEdge.origin.x, lpEdge.origin.y); err != nil {
		return false, fmt.Errorf("getting edge of left page(%d) with %s: %w", leftNum, lpEdge, err)
	}
	fuzz := fuzzFromPercent(fuzzP)
	if err := left.SetImageFuzz(fuzz); err != nil {
		return false, fmt.Errorf("setting left page fuzz %f: %w", fuzz, err)
	}

	// Compare left vs background canvas
	distortion, err := left.GetImageDistortion(bgCanvas, imagick.METRIC_ROOT_MEAN_SQUARED_ERROR)
	if err != nil {
		return false, fmt.Errorf("calculating image distortion(RMSE) between left(%d) and background: %w", leftNum, err)
	}
	if distortion < 0.1 {
		// edge is all background
		olog.Printf("Left page(%d) edge matches with background color - rmse=%f\n", leftNum, distortion)
		return false, nil
	}
	// TODO: Compare right vs background canvas as well

	// Create right edge
	right = right.Clone()
	defer right.Destroy()
	if err := right.CropImage(rpEdge.size.width, rpEdge.size.height, rpEdge.origin.x, rpEdge.origin.y); err != nil {
		return false, fmt.Errorf("getting edge of right page with %s: %w", rpEdge, err)
	}

	// Compare left page edge vs right page edge
	distortion, err = left.GetImageDistortion(right, imagick.METRIC_ROOT_MEAN_SQUARED_ERROR)
	if err != nil {
		return false, fmt.Errorf("calculating image distortion(RMSE) between left(%d) and right(%d): %w", leftNum, rightNum, err)
	}
	// TODO: Make this distortion condition a command argument
	if distortion > 0.2 {
		// have connection
		olog.Printf("Left page(%d) edge and right page edge(%d) does not connect - rmse=%f\n", leftNum, rightNum, distortion)
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

func postProcessingPage(p *Page, outFilePage string) error {
	// Trim image with fuzz
	if err := trimPage(p, trimMinSizeP, fuzzP, bgColor, outFilePage); err != nil {
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
	pageRect := Rect{size: size(p)}
	minSize := pageRect.size.ScaleBy(minSizeP)

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

	var trimRect Rect
	if _, err := fmt.Sscanf(ret.Meta, "%dx%d+%d+%d", &(trimRect.size.width), &(trimRect.size.height), &(trimRect.origin.x), &(trimRect.origin.y)); err != nil {
		return fmt.Errorf("parsing trim box %s: %w", ret.Meta, err)
	}
	trimRect = trimRect.
		TranslateBy(-1, -1).               // remove trim guiding border
		InsetBy(-trimMargin, -trimMargin). // add safety margin
		BoundBy(pageRect)                  // bound by page rect

	vlog.Printf("trim box: %s\n", trimRect)

	if trimRect == pageRect { // trim box equals page rect
		vlog.Printf("No trimming needed\n")
		return nil
	}

	if !minSize.CanFitIn(trimRect.size) {
		// smaller than min size after trimmed
		oldRect := trimRect

		gapX := max(0, int(minSize.width)-int(trimRect.size.width))
		gapY := max(0, int(minSize.height)-int(trimRect.size.height))
		trimRect = trimRect.
			InsetBy(-gapX/2, -gapY/2). // expand each side to minimum size
			MoveInside(pageRect)       // Move the rect to fit inside page rect frame as much as possible

		olog.Printf("Page size %s is trimmed by %s but it is smaller than minimum size %s - expanding trim box to minimum %s\n", pageRect.size, oldRect, minSize, trimRect)
	}

	// Crop based on trim size
	if err := p.CropImage(trimRect.size.width, trimRect.size.height, trimRect.origin.x, trimRect.origin.y); err != nil {
		return fmt.Errorf("trimming page with %s: %w", trimRect, err)
	}

	// Print trim info
	tWidthP := float64(trimRect.size.width) * 100.0 / float64(pageRect.size.width)
	tHeightP := float64(trimRect.size.height) * 100.0 / float64(pageRect.size.height)
	olog.Printf("Page size %s is trimmed by %s (%.2f%% | %.2f%%)\n", pageRect.size, trimRect, tWidthP, tHeightP)

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
