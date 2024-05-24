//
// connect.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"

	"github.com/teerapap/mangafmt/internal/log"
	"gopkg.in/gographics/imagick.v2/imagick"
)

type ConnectConfig struct {
	Enabled    bool
	EdgeWidth  uint
	EdgeMargin uint
	BgDistort  float64
	LrDistort  float64
}

func (left *Page) CanConnect(right *Page, cfg ConnectConfig) (bool, error) {
	lpEdge := left.Rect().RightEdge(cfg.EdgeWidth, cfg.EdgeMargin)
	rpEdge := right.Rect().LeftEdge(cfg.EdgeWidth, cfg.EdgeMargin)

	if lpEdge.size != rpEdge.size {
		log.Printf("[Connect] Two pages (%d <-> %d) are not connected because both edges are not the same size - left(%s) != right(%s)", left.PageNo, right.PageNo, lpEdge.size, rpEdge.size)
		return false, nil
	} else if lpEdge.size.Width == 0 {
		log.Printf("[Connect] Two pages (%d <-> %d) are not connected because both pages are not wide enough - left(%s), right(%s)", left.PageNo, right.PageNo, lpEdge.size, rpEdge.size)
		return false, nil
	}

	edge := lpEdge.size

	// Prepare background canvas for comparison
	bgCanvas := imagick.NewMagickWand()
	defer bgCanvas.Destroy()
	if err := bgCanvas.SetSize(edge.Width, edge.Height); err != nil {
		return false, fmt.Errorf("setting background canvas size %s: %w", edge, err)
	}
	bgColor := left.book.Config.BgColor
	if err := bgCanvas.ReadImage(fmt.Sprintf("canvas:%s", bgColor)); err != nil {
		return false, fmt.Errorf("creating background canvas: %w", err)
	}

	// Create left edge
	mwLeft := left.mw.Clone()
	defer mwLeft.Destroy()
	if err := mwLeft.CropImage(lpEdge.size.Width, lpEdge.size.Height, lpEdge.origin.X, lpEdge.origin.Y); err != nil {
		return false, fmt.Errorf("getting edge of left page(%d) with %s: %w", left.PageNo, lpEdge, err)
	}

	// Compare left vs background canvas
	distortion, err := mwLeft.GetImageDistortion(bgCanvas, imagick.METRIC_ROOT_MEAN_SQUARED_ERROR)
	if err != nil {
		return false, fmt.Errorf("calculating image distortion(RMSE) between left(%d) and background: %w", left.PageNo, err)
	}
	if distortion <= cfg.BgDistort {
		// edge is all background
		log.Printf("[Connect] Left page(%d) edge has background border - distortion(%f) is below threshold(%f)", left.PageNo, distortion, cfg.BgDistort)
		return false, nil
	}

	// Create right edge
	mwRight := right.mw.Clone()
	defer mwRight.Destroy()
	if err := mwRight.CropImage(rpEdge.size.Width, rpEdge.size.Height, rpEdge.origin.X, rpEdge.origin.Y); err != nil {
		return false, fmt.Errorf("getting edge of right page with %s: %w", rpEdge, err)
	}
	// Compare right vs background canvas
	distortion, err = mwRight.GetImageDistortion(bgCanvas, imagick.METRIC_ROOT_MEAN_SQUARED_ERROR)
	if err != nil {
		return false, fmt.Errorf("calculating image distortion(RMSE) between right(%d) and background: %w", right.PageNo, err)
	}
	if distortion <= cfg.BgDistort {
		// edge is all background
		log.Printf("[Connect] Right page(%d) edge has background border - distortion(%f) is below threshold(%f)", right.PageNo, distortion, cfg.BgDistort)
		return false, nil
	}

	// Compare left page edge vs right page edge
	distortion, err = mwLeft.GetImageDistortion(mwRight, imagick.METRIC_ROOT_MEAN_SQUARED_ERROR)
	if err != nil {
		return false, fmt.Errorf("calculating image distortion(RMSE) between left(%d) and right(%d): %w", left.PageNo, right.PageNo, err)
	}
	if distortion > cfg.LrDistort {
		// have connection
		log.Printf("[Connect] Left page(%d) edge and right page edge(%d) do not connect - distortion(%f) is more than threshold(%f)", left.PageNo, right.PageNo, distortion, cfg.LrDistort)
		return false, nil
	}
	log.Printf("[Connect] Page %d and %d are connected! - distortion=%f", left.PageNo, right.PageNo, distortion)

	return true, nil
}

func (left *Page) Connect(right *Page) (*Page, error) {
	canvas := imagick.NewMagickWand()
	defer canvas.Destroy()

	if err := canvas.AddImage(left.mw); err != nil {
		return nil, fmt.Errorf("connecting left page: %w", err)
	}
	if err := canvas.AddImage(right.mw); err != nil {
		return nil, fmt.Errorf("connecting rigt page: %w", err)
	}
	canvas.ResetIterator()

	connected := canvas.AppendImages(false)

	newPage := &Page{
		mw:          connected,
		book:        left.book,
		PageNo:      min(left.PageNo, right.PageNo),
		OtherPageNo: max(left.PageNo, right.PageNo),
	}
	return newPage, nil
}

// for debugging
func printDistortions(mw1 *imagick.MagickWand, name1 string, mw2 *imagick.MagickWand, name2 string, fuzzP float64) {
	fuzz := FuzzFromPercent(fuzzP)
	if err := mw1.SetImageFuzz(fuzz); err != nil {
		log.Verbosef("[Connect] Setting %s page fuzz %f: %s", name1, fuzz, err)
		return
	}

	mnames := map[imagick.MetricType]string{
		imagick.METRIC_ABSOLUTE_ERROR:                     "AE",
		imagick.METRIC_MEAN_ABSOLUTE_ERROR:                "MAE",
		imagick.METRIC_MEAN_ERROR_PER_PIXEL:               "MEPP",
		imagick.METRIC_MEAN_SQUARED_ERROR:                 "MSE",
		imagick.METRIC_PEAK_ABSOLUTE_ERROR:                "PAE",
		imagick.METRIC_PEAK_SIGNAL_TO_NOISE_RATIO:         "PSNR",
		imagick.METRIC_ROOT_MEAN_SQUARED_ERROR:            "RMSE",
		imagick.METRIC_NORMALIZED_CROSS_CORRELATION_ERROR: "NCCE",
		imagick.METRIC_FUZZ_ERROR:                         "FUZZ",
	}

	metrics := []imagick.MetricType{
		imagick.METRIC_ABSOLUTE_ERROR,
		imagick.METRIC_MEAN_ABSOLUTE_ERROR,
		imagick.METRIC_MEAN_ERROR_PER_PIXEL,
		imagick.METRIC_MEAN_SQUARED_ERROR,
		imagick.METRIC_ROOT_MEAN_SQUARED_ERROR,
	}

	log.Verbosef("[Connect] Distortion between %s vs %s", name1, name2)
	for _, m := range metrics {
		mn := mnames[m]
		distortion, err := mw1.GetImageDistortion(mw2, m)
		if err != nil {
			log.Verbosef("[Connect] %s: %s", mn, err)
		} else {
			log.Verbosef("[Connect] %s: %f", mn, distortion)
		}
	}
}
