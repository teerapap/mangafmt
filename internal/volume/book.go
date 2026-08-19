//
// volume.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package volume

import (
	"fmt"
	"path/filepath"
	"strings"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/util"
	"github.com/teerapap/mangafmt/internal/volume/format"
)

type Volume struct {
	Filepath  string
	Info      Info
	PageCount int
	Config    Config
	lruCache  *lru.Cache[int, Page]
	source    InputSource
}

type Info struct {
	Title  string
	Author string
}

type Config struct {
	Density float64
	IsRTL   bool
	// IsRTLSet is true when the read direction is explicitly set by the user.
	// It takes precedence over the read direction in the input file.
	IsRTLSet bool
	// SkipUnreadablePage is true when a page which cannot be read is skipped
	// instead of stopping the formatting.
	SkipUnreadablePage bool
}

func NewVolume(path string, info Info, cfg Config, logger log.Logger) (*Volume, error) {

	source, err := openInputSource(path, cfg, logger)
	if err != nil {
		return nil, err
	}
	logger.Debug("Opened input file -", "format", source.Name(), "page_count", source.PageCount())

	meta := source.Metadata()
	if !cfg.IsRTLSet && meta.IsRTL != nil {
		// the input file knows its own read direction
		cfg.IsRTL = *meta.IsRTL
		logger.Info("Using read direction from the input file -", "rtl", cfg.IsRTL)
	}

	// the title and the author from the command-line flags take precedence
	// over the ones in the input file
	info.Title = strings.TrimSpace(info.Title)
	if info.Title == "" {
		info.Title = strings.TrimSpace(meta.Title)
	}
	if info.Title == "" {
		info.Title = util.NameWithoutExt(filepath.Base(path))
	}
	info.Author = strings.TrimSpace(info.Author)
	if info.Author == "" {
		info.Author = strings.TrimSpace(meta.Author)
	}

	lruCache, err := lru.New[int, Page](2)
	if err != nil {
		return nil, fmt.Errorf("creating page cache: %w", err)
	}

	return &Volume{
		Filepath:  path,
		Info:      info,
		PageCount: source.PageCount(),
		Config:    cfg,
		lruCache:  lruCache,
		source:    source,
	}, nil
}

// Metadata is the volume information found in the input file
func (v *Volume) Metadata() SourceMetadata {
	return v.source.Metadata()
}

// Skipped is the pages in the input file which are skipped so they are not in
// the output file
func (v *Volume) Skipped() SkippedPages {
	return v.source.Skipped()
}

func (v *Volume) LoadPage(pageNo int, workDir string, logger log.Logger) (*Page, error) {
	// load from cache first
	cachedPage, found := v.lruCache.Get(pageNo)
	if found {
		logger.Debug("Loading page from cache -", "page_no", pageNo)
		return &cachedPage, nil
	}

	// load page image from the input file
	logger.Info("Loading page -", "page_no", pageNo, "format", v.source.Name())
	img, err := v.source.LoadImage(pageNo, v.Config, workDir, logger)
	if err != nil {
		return nil, fmt.Errorf("loading page %d from %s file: %w", pageNo, v.source.Name(), err)
	}
	logger.Debug("Loaded page -", "page_no", pageNo, "size", img.Bounds())

	page := Page{
		img:    img,
		volume: v,
		PageNo: pageNo,
	}

	// save to cache
	v.lruCache.Add(pageNo, page)

	return &page, nil
}

type FormatConfig struct {
	Spread    SpreadConfig
	Trim      TrimConfig
	Resize    ResizeConfig
	Grayscale GrayscaleConfig
	WorkDir   string
}

type FormattingProgressFunc func(v *Volume, completed float64, lastPageNo int)

func (v *Volume) Format(pr PageRange, cfg FormatConfig, logger log.Logger, progress FormattingProgressFunc) (*format.Volume, error) {
	defer v.lruCache.Purge() // purge the whole cache after done

	// For loop each page
	partials := pr.PageCount() != v.PageCount
	if partials {
		logger.Infof("Start formatting page(s) in range %s. Total %d page(s).", pr, pr.PageCount())
	} else {
		logger.Infof("Start formatting. Total %d page(s).", pr.PageCount())
	}

	outPages := make([]format.Page, 0, v.PageCount)
	// the output page number(1-based) each input page lands on. two input pages
	// connected into one double-page spread land on the same output page.
	outPageOf := make(map[int]int, v.PageCount)
	for pageNo, i := 1, 1; pageNo <= v.PageCount; {
		if !pr.Contains(pageNo) {
			pageNo += 1
			continue
		}
		if partials {
			logger.Infof("Formatting page %d....(%d/%d)", pageNo, i, pr.PageCount())
		} else {
			logger.Infof("Formatting page....(%d/%d)", pageNo, pr.PageCount())
		}

		pageLogger := logger.Indent(fmt.Sprintf("> Page[%d] ", pageNo))

		outPage, formatted, err := v.formatPage(pageNo, pr, cfg, pageLogger)
		if err != nil {
			return nil, fmt.Errorf("formatting page %d: %w", pageNo, err)
		}
		for p := pageNo; p < pageNo+formatted; p++ {
			outPageOf[p] = len(outPages) + 1
		}
		outPages = append(outPages, outPage...)
		pageNo += formatted
		i += formatted
		progress(v, float64(i-1)/float64(pr.PageCount()), pageNo-1)
		logger.Debug("Done formatting page -", "next_input_page", pageNo, "next_output_page", len(outPages))
	}
	toc := v.remapToc(outPageOf, logger)

	skipped := v.Skipped()
	doneArgs := []any{"total_input_pages", pr.PageCount(), "total_output_pages", len(outPages)}
	if noImage := skipped.Count(SkipNoImage); noImage > 0 {
		doneArgs = append(doneArgs, "skipped_no_image_pages", noImage)
	}
	if encrypted := skipped.Count(SkipEncrypted); encrypted > 0 {
		doneArgs = append(doneArgs, "skipped_encrypted_pages", encrypted)
	}
	logger.Info("Done formatting volume -", doneArgs...)

	return &format.Volume{
		Title:      v.Info.Title,
		Author:     v.Info.Author,
		Language:   v.Metadata().Language,
		Identifier: v.Metadata().Identifier,
		IsRTL:      v.Config.IsRTL,
		Epub: format.EpubMetadata{
			TableOfContents: toc,
			Namespaces:      v.Metadata().Epub.Namespaces,
			Prefix:          v.Metadata().Epub.Prefix,
			Entries:         v.Metadata().Epub.Entries,
		},
		Pages: outPages,
	}, nil
}

// remapToc moves the table of content of the input file onto the output pages.
// An entry pointing to a page which is not in the output is removed from the
// table.
func (v *Volume) remapToc(outPageOf map[int]int, logger log.Logger) []format.EpubTocEntry {
	if len(v.Metadata().Epub.TableOfContents) == 0 {
		return nil
	}

	logger.Info("Remapping table of contents onto the output pages")
	toc := remapTocEntries(v.Metadata().Epub.TableOfContents, outPageOf)
	logger.Debug("Done remapping table of contents -", "entries", len(toc), "input_entries", len(v.Metadata().Epub.TableOfContents))
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

func (v *Volume) formatPage(pageNo int, pr PageRange, cfg FormatConfig, logger log.Logger) ([]format.Page, int, error) {
	formatted := 0
	current, err := v.LoadPage(pageNo, cfg.WorkDir, logger)
	if err != nil {
		return nil, 0, fmt.Errorf("loading page %d: %w", pageNo, err)
	}
	defer current.Destroy()

	formatted += 1

	connected := false
	var next *Page = nil

	outPages := make([]format.Page, 0, 3)

	// Look ahead next page
	if !cfg.Spread.Enabled {
		logger.Indent("> Spread    ").Debug("Disabled")
	} else if pr.Contains(pageNo + 1) { // has next page
		// Read next page
		next, err = v.LoadPage(pageNo+1, cfg.WorkDir, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("loading next page %d: %w", pageNo+1, err)
		}
		defer next.Destroy()

		// Check if the next page can merge with current page
		left, right := current.LeftRight(next)
		connected, err = left.IsDoublePageSpread(right, cfg.Spread, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("checking if two pages are double-page spread: %w", err)
		}
		if connected {
			// connect two pages
			spread, err := left.Connect(right, logger)
			if err != nil {
				return nil, 0, fmt.Errorf("connecting two pages: %w", err)
			}
			defer spread.Destroy()
			formatted += 1

			// format the spread page
			outPage, err := spread.Format(cfg, cfg.Spread.KeepOrientation, logger)
			if err != nil {
				return nil, 0, fmt.Errorf("formatting spread page: %w", err)
			}
			outPages = append(outPages, *outPage)
		}
	}

	if !connected || cfg.Spread.KeepOriginal {
		// format current page
		outPage, err := current.Format(cfg, false, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("formatting current page: %w", err)
		}
		outPages = append(outPages, *outPage)
	}

	if connected && cfg.Spread.KeepOriginal {
		// format next page
		outPage, err := next.Format(cfg, false, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("formatting original next page: %w", err)
		}
		outPages = append(outPages, *outPage)
	}

	return outPages, formatted, nil
}
