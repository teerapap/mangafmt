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
	"github.com/teerapap/mangafmt/internal/volume/format"
)

// InputSource reads pages from an input volume file. Each supported input file
// format has its own implementation.
type InputSource interface {
	// Name is the input format name. It is used in log messages.
	Name() string

	// PageCount is the total number of pages in the input file.
	PageCount() int

	// Metadata is the volume information found in the input file. Its fields
	// are left empty if the input file has no such information.
	Metadata() SourceMetadata

	// LoadImage loads the image of the page number(1-based) in the input file.
	LoadImage(pageNo int, cfg Config, workDir string, logger log.Logger) (image.Image, error)
}

// SourceMetadata is the volume information read from the input file
type SourceMetadata struct {
	Title      string
	Author     string
	Language   string
	Identifier string
	IsRTL      *bool // nil means the input file does not specify the read direction

	// Epub is the metadata which only an epub input file has
	Epub format.EpubMetadata
}

func boolPtr(v bool) *bool {
	return &v
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
