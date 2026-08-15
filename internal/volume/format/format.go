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
)

type Volume struct {
	Title      string
	Author     string
	Language   string
	Identifier string
	IsRTL      bool
	Epub       EpubMetadata
	Pages      []Page
}

type Page struct {
	Id        string
	Filepath  string
	MediaType string
	Size      Size
}

type Size struct {
	Width  uint
	Height uint
}

type OutputFormat int

const (
	OutputFormatRaw = iota
	OutputFormatCbz
	OutputFormatEpub
	OutputFormatKepub
)

func (f OutputFormat) String() string {
	switch f {
	case OutputFormatRaw:
		return "raw"
	case OutputFormatCbz:
		return "cbz"
	case OutputFormatEpub:
		return "epub"
	case OutputFormatKepub:
		return "kepub"
	default:
		return "unknown"
	}
}

func (f OutputFormat) Ext() string {
	switch f {
	case OutputFormatRaw:
		return ""
	case OutputFormatCbz:
		return "cbz"
	case OutputFormatEpub:
		return "epub"
	case OutputFormatKepub:
		return "kepub"
	default:
		return ""
	}
}

func (f *OutputFormat) Set(val string) error {
	switch strings.ToLower(val) {
	case "raw":
		*f = OutputFormatRaw
	case "cbz":
		*f = OutputFormatCbz
	case "epub":
		*f = OutputFormatEpub
	case "kepub":
		*f = OutputFormatKepub
	default:
		return fmt.Errorf("unknown format: %s", val)
	}
	return nil
}

type PackagingProgressFunc func(completed float64)
