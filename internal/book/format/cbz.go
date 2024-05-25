//
// cbz.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package format

import (
	"archive/zip"
	"fmt"
	"io"
	"os"

	"github.com/teerapap/mangafmt/internal/log"
)

func SaveAsCBZ(pages []Page, outFile string) error {
	log.Printf("Start packaging in CBZ format to %s", outFile)

	zipFile, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	pageCount := len(pages)
	log.SetLogIndent(1)
	for i, page := range pages {
		log.Printf("Packaging page....(%d/%d)", i+1, pageCount)
		log.SetLogIndent(2)

		pageFile, err := os.Open(page.Filepath)
		if err != nil {
			return fmt.Errorf("opening page file: %w", err)
		}
		defer pageFile.Close()

		info, err := pageFile.Stat()
		if err != nil {
			return fmt.Errorf("getting page file info: %w", err)
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("converting page file info to file header: %w", err)
		}
		header.Method = zip.Deflate

		out, err := w.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("creating page file entry in the format: %w", err)
		}

		_, err = io.Copy(out, pageFile)
		if err != nil {
			return fmt.Errorf("adding page file to final file: %w", err)
		}

		log.SetLogIndent(1)
		log.Print("")

	}
	log.SetLogIndent(0)
	log.Printf("Done packaging.")
	return nil
}
