//
// spread.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"

	"github.com/teerapap/mangafmt/internal/imgutil"
	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/spread"
)

type SpreadConfig struct {
	Enabled         bool
	KeepOrientation bool
	KeepOriginal    bool
	EdgeWidth       int
	Confidence      float64
}

func (left *Page) IsDoublePageSpread(right *Page, cfg SpreadConfig) (bool, error) {
	sd := spread.NewSpreadDetector()
	sd.EdgeStripWidth = cfg.EdgeWidth
	sd.SpreadThreshold = cfg.Confidence
	result, err := sd.IsTwoPageSpread(left.img, right.img)
	if err != nil {
		return false, fmt.Errorf("check two-page spread: %w", err)
	}
	if result.IsSpread {
		// they are double-page spread
		log.Printf("[Spread] Page %d and %d are DOUBLE-PAGE SPREAD! - %+v", left.PageNo, right.PageNo, result)
	} else {
		log.Printf("[Spread] Page %d and %d are NOT double-page spread - %+v", left.PageNo, right.PageNo, result)
	}

	return result.IsSpread, nil
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
