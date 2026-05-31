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
	"path/filepath"

	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/util"
)

func SaveAsCBZ(volume Volume, outFile string, logger log.Logger) error {
	logger = logger.Indent("> Package > CBZ ")
	logger.Info("Start packaging", "file", outFile)

	zipFile, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	pageCount := len(volume.Pages)
	for i, page := range volume.Pages {
		logger.Infof("Packaging page....(%d/%d)", i+1, pageCount)

		filenameFmt := fmt.Sprintf("%%0%dd-%%s", util.DigitCount(pageCount))
		outFileName := fmt.Sprintf(filenameFmt, i+1, filepath.Base(page.Filepath))
		err := util.CopyFileToZip(w, outFileName, page.Filepath)
		if err != nil {
			return fmt.Errorf("copying page file to the output file: %w", err)
		}
	}
	logger.Info("Done packaging")
	return nil
}
