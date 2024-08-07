//
// page.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/teerapap/mangafmt/internal/log"
)

type Page struct {
	img  image.Image
	book *Book

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

func digitCount(total int) int {
	d := 1
	for ; total >= 10; total = total / 10 {
		d += 1
	}
	return d
}

func (p Page) Filename(suffix string) string {
	digits := digitCount(p.book.PageCount)
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
	isRTL := p.book.Config.IsRTL
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

func (p Page) WriteFile(dir string) (string, string, error) {
	// Save as raw image
	log.Printf("[Save] Writing to filesystem")
	filename := p.Filepath(dir, ".png")
	f, err := os.Create(filename)
	if err != nil {
		return "", "", fmt.Errorf("create image file %s: %w", filename, err)
	}
	defer f.Close()

	if err := png.Encode(f, p.img); err != nil {
		return "", "", fmt.Errorf("writing page to image file %s: %w", filename, err)
	}

	return filename, "image/png", nil
}
