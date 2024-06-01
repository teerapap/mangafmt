package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/teerapap/mangafmt/internal/book"
	"github.com/teerapap/mangafmt/internal/book/format"
	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/util"
	"gopkg.in/gographics/imagick.v2/imagick"
)

// Command-line Parsing
var help bool
var verbose bool
var version bool
var workDir string
var pageRangeStr string
var pageRange = book.NewPageRange()
var bookTitle string
var bookConfig book.BookConfig
var fuzzP float64
var trimConfig book.TrimConfig
var spreadConfig book.SpreadConfig
var targetSize book.Size
var grayscaleStr string
var grayscalePR = book.NewPageRange()
var outputFile string
var outputFormat format.OutputFormat

func init() {
	flag.Usage = func() {
		helpUsage("")
	}
	flag.BoolVar(&help, "help", false, "show help")
	flag.BoolVar(&help, "h", false, "show help")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.BoolVar(&version, "version", false, "show version")
	flag.StringVar(&workDir, "work-dir", "", "work directory path. Unspecified or blank means using system temp path")
	flag.StringVar(&pageRangeStr, "pages", "1-", "page range (Ex. '4-10, 15, 39-'). Default is all pages. Open right range means to the end.")
	flag.StringVar(&bookTitle, "title", "", "Book title. This affects epub/kepub output. Unspecified or blank means using filename without extension")
	flag.Float64Var(&bookConfig.Density, "density", 300.0, "output density (DPI)")
	flag.StringVar(&bookConfig.BgColor, "background", "white", "background color")
	flag.BoolVar(&bookConfig.IsRTL, "rtl", false, "right-to-left read direction (ex. Japanese manga)")
	flag.BoolVar(&bookConfig.IsRTL, "right-to-left", false, "right-to-left read direction (ex. Japanese manga)")
	flag.Float64Var(&fuzzP, "fuzz", 0.1, "color fuzz (percentage)[0.0-1.0]")
	flag.BoolVar(&trimConfig.Enabled, "trim", true, "enable trim edge")
	flag.Float64Var(&trimConfig.MinSizeP, "trim-min-size", 0.85, "minimum size after trimmed (percentage)[0.0-1.0]")
	flag.IntVar(&trimConfig.Margin, "trim-margin", 10, "safety trim margin (pixel)")
	flag.BoolVar(&spreadConfig.Enabled, "spread", true, "enable double-page spread detection and connection")
	flag.UintVar(&spreadConfig.EdgeWidth, "spread-edge", 2, "edge width for double-page spread detection (pixel)")
	flag.UintVar(&spreadConfig.EdgeMargin, "spread-margin", 2, "safety margin before edge width (pixel)")
	flag.Float64Var(&spreadConfig.BgDistort, "spread-bg-distortion", 0.4, "a page is considered a single page if the distortion between its edge and background color are less than this threshold (percentage)[0.0-1.0]")
	flag.Float64Var(&spreadConfig.LrDistort, "spread-lr-distortion", 0.4, "two pages are considered double-page spread if the distortion between their edges are less than this threshold (percentage)[0.0-1.0]")
	flag.UintVar(&targetSize.Width, "width", 1264, "output screen width (pixel)")
	flag.UintVar(&targetSize.Height, "height", 1680, "output screen heigt (pixel)")
	flag.StringVar(&grayscaleStr, "grayscale", "2-", "page range (Ex. '4-10, 15, 39-') to convert to grayscale. Default is all pages except the first page(cover). 'false' means no grayscale conversion")
	flag.Var(&outputFormat, "format", "output file format. The supported formats\n\t- raw (default)\n\t- cbz\n\t- epub\n\t- kepub")
	flag.StringVar(&outputFile, "output", "", "output file. Unspecified or blank means using the same file name as input file")
}

func helpUsage(msg string) {
	if msg != "" {
		log.Error(msg)
	}
	fmt.Fprintf(flag.CommandLine.Output(), "%s [options] <input_pdf_file>\n", os.Args[0])
	flag.PrintDefaults()
	if msg != "" {
		os.Exit(1)
	}
}

func showVersion() {
	fmt.Printf("mangafmt-%s\n", util.AppVersion)
}

// Helper functions

func handleExit() {
	if !verbose {
		if r := recover(); r != nil {
			// exit gracefully if not verbose
			log.Errorf("%s", r)
			os.Exit(1)
		}
	}
}

func main() {
	defer handleExit()

	// Parse command-line
	flag.Parse()
	inputFile := flag.Arg(0)
	log.SetVerbose(verbose)

	if help {
		flag.Usage()
		os.Exit(0)
	} else if version {
		showVersion()
		os.Exit(0)
	} else if inputFile == "" {
		flag.Usage()
		os.Exit(1)
	}
	inputFile = util.Must1(util.IsReadableFile(inputFile))("checking input file path")
	log.Verbosef("Input: %s", inputFile)
	outputFile = strings.TrimSpace(outputFile)
	if outputFile == "" {
		outputFile = util.ReplaceExt(inputFile, outputFormat.Ext())
	} else {
		outputFile = util.Must1(util.IsWritableFile(outputFile))("checking output file path")
	}
	log.Verbosef("Output: %s", outputFile)
	trimConfig.MinSizeP = max(min(trimConfig.MinSizeP, 1.0), 0.0)
	fuzzP = max(min(fuzzP, 1.0), 0.0)

	// Load input book file
	theBook := util.Must1(book.NewBook(inputFile, bookConfig))("loading book")
	bookTitle = strings.TrimSpace(bookTitle)
	if bookTitle != "" {
		theBook.Title = bookTitle
	}
	log.Printf("Total Number of Pages: %d", theBook.PageCount)

	// Parse page range arguments
	util.Must(pageRange.Parse(pageRangeStr, theBook.PageCount))(fmt.Sprintf("parsing page range(%s)", pageRangeStr))
	if strings.ToLower(grayscaleStr) != "false" {
		util.Must(grayscalePR.Parse(grayscaleStr, theBook.PageCount))(fmt.Sprintf("parsing grayscale page range(%s)", grayscaleStr))
	}

	// Create work dir
	util.Must1(util.CreateWorkDir(&workDir, true))("creating work directory")
	defer os.RemoveAll(workDir)
	log.Verbosef("Work directory: %s", workDir)

	// Initialize Imagemagick
	log.Verbose("Initializing Imagemagick")
	imagick.Initialize()
	defer func() {
		log.Verbose("Terminating Imagemagick")
		imagick.Terminate()
		log.Verbose("Terminated Imagemagick")
	}()
	log.Verbose("Initialized Imagemagick")

	// For loop each page
	partials := pageRange.PageCount() != theBook.PageCount
	if partials {
		log.Printf("Start processing page(s) in range %s. Total %d page(s).", pageRange, pageRange.PageCount())
	} else {
		log.Printf("Start processing. Total %d page(s).", pageRange.PageCount())
	}
	log.Indent()

	outPages := make([]format.Page, 0, theBook.PageCount)
	for page, i := 1, 1; page <= theBook.PageCount; {
		if !pageRange.Contains(page) {
			page += 1
			continue
		}
		if partials {
			log.Printf("Processing page %d....(%d/%d)", page, i, pageRange.PageCount())
		} else {
			log.Printf("Processing page....(%d/%d)", page, theBook.PageCount)
		}
		log.Indent()

		outPage, processed := util.Must2(processEachPage(theBook, pageRange, page))(fmt.Sprintf("processing page %d", page))
		outPages = append(outPages, *outPage)
		page += processed
		i += processed
		log.Verbosef("next input page = %d, next output page = %d", page, len(outPages))

		log.Unindent()
	}
	log.Unindent()
	log.Printf("Done processing.")
	log.Printf("Total Input %d page(s). Total Output %d pages(s).", pageRange.PageCount(), len(outPages))

	// Packaging
	switch outputFormat {
	case format.RAW:
		util.Must(format.SaveAsRaw(outPages, outputFile))("saving in raw format")
	case format.CBZ:
		util.Must(format.SaveAsCBZ(outPages, outputFile))("saving in cbz format")
	case format.EPUB:
		util.Must(format.SaveAsEPUB(theBook, outPages, outputFile))("saving in epub format")
	case format.KEPUB:
		util.Must(format.SaveAsKEPUB(theBook, outPages, outputFile))("saving in kepub format")
	}
	log.Printf("Total Input %d page(s). Total Output %d pages(s).", pageRange.PageCount(), len(outPages))
}

func processEachPage(theBook *book.Book, pr *book.PageRange, pageNo int) (*format.Page, int, error) {
	processed := 0
	current, err := theBook.LoadPage(pageNo)
	if err != nil {
		return nil, 0, fmt.Errorf("loading page %d: %w", pageNo, err)
	}
	defer current.Destroy()

	processed += 1

	// Look ahead next page
	if pr.Contains(pageNo+1) && spreadConfig.Enabled { // has next page
		// Read next page
		next, err := theBook.LoadPage(pageNo + 1)
		if err != nil {
			return nil, 0, fmt.Errorf("loading next page %d: %w", pageNo+1, err)
		}
		defer next.Destroy()

		// Check if the next page can merge with current page
		left, right := current.LeftRight(next)
		connected, err := left.IsDoublePageSpread(right, spreadConfig)
		if err != nil {
			return nil, 0, fmt.Errorf("checking if two pages are double-page spread: %w", err)
		}
		if connected {
			// connect two pages
			if current, err = left.Connect(right); err != nil {
				return nil, 0, fmt.Errorf("connecting two pages: %w", err)
			}
			defer current.Destroy()
			processed += 1
		}
	}

	// Trim image with fuzz
	if err := current.Trim(trimConfig, fuzzP, workDir); err != nil {
		return nil, 0, fmt.Errorf("trimming page: %w", err)
	}

	// Resize page to aspect fit screen
	if err := current.ResizeToFit(targetSize); err != nil {
		return nil, 0, fmt.Errorf("resizing page to fit to screen: %w", err)
	}

	// Convert to grayscale
	if grayscalePR.Contains(current.PageNo) || (current.OtherPageNo > 0 && grayscalePR.Contains(current.OtherPageNo)) {
		if err := current.ConvertToGrayscale(); err != nil {
			return nil, 0, fmt.Errorf("converting page to grayscale: %w", err)
		}
	}

	// Write to filesystem
	outFile, mediaType, err := current.WriteFile(workDir)
	if err != nil {
		return nil, 0, fmt.Errorf("writing to filesystem: %w", err)
	}

	outPage := format.Page{
		Id:        current.Filename(""),
		Filepath:  outFile,
		MediaType: mediaType,
		Size:      current.Size(),
	}

	return &outPage, processed, nil
}
