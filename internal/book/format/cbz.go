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
	"os"

	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/util"
)

func SaveAsCBZ(pages []Page, outFile string) error {
	defer log.SetIndentLevel(log.IndentLevel()) // reset indent level after return

	log.Printf("Start packaging in CBZ format to %s", outFile)

	zipFile, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	pageCount := len(pages)
	log.Indent()
	for i, page := range pages {
		log.Printf("Packaging page....(%d/%d)", i+1, pageCount)
		log.Indent()

		err := util.CopyFileToZip(w, "", page.Filepath)
		if err != nil {
			return fmt.Errorf("copying page file to the output file: %w", err)
		}

		log.Unindent()
	}
	log.Unindent()
	log.Printf("Done packaging.")
	return nil
}
