//
// spread.go
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

type SpreadConfig struct {
	Enabled    bool
	EdgeWidth  uint
	EdgeMargin uint
	BgDistort  []float64
	LrDistort  float64
}

func (left *Page) IsDoublePageSpread(right *Page, cfg SpreadConfig) (bool, error) {
	lpEdge := left.Rect().RightEdge(cfg.EdgeWidth, cfg.EdgeMargin)
	rpEdge := right.Rect().LeftEdge(cfg.EdgeWidth, cfg.EdgeMargin)

	if lpEdge.size != rpEdge.size {
		log.Printf("[Spread] Two pages (%d and %d) are not connected because both edges are not the same size - left(%s) != right(%s)", left.PageNo, right.PageNo, lpEdge.size, rpEdge.size)
		return false, nil
	} else if lpEdge.size.Width == 0 {
		log.Printf("[Spread] Two pages (%d and %d) are not connected because both pages are not wide enough - left(%s), right(%s)", left.PageNo, right.PageNo, lpEdge.size, rpEdge.size)
		return false, nil
	}

	// Prepare background canvas for comparison
	for i, bgColor := range left.book.Config.BgColor {
		bgDistort := cfg.BgDistort[min(i, len(cfg.BgDistort)-1)]
		bgCanvas := image.NewUniform(bgColor)
		// Compare left vs background canvas
		distortion := imgutil.GetRMSEDistortion(left.img, lpEdge.ToRectangle(), bgCanvas, image.Point{})
		if distortion <= bgDistort {
			// edge is all background
			log.Printf("[Spread] Left page(%d) edge has background border (%s) - distortion(%f) is below threshold(%f)", left.PageNo, imgutil.ToHexString(bgColor), distortion, bgDistort)
			return false, nil
		}
		log.Verbosef("[Spread] Left page(%d) edge does not have background border (%s) - distortion(%f) is higher than threshold(%f)", left.PageNo, imgutil.ToHexString(bgColor), distortion, bgDistort)

		// Compare right vs background canvas
		distortion = imgutil.GetRMSEDistortion(right.img, rpEdge.ToRectangle(), bgCanvas, image.Point{})
		if distortion <= bgDistort {
			// edge is all background
			log.Printf("[Spread] Right page(%d) edge has background border (%s) - distortion(%f) is below threshold(%f)", right.PageNo, imgutil.ToHexString(bgColor), distortion, bgDistort)
			return false, nil
		}
		log.Verbosef("[Spread] Right page(%d) edge does not have background border (%s) - distortion(%f) is higher than threshold(%f)", right.PageNo, imgutil.ToHexString(bgColor), distortion, bgDistort)
	}

	// Compare left page edge vs right page edge
	distortion := imgutil.GetRMSEDistortion(left.img, lpEdge.ToRectangle(), right.img, rpEdge.ToRectangle().Min)
	if distortion > cfg.LrDistort {
		log.Printf("[Spread] Left page(%d) edge and right page edge(%d) do not connect - distortion(%f) is more than threshold(%f)", left.PageNo, right.PageNo, distortion, cfg.LrDistort)
		return false, nil
	}
	// they are double-page spread
	log.Printf("[Spread] Page %d and %d are double-page spread! - distortion(%f) is below threshold(%f)", left.PageNo, right.PageNo, distortion, cfg.LrDistort)

	return true, nil
}

func (left *Page) Connect(right *Page) (*Page, error) {
	connected := imgutil.AppendHorizontally(left.img, right.img)

	newPage := &Page{
		img:         connected,
		book:        left.book,
		PageNo:      min(left.PageNo, right.PageNo),
		OtherPageNo: max(left.PageNo, right.PageNo),
	}
	return newPage, nil
}
