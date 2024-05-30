//
// book.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/teerapap/mangafmt/internal/util"
	"gopkg.in/gographics/imagick.v2/imagick"
	"rsc.io/pdf"
)

type Book struct {
	Filepath  string
	Title     string
	PageCount int
	Config    BookConfig
}

type BookConfig struct {
	Density float64
	IsRTL   bool
	BgColor string
}

func NewBook(path string, config BookConfig) (*Book, error) {

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

	return &Book{
		Filepath:  path,
		Title:     title,
		PageCount: r.NumPage(),
		Config:    config,
	}, nil
}

func (b *Book) LoadPage(pageNo int) (*Page, error) {
	pageFile := fmt.Sprintf("%s[%d]", b.Filepath, pageNo-1)

	mw := imagick.NewMagickWand()
	defer mw.Destroy()

	// Read page
	if err := mw.SetResolution(b.Config.Density, b.Config.Density); err != nil {
		return nil, fmt.Errorf("setting page resolution %f: %w", b.Config.Density, err)
	}
	if err := mw.ReadImage(pageFile); err != nil {
		return nil, fmt.Errorf("reading input file(%s) at page %d: %w", b.Filepath, pageNo, err)
	}
	page := &Page{
		mw:     mw.Clone(),
		book:   b,
		PageNo: pageNo,
	}
	return page, nil
}
