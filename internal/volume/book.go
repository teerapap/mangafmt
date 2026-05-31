//
// volume.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package volume

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/util"
	"github.com/teerapap/mangafmt/internal/volume/format"
	"rsc.io/pdf"
)

type Volume struct {
	Filepath  string
	Info      Info
	PageCount int
	Config    Config
	lruCache  *lru.Cache[int, Page]
	extractor PdfPageExtractor
}

type Info struct {
	Title  string
	Author string
}

type Config struct {
	Density float64
	IsRTL   bool
}

func NewVolume(path string, info Info, cfg Config, logger log.Logger) (*Volume, error) {

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening input pdf file: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("checking input pdf file size: %w", err)
	}
	r, err := pdf.NewReader(f, fi.Size())
	if err != nil {
		return nil, fmt.Errorf("reading input pdf file: %w", err)
	}
	info.Title = strings.TrimSpace(info.Title)
	if info.Title == "" {
		info.Title = util.NameWithoutExt(filepath.Base(path))
	}
	info.Author = strings.TrimSpace(info.Author)

	extractor, err := FindPdfExtractor(logger)
	if err != nil {
		return nil, err
	}

	lruCache, err := lru.New[int, Page](2)
	if err != nil {
		return nil, fmt.Errorf("reading input pdf file: %w", err)
	}

	return &Volume{
		Filepath:  path,
		Info:      info,
		PageCount: r.NumPage(),
		Config:    cfg,
		lruCache:  lruCache,
		extractor: extractor,
	}, nil
}

func (v *Volume) LoadPage(pageNo int, workDir string, logger log.Logger) (*Page, error) {
	// load from cache first
	cachedPage, found := v.lruCache.Get(pageNo)
	if found {
		logger.Debug("Loading page from cache -", "page_no", pageNo)
		return &cachedPage, nil
	}

	// create temp directory
	tmpFile, err := os.CreateTemp(workDir, "mangafmt-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("create tmp file for input file(%s) at page %d: %w", v.Filepath, pageNo, err)
	}
	filename := tmpFile.Name()
	defer os.RemoveAll(filename)
	defer tmpFile.Close()

	// extract page from pdf file
	logger.Info("Loading page -", "page_no", pageNo, "tool", v.extractor.Name())
	if err = v.extractor.Extract(v.Filepath, pageNo, v.Config.Density, filename, logger); err != nil {
		return nil, fmt.Errorf("extracting pdf page to tmp file %s: %w", filename, err)
	}

	// load image file
	img, format, err := image.Decode(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("loading tmp image file %s: %w", filename, err)
	}
	logger.Debug("Loaded page -", "page_no", pageNo, "file", filename, "format", format, "size", img.Bounds())

	page := Page{
		img:    img,
		volume: v,
		PageNo: pageNo,
	}

	// save to cache
	v.lruCache.Add(pageNo, page)

	return &page, nil
}

type FormatConfig struct {
	Spread    SpreadConfig
	Trim      TrimConfig
	Resize    ResizeConfig
	Grayscale GrayscaleConfig
	WorkDir   string
}

func (v *Volume) Format(pr PageRange, cfg FormatConfig, logger log.Logger) (*format.Volume, error) {
	// For loop each page
	partials := pr.PageCount() != v.PageCount
	if partials {
		logger.Infof("Start formatting page(s) in range %s. Total %d page(s).", pr, pr.PageCount())
	} else {
		logger.Infof("Start formatting. Total %d page(s).", pr.PageCount())
	}

	outPages := make([]format.Page, 0, v.PageCount)
	for pageNo, i := 1, 1; pageNo <= v.PageCount; {
		if !pr.Contains(pageNo) {
			pageNo += 1
			continue
		}
		if partials {
			logger.Infof("Formatting page %d....(%d/%d)", pageNo, i, pr.PageCount())
		} else {
			logger.Infof("Formatting page....(%d/%d)", pageNo, pr.PageCount())
		}

		pageLogger := logger.Indent(fmt.Sprintf("> Page[%d] ", pageNo))

		outPage, formatted, err := v.formatPage(pageNo, pr, cfg, pageLogger)
		if err != nil {
			return nil, fmt.Errorf("formatting page %d: %w", pageNo, err)
		}
		outPages = append(outPages, outPage...)
		pageNo += formatted
		i += formatted
		logger.Debug("Done formatting page -", "next_input_page", pageNo, "next_output_page", len(outPages))
	}
	logger.Info("Done formatting volume -", "total_input_pages", pr.PageCount(), "total_output_pages", len(outPages))

	return &format.Volume{
		Title:  v.Info.Title,
		Author: v.Info.Author,
		IsRTL:  v.Config.IsRTL,
		Pages:  outPages,
	}, nil
}

func (v *Volume) formatPage(pageNo int, pr PageRange, cfg FormatConfig, logger log.Logger) ([]format.Page, int, error) {
	formatted := 0
	current, err := v.LoadPage(pageNo, cfg.WorkDir, logger)
	if err != nil {
		return nil, 0, fmt.Errorf("loading page %d: %w", pageNo, err)
	}
	defer current.Destroy()

	formatted += 1

	connected := false
	var next *Page = nil

	outPages := make([]format.Page, 0, 3)

	// Look ahead next page
	if pr.Contains(pageNo+1) && cfg.Spread.Enabled { // has next page
		// Read next page
		next, err = v.LoadPage(pageNo+1, cfg.WorkDir, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("loading next page %d: %w", pageNo+1, err)
		}
		defer next.Destroy()

		// Check if the next page can merge with current page
		left, right := current.LeftRight(next)
		connected, err = left.IsDoublePageSpread(right, cfg.Spread, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("checking if two pages are double-page spread: %w", err)
		}
		if connected {
			// connect two pages
			spread, err := left.Connect(right, logger)
			if err != nil {
				return nil, 0, fmt.Errorf("connecting two pages: %w", err)
			}
			defer spread.Destroy()
			formatted += 1

			// format the spread page
			outPage, err := spread.Format(cfg, cfg.Spread.KeepOrientation, logger)
			if err != nil {
				return nil, 0, fmt.Errorf("formatting spread page: %w", err)
			}
			outPages = append(outPages, *outPage)
		}
	}

	if !connected || cfg.Spread.KeepOriginal {
		// format current page
		outPage, err := current.Format(cfg, false, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("formatting current page: %w", err)
		}
		outPages = append(outPages, *outPage)
	}

	if connected && cfg.Spread.KeepOriginal {
		// format next page
		outPage, err := next.Format(cfg, false, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("formatting original next page: %w", err)
		}
		outPages = append(outPages, *outPage)
	}

	return outPages, formatted, nil
}
