package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"gopkg.in/gographics/imagick.v2/imagick"
	"rsc.io/pdf"
)

// Loggers
var vlog = log.New(io.Discard, "Verbose: ", 0) // verbose log
var olog = log.New(os.Stdout, "", 0)           // output log
var elog = log.New(os.Stderr, "Error: ", 0)    // error log

// Command-line Parsing
var help bool
var verbose bool
var workDir string
var density uint
var start int
var end int
var trimMax float64
var trimFuzz float64
var targetSize Size
var isRTL bool

func init() {
	flag.Usage = func() {
		helpUsage("")
	}
	flag.BoolVar(&help, "help", false, "show help")
	flag.BoolVar(&help, "h", false, "show help")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.BoolVar(&verbose, "v", false, "verbose output")
	flag.StringVar(&workDir, "work-dir", "", "work directory path. Empty string means using system temp path")
	flag.UintVar(&density, "density", 300, "output density (DPI)")
	flag.IntVar(&start, "start", 1, "page start. (non-negative means first page)")
	flag.IntVar(&end, "end", -1, "page end. (non-negative means last page)")
	flag.BoolVar(&isRTL, "rtl", false, "right-to-left read direction (ex. Japanese manga)")
	flag.BoolVar(&isRTL, "right-to-left", false, "right-to-left read direction (ex. Japanese manga)")
	flag.Float64Var(&trimMax, "trim-max", 0.15, "maximum trim percentage (0.0-1.0)")
	flag.Float64Var(&trimFuzz, "trim-fuzz", 1.0, "trim fuzz")
	flag.UintVar(&targetSize.width, "width", 1263, "output screen width (pixel)")
	flag.UintVar(&targetSize.height, "height", 1680, "output screen heigt (pixel)")
}

func helpUsage(msg string) {
	if msg != "" {
		elog.Printf("Require input pdf file")
	}
	fmt.Fprintf(flag.CommandLine.Output(), "%s [options] <input_pdf_file>\n", os.Args[0])
	flag.PrintDefaults()
	if msg != "" {
		os.Exit(1)
	}
}

// Helper functions
func check(err error, doing string) {
	if err != nil {
		elog.Printf("while %s - %s\n", doing, err)
		panic(err)
	}
}

func handleExit() {
	if !verbose {
		if r := recover(); r != nil {
			// exit gracefully if not verbose
			os.Exit(1)
		}
	}
}

func createWorkDir(path string, clean bool) (string, bool, error) {
	if path == "" {
		// create temp directory
		tmpDir, err := os.MkdirTemp("", "mangafmt-")
		if err != nil {
			return "", true, fmt.Errorf("creating temp directory: %w", err)
		}
		return tmpDir, true, nil
	} else {
		if clean {
			// clean existing directory
			err := os.RemoveAll(path)
			if err != nil {
				return path, false, fmt.Errorf("removing existing work directory: %w", err)
			}
		}
		err := os.MkdirAll(path, 0750)
		if err != nil {
			return path, false, err
		}
		return path, false, nil
	}
}

func main() {
	defer handleExit()

	// Parse command-line
	flag.Parse()
	inputFile := flag.Arg(0)
	if verbose {
		vlog.SetOutput(os.Stdout)
	}

	if help {
		flag.Usage()
		os.Exit(0)
	} else if len(inputFile) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	start = max(start, 1)
	trimMax = max(min(trimMax, 1.0), 0.0)

	// Get number of pages
	totalPages := getNumberOfPages(inputFile)
	olog.Printf("Total Number of Pages: %d\n", totalPages)
	if end <= 0 {
		end = totalPages
	}
	if start > totalPages {
		elog.Panic("`--start` cannot exceeds total number of pages")
	} else if start > end {
		elog.Panic("`--start` cannot be larger than `--end`")
	}

	// Create work dir
	workDir, isTmp, err := createWorkDir(workDir, true)
	check(err, "creating work directory")
	vlog.Println("Work directory:", workDir)
	if isTmp {
		defer os.RemoveAll(workDir)
	}

	// Initialize Imagemagick
	vlog.Println("Initializing Imagemagick")
	imagick.Initialize()
	defer func() {
		vlog.Println("Terminating Imagemagick")
		imagick.Terminate()
		vlog.Println("Terminated Imagemagick")
	}()
	vlog.Println("Initialized Imagemagick")

	// For loop each page
	var itr *imagick.MagickWand
	outPage := 1
	if start != 1 && end != totalPages {
		olog.Printf("Select page(s) from %d to %d to process. Total %d pages.\n", start, end, end-start+1)
	}
	for page := start; page <= end; {
		olog.Printf("Processing page....(%d/%d)\n", page, end)
		check(process(&itr, inputFile, &page, end, &outPage), fmt.Sprintf("processing page %d", page))
		vlog.Printf("next input page = %d, next output page = %d\n", page, outPage)
	}
	olog.Printf("Total Input %d page(s). Total Output %d pages(s).", end-start+1, outPage+1)

	// TODO: Packaging

}

func getNumberOfPages(filename string) int {
	f, err := os.Open(filename)
	defer f.Close()
	check(err, "opening input pdf file")
	fi, err := f.Stat()
	check(err, "checking input pdf file size")
	r, err := pdf.NewReader(f, fi.Size())
	check(err, "reading input pdf file")
	return r.NumPage()
}
