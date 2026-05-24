//
// book.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/util"
	"rsc.io/pdf"
)

type Book struct {
	Filepath  string
	Title     string
	Author    string
	PageCount int
	Config    BookConfig
	lruCache  *lru.Cache[int, Page]
	extractor PageExtractor
}

type BookConfig struct {
	Density float64
	IsRTL   bool
}

func NewBook(path string, config BookConfig, logger log.Logger) (*Book, error) {

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
	title := util.NameWithoutExt(filepath.Base(path))

	extractor, err := FindExtractor(logger)
	if err != nil {
		return nil, err
	}

	lruCache, err := lru.New[int, Page](2)
	if err != nil {
		return nil, fmt.Errorf("reading input pdf file: %w", err)
	}

	return &Book{
		Filepath:  path,
		Title:     title,
		PageCount: r.NumPage(),
		Config:    config,
		lruCache:  lruCache,
		extractor: extractor,
	}, nil
}

func (b *Book) LoadPage(pageNo int, logger log.Logger) (*Page, error) {
	// load from cache first
	cachedPage, found := b.lruCache.Get(pageNo)
	if found {
		logger.Debug("Loading page from cache -", "page_no", pageNo)
		return &cachedPage, nil
	}

	// create temp directory
	tmpFile, err := os.CreateTemp("", "mangafmt-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("create tmp file for input file(%s) at page %d: %w", b.Filepath, pageNo, err)
	}
	filename := tmpFile.Name()
	defer os.RemoveAll(filename)
	defer tmpFile.Close()

	// extract page from pdf file
	logger.Info("Loading page -", "page_no", pageNo, "tool", b.extractor.Name())
	if err = b.extractor.Extract(b.Filepath, pageNo, b.Config.Density, filename, logger); err != nil {
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
		book:   b,
		PageNo: pageNo,
	}

	// save to cache
	b.lruCache.Add(pageNo, page)

	return &page, nil
}

type PageExtractor interface {
	Name() string
	Detect() error
	Extract(inputFile string, page int, dpi float64, outputFile string, logger log.Logger) error
}

func FindExtractor(logger log.Logger) (PageExtractor, error) {
	extractors := []PageExtractor{vips{}, imagemagick7{}, imagemagick6{}}

	for _, ext := range extractors {
		if err := ext.Detect(); err != nil {
			logger.Debug("Cannot find tool -", "name", ext.Name(), "error", err)
		} else {
			// found the extractor
			logger.Debug("Found tool installed", "name", ext.Name())
			return ext, nil
		}
	}

	return nil, fmt.Errorf("either ImageMagick or VIPS(libvips) is required to extract page from pdf file")
}

type imagemagick6 struct {
}

func (i imagemagick6) Name() string {
	return "ImageMagick6"
}

func (i imagemagick6) Detect() error {
	path, err := exec.LookPath("convert")
	if strings.Contains(strings.ToLower(path), "system32") {
		// Windows system convert.exe
		return fmt.Errorf("ImageMagick6 convert utility is not found but convert.exe is found at %s", path)
	}
	return err
}

func (i imagemagick6) Extract(inputFile string, page int, dpi float64, outputFile string, logger log.Logger) error {
	logger = logger.Indent("> Load   ")
	pageFile := fmt.Sprintf("%s[%d]", inputFile, page-1)
	cmd := exec.Command("convert", "-density", fmt.Sprintf("%0.2f", dpi), "-define", "pdf:use-cropbox=true", "-auto-orient", pageFile, outputFile)
	out, err := cmd.CombinedOutput()
	logger.Debug("Run", "name", i.Name(), "cmd", cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	} else {
		logger.Debug("Done", "name", i.Name(), "output", cmd)
	}
	return nil
}

type imagemagick7 struct {
}

func (i imagemagick7) Name() string {
	return "ImageMagick7"
}

func (i imagemagick7) Detect() error {
	_, err := exec.LookPath("magick")
	return err
}

func (i imagemagick7) Extract(inputFile string, page int, dpi float64, outputFile string, logger log.Logger) error {
	logger = logger.Indent("> Load   ")
	pageFile := fmt.Sprintf("%s[%d]", inputFile, page-1)
	cmd := exec.Command("magick", "-density", fmt.Sprintf("%0.2f", dpi), "-define", "pdf:use-cropbox=true", "-auto-orient", pageFile, outputFile)
	out, err := cmd.CombinedOutput()
	logger.Debug("Run", "name", i.Name(), "cmd", cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	} else {
		logger.Debug("Done", "name", i.Name(), "output", cmd)
	}
	return nil
}

type vips struct {
}

func (v vips) Name() string {
	return "VIPS"
}

func (v vips) Detect() error {
	_, err := exec.LookPath("vips")
	return err
}

func (v vips) Extract(inputFile string, page int, dpi float64, outputFile string, logger log.Logger) error {
	logger = logger.Indent("> Load   ")
	pageFile := fmt.Sprintf("%s[page=%d,dpi=%0.2f]", inputFile, page-1, dpi)
	cmd := exec.Command("vips", "copy", pageFile, outputFile)
	out, err := cmd.CombinedOutput()
	logger.Debug("Run", "name", v.Name(), "cmd", cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	} else {
		logger.Debug("Done", "name", v.Name(), "output", cmd)
	}
	return nil
}
