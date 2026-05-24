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
	"github.com/teerapap/mangafmt/internal/util"
)

func SaveAsRaw(pages []Page, outDir string, logger log.Logger) error {
	logger = logger.Indent("> Package > RAW ")
	logger.Info("Start packaging", "dir", outDir)

	err := os.MkdirAll(outDir, 0750)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}

	pageCount := len(pages)
	for i, page := range pages {
		logger.Infof("Packaging page....(%d/%d)", i+1, pageCount)

		filenameFmt := fmt.Sprintf("%%0%dd-%%s", util.DigitCount(pageCount))
		outFile := filepath.Join(outDir, fmt.Sprintf(filenameFmt, i+1, filepath.Base(page.Filepath)))

		err := os.Rename(page.Filepath, outFile)
		if err != nil {
			return fmt.Errorf("moving page file from %s to %s: %w", page.Filepath, outFile, err)
		}
	}
	logger.Info("Done packaging")
	return nil
}
