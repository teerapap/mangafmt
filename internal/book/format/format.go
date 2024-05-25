//
// format.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package format

import (
	"github.com/teerapap/mangafmt/internal/book"
)

type Page struct {
	Filepath string
	Size     book.Size
}

type OutputFormat int

const (
	RAW = iota
	CBZ
)

func (f OutputFormat) String() string {
	switch f {
	case RAW:
		return "raw"
	case CBZ:
		return "cbz"
	default:
		return "unknown"
	}
}

func AllOutputFormats() []OutputFormat {
	// exclude RAW
	return []OutputFormat{CBZ}
}
