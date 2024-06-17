//
// raw.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package format

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/teerapap/mangafmt/internal/log"
)

func SaveAsRaw(pages []Page, outDir string) error {
	defer log.SetIndentLevel(log.IndentLevel()) // reset indent level after return

	log.Printf("Start packaging in RAW format to %s", outDir)

	err := os.MkdirAll(outDir, 0750)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}

	pageCount := len(pages)
	log.Indent()
	for i, page := range pages {
		log.Printf("Packaging page....(%d/%d)", i+1, pageCount)
		log.Indent()

		outFile := filepath.Join(outDir, filepath.Base(page.Filepath))

		err := os.Rename(page.Filepath, outFile)
		if err != nil {
			return fmt.Errorf("moving page file from %s to %s: %w", page.Filepath, outFile, err)
		}

		log.Unindent()
	}
	log.Unindent()
	log.Printf("Done packaging.")
	return nil
}
