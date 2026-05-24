package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	clog "charm.land/log/v2"

	"github.com/teerapap/mangafmt/internal/book"
	"github.com/teerapap/mangafmt/internal/book/format"
	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/spread"
	"github.com/teerapap/mangafmt/internal/util"
)

// Command-line Parsing
var help bool
var verbose bool
var version bool
var workDir string
var pageRangeStr string
var pageRange = book.NewPageRange()
var bookTitle string
var bookAuthor string
var bookConfig book.BookConfig
var fuzzP float64
var trimConfig book.TrimConfig
var spreadConfig book.SpreadConfig
var resize bool
var targetSize book.Size
var grayscaleStr string
var grayConfig book.GrayscaleConfig
var outputFile string
var outputFormat format.OutputFormat
var convertOnly bool

func init() {
	flag.Usage = func() {
		helpUsage("")
	}
	flag.BoolVar(&help, "help", false, "Show help")
	flag.BoolVar(&help, "h", false, "Show help")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&verbose, "v", false, "Verbose output")
	flag.BoolVar(&version, "version", false, "Show version")
	flag.StringVar(&workDir, "work-dir", "", "Work directory path. Unspecified or blank means using system temp path")
	flag.StringVar(&pageRangeStr, "pages", "1-", "Page range (Ex. '4-10, 15, 39-'). Default is all pages. Open right range means to the end.")
	flag.StringVar(&bookTitle, "title", "", "Book title. This affects epub/kepub output. Unspecified or blank means using filename without extension")
	flag.StringVar(&bookAuthor, "author", "", "Book author. This affects epub/kepub output. Unspecified or blank means 'Anonymous'")
	flag.Float64Var(&bookConfig.Density, "density", 300.0, "Output density (DPI)")
	flag.BoolVar(&bookConfig.IsRTL, "rtl", false, "Right-to-left read direction (ex. Japanese manga)")
	flag.BoolVar(&bookConfig.IsRTL, "right-to-left", false, "Right-to-left read direction (ex. Japanese manga)")
	flag.Float64Var(&fuzzP, "fuzz", 0.1, "Color fuzz (percentage)[0.0-1.0]")
	flag.BoolVar(&trimConfig.Enabled, "trim", true, "Enable trim edge")
	flag.Float64Var(&trimConfig.MinSizeP, "trim-min-size", 0.85, "Minimum size after trimmed (percentage)[0.0-1.0]")
	flag.IntVar(&trimConfig.Margin, "trim-margin", 10, "Safety trim margin (pixel)")
	flag.BoolVar(&spreadConfig.Enabled, "spread", true, "Enable double-page spread detection and connection")
	flag.BoolVar(&spreadConfig.KeepOrientation, "spread-keep-orientation", false, "Keep the page original orientation. Do not rotate to maximize screen area")
	flag.BoolVar(&spreadConfig.KeepOriginal, "spread-keep-original", false, "Keep the original left and right page")
	sd := spread.NewSpreadDetector()
	flag.IntVar(&spreadConfig.EdgeWidth, "spread-edge", sd.EdgeStripWidth, "Edge width for double-page spread detection (pixel)")
	flag.Float64Var(&spreadConfig.Confidence, "spread-confidence", sd.SpreadThreshold, "Confidence threshold for double-page spread detection. The higher the value, the stricter the criteria become. (percentage)[0.0-1.0]")
	flag.BoolVar(&resize, "resize", true, "Resize to aspect fit in output screen size")
	flag.UintVar(&targetSize.Width, "width", 1264, "Output screen width (pixel)")
	flag.UintVar(&targetSize.Height, "height", 1680, "Output screen heigt (pixel)")
	flag.StringVar(&grayscaleStr, "grayscale", "2-", "Page range (Ex. '4-10, 15, 39-') to convert to grayscale. Default is all pages except the first page(cover). 'false' means no grayscale conversion")
	flag.UintVar(&grayConfig.ColorDepth, "grayscale-depth", 4, "Grayscale color depth in number of bits. Possible values are 1, 2, 4, 8, 16 bits. No upscale if source image is in lower depth.")
	flag.BoolVar(&convertOnly, "convert-only", false, "Convert from input to output format only without any modification to the pages at all")
	flag.Var(&outputFormat, "format", "Output file format. The supported formats\n\t- raw (default)\n\t- cbz\n\t- epub\n\t- kepub")
	flag.StringVar(&outputFile, "output", "", "Output file. Unspecified or blank means using the same file name as input file")
}

func helpUsage(msg string) {
	if msg != "" {
		clog.Error(msg)
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
			clog.Errorf("%s", r)
			os.Exit(1)
		}
	}
}

func main() {
	defer handleExit()

	// Parse command-line
	flag.Parse()
	inputFile := flag.Arg(0)
	if verbose {
		clog.SetLevel(clog.DebugLevel)
	} else {
		clog.SetLevel(clog.InfoLevel)
	}
	clogger := clog.NewWithOptions(os.Stdout, clog.Options{
		Level:           clog.GetLevel(),
		ReportTimestamp: true,
	})
	logger := log.Wrap(clogger)

	logger.Debug("mangafmt", "ver", util.AppVersion, "args", os.Args)

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
	logger.Debug("Input", "file", inputFile)
	outputFile = strings.TrimSpace(outputFile)
	if outputFile == "" {
		outputFile = util.ReplaceExt(inputFile, outputFormat.Ext())
	} else {
		outputFile = util.Must1(util.IsWritableFile(outputFile))("checking output file path")
	}
	logger.Debug("Output", "file", outputFile)

	trimConfig.MinSizeP = max(min(trimConfig.MinSizeP, 1.0), 0.0)
	fuzzP = max(min(fuzzP, 1.0), 0.0)
	util.Must(book.IsSupportedColorDepth(grayConfig.ColorDepth))("checking grayscale color depth")

	// Create work dir
	util.Must1(util.CreateWorkDir(&workDir, true))("creating work directory")
	defer os.RemoveAll(workDir)
	logger.Debug("Work directory", "path", workDir)

	logger = logger.Indent("> Book ")

	// Load input book file
	theBook := util.Must1(book.NewBook(inputFile, bookConfig, logger))("loading book")
	bookTitle = strings.TrimSpace(bookTitle)
	if bookTitle != "" {
		theBook.Title = bookTitle
	}
	bookAuthor = strings.TrimSpace(bookAuthor)
	if bookAuthor != "" {
		theBook.Author = bookAuthor
	}
	logger.Infof("Total Number of Pages: %d", theBook.PageCount)

	// Parse page range arguments
	util.Must(pageRange.Parse(pageRangeStr, theBook.PageCount))(fmt.Sprintf("parsing page range(%s)", pageRangeStr))
	if strings.ToLower(grayscaleStr) != "false" {
		grayConfig.PageRange = book.NewPageRange()
		util.Must(grayConfig.PageRange.Parse(grayscaleStr, theBook.PageCount))(fmt.Sprintf("parsing grayscale page range(%s)", grayscaleStr))
	}

	if convertOnly {
		// disable all modification
		trimConfig.Enabled = false
		spreadConfig.Enabled = false
		grayConfig.PageRange = nil
		resize = false
	}

	// For loop each page
	partials := pageRange.PageCount() != theBook.PageCount
	if partials {
		logger.Infof("Start formatting page(s) in range %s. Total %d page(s).", pageRange, pageRange.PageCount())
	} else {
		logger.Infof("Start formatting. Total %d page(s).", pageRange.PageCount())
	}

	outPages := make([]format.Page, 0, theBook.PageCount)
	for page, i := 1, 1; page <= theBook.PageCount; {
		if !pageRange.Contains(page) {
			page += 1
			continue
		}
		if partials {
			logger.Infof("Formatting page %d....(%d/%d)", page, i, pageRange.PageCount())
		} else {
			logger.Infof("Formatting page....(%d/%d)", page, theBook.PageCount)
		}

		pageLogger := logger.Indent(fmt.Sprintf("> Page[%d] ", page))
		outPage, formatted := util.Must2(formatEachPage(theBook, pageRange, page, pageLogger))(fmt.Sprintf("formatting page %d", page))
		outPages = append(outPages, outPage...)
		page += formatted
		i += formatted
		logger.Debug("Done formatting page -", "next_input_page", page, "next_output_page", len(outPages))
	}
	logger.Info("Done formatting book -", "total_input_pages", pageRange.PageCount(), "total_output_pages", len(outPages))

	// Packaging
	switch outputFormat {
	case format.RAW:
		util.Must(format.SaveAsRaw(outPages, outputFile, logger))("saving in raw format")
	case format.CBZ:
		util.Must(format.SaveAsCBZ(outPages, outputFile, logger))("saving in cbz format")
	case format.EPUB:
		util.Must(format.SaveAsEPUB(theBook, outPages, outputFile, logger))("saving in epub format")
	case format.KEPUB:
		util.Must(format.SaveAsKEPUB(theBook, outPages, outputFile, logger))("saving in kepub format")
	}
	logger.Infof("Total Input %d page(s). Total Output %d pages(s).", pageRange.PageCount(), len(outPages))
}

func formatEachPage(theBook *book.Book, pr *book.PageRange, pageNo int, logger log.Logger) ([]format.Page, int, error) {
	formatted := 0
	current, err := theBook.LoadPage(pageNo, logger)
	if err != nil {
		return nil, 0, fmt.Errorf("loading page %d: %w", pageNo, err)
	}
	defer current.Destroy()

	formatted += 1

	connected := false
	var next *book.Page = nil

	outPages := make([]format.Page, 0, 3)

	// Look ahead next page
	if pr.Contains(pageNo+1) && spreadConfig.Enabled { // has next page
		// Read next page
		next, err = theBook.LoadPage(pageNo+1, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("loading next page %d: %w", pageNo+1, err)
		}
		defer next.Destroy()

		// Check if the next page can merge with current page
		left, right := current.LeftRight(next)
		connected, err = left.IsDoublePageSpread(right, spreadConfig, logger)
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
			outPage, err := formatSinglePage(spread, spreadConfig.KeepOrientation, logger)
			if err != nil {
				return nil, 0, fmt.Errorf("formatting spread page: %w", err)
			}
			outPages = append(outPages, *outPage)
		}
	}

	if !connected || spreadConfig.KeepOriginal {
		// format current page
		outPage, err := formatSinglePage(current, false, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("formatting current page: %w", err)
		}
		outPages = append(outPages, *outPage)
	}

	if connected && spreadConfig.KeepOriginal {
		// format next page
		outPage, err := formatSinglePage(next, false, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("formatting original next page: %w", err)
		}
		outPages = append(outPages, *outPage)
	}

	return outPages, formatted, nil
}

func formatSinglePage(current *book.Page, keepOrientation bool, logger log.Logger) (*format.Page, error) {

	// Trim image with fuzz
	if err := current.Trim(trimConfig, fuzzP, logger); err != nil {
		return nil, fmt.Errorf("trimming page: %w", err)
	}

	// Resize page to aspect fit screen
	if resize {
		if err := current.ResizeToFit(targetSize, keepOrientation, logger); err != nil {
			return nil, fmt.Errorf("resizing page to fit to screen: %w", err)
		}
	}

	// Convert to grayscale
	if err := current.ConvertToGrayscale(grayConfig, logger); err != nil {
		return nil, fmt.Errorf("converting page to grayscale: %w", err)
	}

	// Write to filesystem
	outFile, mediaType, err := current.WriteFile(workDir, logger)
	if err != nil {
		return nil, fmt.Errorf("writing to filesystem: %w", err)
	}

	outPage := format.Page{
		Id:        current.Filename(""),
		Filepath:  outFile,
		MediaType: mediaType,
		Size:      current.Size(),
	}
	return &outPage, nil
}
