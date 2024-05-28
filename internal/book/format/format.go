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

func (f OutputFormat) Ext() string {
	switch f {
	case RAW:
		return ""
	case CBZ:
		return "cbz"
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
	default:
		return fmt.Errorf("Unknown format: %s", val)
	}
	return nil
}
