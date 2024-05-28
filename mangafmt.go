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
var workDir string
var start int
var end int
var bookConfig book.BookConfig
var fuzzP float64
var trimConfig book.TrimConfig
var connConfig book.ConnectConfig
var targetSize book.Size
var grayscale bool
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
	flag.StringVar(&workDir, "work-dir", "", "work directory path. Empty string means using system temp path")
	flag.IntVar(&start, "start", 1, "page start. (non-negative means first page)")
	flag.IntVar(&end, "end", -1, "page end. (non-negative means last page)")
	flag.Float64Var(&bookConfig.Density, "density", 300.0, "output density (DPI)")
	flag.StringVar(&bookConfig.BgColor, "background", "white", "background color")
	flag.BoolVar(&bookConfig.IsRTL, "rtl", false, "right-to-left read direction (ex. Japanese manga)")
	flag.BoolVar(&bookConfig.IsRTL, "right-to-left", false, "right-to-left read direction (ex. Japanese manga)")
	flag.Float64Var(&fuzzP, "fuzz", 0.1, "color fuzz (percentage)[0.0-1.0]")
	flag.BoolVar(&trimConfig.Enabled, "trim", true, "enable trim edge")
	flag.Float64Var(&trimConfig.MinSizeP, "trim-min-size", 0.85, "minimum size after trimmed (percentage)[0.0-1.0]")
	flag.IntVar(&trimConfig.Margin, "trim-margin", 10, "safety trim margin (pixel)")
	flag.BoolVar(&connConfig.Enabled, "connect", true, "enable two-page connection")
	flag.UintVar(&connConfig.EdgeWidth, "connect-edge", 2, "edge width for two-page connection check (pixel)")
	flag.UintVar(&connConfig.EdgeMargin, "connect-margin", 2, "safety margin before edge width (pixel)")
	flag.Float64Var(&connConfig.BgDistort, "connect-bg-distortion", 0.4, "a page is considered a single page if the distortion between its edge and background color are less within this threshold (percentage)[0.0-1.0]")
	flag.Float64Var(&connConfig.LrDistort, "connect-lr-distortion", 0.4, "two pages are considered connected if the distortion between their edges are less within this threshold (percentage)[0.0-1.0]")
	flag.UintVar(&targetSize.Width, "width", 1264, "output screen width (pixel)")
	flag.UintVar(&targetSize.Height, "height", 1680, "output screen heigt (pixel)")
	flag.BoolVar(&grayscale, "grayscale", true, "convert to grayscale images")
	flag.Var(&outputFormat, "format", "output file format. The supported formats\n\t- raw (default)\n\t- cbz")
	flag.StringVar(&outputFile, "output", "", "output file. Empty string means using the same file name as input file")
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

// Helper functions

func handleExit() {
	if !verbose {
		if r := recover(); r != nil {
			// exit gracefully if not verbose
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
	} else if len(inputFile) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	inputFile = util.Must1(util.IsReadableFile(inputFile))("input file path")
	log.Verbosef("Input: %s", inputFile)
	outputFile = strings.TrimSpace(outputFile)
	if len(outputFile) == 0 {
		outputFile = util.ReplaceExt(inputFile, outputFormat.Ext())
	} else {
		outputFile = util.Must1(util.IsWritableFile(outputFile))("output file path")
	}
	log.Verbosef("Output: %s", outputFile)
	start = max(start, 1)
	trimConfig.MinSizeP = max(min(trimConfig.MinSizeP, 1.0), 0.0)
	fuzzP = max(min(fuzzP, 1.0), 0.0)

	// Load input book file
	theBook := util.Must1(book.NewBook(inputFile, bookConfig))("loading book")
	log.Printf("Total Number of Pages: %d", theBook.PageCount)
	if end <= 0 {
		end = theBook.PageCount
	}
	if start > theBook.PageCount {
		log.Panic("`--start` cannot exceeds total number of pages")
	} else if start > end {
		log.Panic("`--start` cannot be larger than `--end`")
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
	if start != 1 || end != theBook.PageCount {
		log.Printf("Start processing from %d to %d to process. Total %d pages.", start, end, end-start+1)
	} else {
		log.Printf("Start processing. Total %d pages.", end-start+1)
	}
	log.Indent()

	outPages := make([]format.Page, 0, theBook.PageCount)
	for page := start; page <= end; {
		log.Printf("Processing page....(%d/%d)", page, end)
		log.Indent()

		outPage, processed := util.Must2(processEachPage(theBook, page, end))(fmt.Sprintf("processing page %d", page))
		outPages = append(outPages, *outPage)
		page += processed
		log.Verbosef("next input page = %d, next output page = %d", page, len(outPages))

		log.Unindent()
		log.Print("")
	}
	log.Unindent()
	log.Printf("Done processing.")
	log.Printf("Total Input %d page(s). Total Output %d pages(s).", end-start+1, len(outPages))

	// Packaging
	switch outputFormat {
	case format.RAW:
		util.Must(format.SaveAsRaw(outPages, outputFile))("saving in raw format")
		return
	case format.CBZ:
		util.Must(format.SaveAsCBZ(outPages, outputFile))("saving in cbz format")
	}

}

func processEachPage(theBook *book.Book, pageNo int, end int) (*format.Page, int, error) {
	processed := 0
	current, err := theBook.LoadPage(pageNo)
	if err != nil {
		return nil, 0, fmt.Errorf("loading page %d: %w", pageNo, err)
	}
	defer current.Destroy()

	processed += 1

	// Look ahead next page
	if pageNo+1 <= end && connConfig.Enabled { // has next page
		// Read next page
		next, err := theBook.LoadPage(pageNo + 1)
		if err != nil {
			return nil, 0, fmt.Errorf("loading next page %d: %w", pageNo+1, err)
		}
		defer next.Destroy()

		// Check if the next page can merge with current page
		left, right := current.LeftRight(next)
		connected, err := left.CanConnect(right, connConfig)
		if err != nil {
			return nil, 0, fmt.Errorf("checking if two pages are connected: %w", err)
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
	if grayscale {
		if err := current.ConvertToGrayscale(); err != nil {
			return nil, 0, fmt.Errorf("converting page to grayscale: %w", err)
		}
	}

	// Write to filesystem
	outFile, err := current.WriteFile(workDir)
	if err != nil {
		return nil, 0, fmt.Errorf("writing to filesystem: %w", err)
	}

	outPage := format.Page{
		Filepath: outFile,
		Size:     current.Size(),
	}

	return &outPage, processed, nil
}
