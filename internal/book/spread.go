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

func (left *Page) IsDoublePageSpread(right *Page, cfg SpreadConfig, logger log.Logger) (bool, error) {
	logger = logger.Indent("> Spread ")
	sd := spread.NewSpreadDetector()
	sd.EdgeStripWidth = cfg.EdgeWidth
	sd.SpreadThreshold = cfg.Confidence
	result, err := sd.IsTwoPageSpread(left.img, right.img)
	if err != nil {
		return false, fmt.Errorf("check two-page spread: %w", err)
	}

	logger.Debug("Spread detection", "result", result)
	if result.IsSpread {
		// they are double-page spread
		logger.Info("DOUBLE-PAGE SPREAD!", "left", left.PageNo, "right", right.PageNo, "reason", result.Reason)
	} else {
		logger.Info("NOT double-page spread", "left", left.PageNo, "right", right.PageNo, "reason", result.Reason)
	}

	return result.IsSpread, nil
}

func (left *Page) Connect(right *Page, logger log.Logger) (*Page, error) {
	logger = logger.Indent("> Spread ")
	connected := imgutil.AppendHorizontally(left.img, right.img, logger)

	newPage := &Page{
		img:         connected,
		book:        left.book,
		PageNo:      min(left.PageNo, right.PageNo),
		OtherPageNo: max(left.PageNo, right.PageNo),
	}
	return newPage, nil
}
