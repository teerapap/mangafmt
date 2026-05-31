//
// page.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package volume

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/teerapap/mangafmt/internal/log"

	"github.com/teerapap/mangafmt/internal/util"
	"github.com/teerapap/mangafmt/internal/volume/format"
)

type Page struct {
	img    image.Image
	volume *Volume

	PageNo      int
	OtherPageNo int // the other page number that this page connected with
}

func (p *Page) Destroy() {
	p.img = image.White
}

func (p Page) Rect() Rect {
	return Rect{Point{}, p.Size()}
}

func (p Page) Size() Size {
	return SizeFromBounds(p.img.Bounds())
}

func (p Page) Filename(suffix string) string {
	digits := util.DigitCount(p.volume.PageCount)
	if p.OtherPageNo > 0 { // two-page connected
		fileFmt := fmt.Sprintf("page-%%0%dd-%%0%dd%%s", digits, digits)
		return fmt.Sprintf(fileFmt, p.PageNo, p.OtherPageNo, suffix)
	} else {
		fileFmt := fmt.Sprintf("page-%%0%dd%%s", digits)
		return fmt.Sprintf(fileFmt, p.PageNo, suffix)
	}
}

func (p Page) Filepath(dir string, suffix string) string {
	return fmt.Sprintf("%s/%s", dir, p.Filename(suffix))
}

func (p *Page) LeftRight(other *Page) (left *Page, right *Page) {
	isRTL := p.volume.Config.IsRTL
	left = p
	right = other
	if isRTL {
		if left.PageNo < right.PageNo {
			left, right = right, left
		}
	} else {
		if left.PageNo > right.PageNo {
			left, right = right, left
		}
	}
	return
}

func (p Page) WriteFile(filepath string, logger log.Logger) error {
	// Save as raw image
	logger.Info("Writing to filesystem")
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("create image file %s: %w", filepath, err)
	}
	defer f.Close()

	if err := png.Encode(f, p.img); err != nil {
		return fmt.Errorf("writing page to image file %s: %w", filepath, err)
	}

	return nil
}

func (p *Page) Format(cfg FormatConfig, keepOrientation bool, logger log.Logger) (*format.Page, error) {

	// Trim image with fuzz
	if err := p.Trim(cfg.Trim, logger); err != nil {
		return nil, fmt.Errorf("trimming page: %w", err)
	}

	// Resize page to aspect fit screen
	if err := p.ResizeToFit(cfg.Resize, keepOrientation, logger); err != nil {
		return nil, fmt.Errorf("resizing page to fit to screen: %w", err)
	}

	// Convert to grayscale
	if err := p.ConvertToGrayscale(cfg.Grayscale, logger); err != nil {
		return nil, fmt.Errorf("converting page to grayscale: %w", err)
	}

	// Write to filesystem
	filepath := p.Filepath(cfg.WorkDir, ".png")
	err := p.WriteFile(filepath, logger)
	if err != nil {
		return nil, fmt.Errorf("writing to filesystem: %w", err)
	}

	// Create formatted page struct
	outPage := &format.Page{
		Id:        p.Filename(""),
		Filepath:  filepath,
		MediaType: "image/png",
		Size:      format.Size(p.Size()),
	}

	return outPage, nil
}
