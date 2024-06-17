//
// format.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package format

import (
	"fmt"
	"strings"

	"github.com/teerapap/mangafmt/internal/book"
)

type Page struct {
	Id        string
	Filepath  string
	MediaType string
	Size      book.Size
}

type OutputFormat int

const (
	RAW = iota
	CBZ
	EPUB
	KEPUB
)

func (f OutputFormat) String() string {
	switch f {
	case RAW:
		return "raw"
	case CBZ:
		return "cbz"
	case EPUB:
		return "epub"
	case KEPUB:
		return "kepub"
	default:
		return "unknown"
	}
}

func (f OutputFormat) Ext() string {
	switch f {
	case RAW:
		return ""
	case CBZ:
		return "cbz"
	case EPUB:
		return "epub"
	case KEPUB:
		return "kepub"
	default:
		return ""
	}
}

func (f *OutputFormat) Set(val string) error {
	switch strings.ToLower(val) {
	case "raw":
		*f = RAW
	case "cbz":
		*f = CBZ
	case "epub":
		*f = EPUB
	case "kepub":
		*f = KEPUB
	default:
		return fmt.Errorf("unknown format: %s", val)
	}
	return nil
}
