//
// navigation.go
// Copyright (C) 2026 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package volume

import (
	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/volume/format"
)

// remapToc moves the table of content of the input file onto the output pages.
// outPageOf is the output page number(1-based) each input page lands on. An
// entry pointing to a page which is not in the output is removed from the
// table.
func remapToc(entries []format.EpubTocEntry, outPageOf map[int]int, logger log.Logger) []format.EpubTocEntry {
	if len(entries) == 0 {
		return nil
	}

	logger.Info("Remapping table of contents onto the output pages")
	toc := remapTocEntries(entries, outPageOf)
	logger.Debug("Done remapping table of contents -", "entries", len(toc), "input_entries", len(entries))
	return toc
}

func remapTocEntries(entries []format.EpubTocEntry, outPageOf map[int]int) []format.EpubTocEntry {
	remapped := make([]format.EpubTocEntry, 0, len(entries))
	for _, entry := range entries {
		mapped := format.EpubTocEntry{
			Label:    entry.Label,
			Children: remapTocEntries(entry.Children, outPageOf),
		}

		// two pages connected into one double-page spread land on the same
		// output page
		if outPageNo, found := outPageOf[entry.PageNo]; found {
			mapped.PageNo = outPageNo
		} else if len(mapped.Children) == 0 {
			// the page of the entry is not in the output
			continue
		} else {
			// keep the children by pointing the entry to its first child
			mapped.PageNo = mapped.Children[0].PageNo
		}

		remapped = append(remapped, mapped)
	}
	return remapped
}

// remapLandmarks moves the landmarks of the input file onto the output pages.
// outPageOf is the output page number(1-based) each input page lands on. A
// landmark pointing to a page which is not in the output is removed.
func remapLandmarks(entries []format.EpubLandmark, outPageOf map[int]int, logger log.Logger) []format.EpubLandmark {
	if len(entries) == 0 {
		return nil
	}

	logger.Info("Remapping landmarks onto the output pages")
	landmarks := make([]format.EpubLandmark, 0, len(entries))
	for _, entry := range entries {
		// two pages connected into one double-page spread land on the same
		// output page
		outPageNo, found := outPageOf[entry.PageNo]
		if !found {
			// the page of the landmark is not in the output
			continue
		}
		entry.PageNo = outPageNo
		landmarks = append(landmarks, entry)
	}
	logger.Debug("Done remapping landmarks -", "entries", len(landmarks), "input_entries", len(entries))
	return landmarks
}
