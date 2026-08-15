//
// source.go
// Copyright (C) 2026 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package volume

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/teerapap/mangafmt/internal/log"
)

// InputSource reads pages from an input volume file. Each supported input file
// format has its own implementation.
type InputSource interface {
	// Name is the input format name. It is used in log messages.
	Name() string

	// PageCount is the total number of pages in the input file.
	PageCount() int

	// LoadImage loads the image of the page number(1-based) in the input file.
	LoadImage(pageNo int, cfg Config, workDir string, logger log.Logger) (image.Image, error)
}

// openInputSource creates an [InputSource] for the input file. The input file
// format is determined by its file extension.
func openInputSource(path string, logger log.Logger) (InputSource, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return newPdfSource(path, logger)
	case ".epub", ".kepub":
		return newEpubSource(path, logger)
	default:
		return nil, fmt.Errorf("unsupported input file extension(%s). The supported extensions are .pdf, .epub and .kepub", ext)
	}
}
