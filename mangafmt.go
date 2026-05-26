package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	clog "charm.land/log/v2"
	"golang.org/x/sync/errgroup"

	"github.com/teerapap/mangafmt/internal/book"
	"github.com/teerapap/mangafmt/internal/book/format"
	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/spread"
	"github.com/teerapap/mangafmt/internal/util"
)

func helpUsage() {
	fmt.Fprintf(flag.CommandLine.Output(), "%s [options] <input_pdf_file> [input_pdf_file2...]\n", os.Args[0])
	flag.PrintDefaults()
}

func showVersion() {
	fmt.Printf("mangafmt-%s\n", util.AppVersion)
}

// Helper functions

func handlePanic(logger log.Logger) {
	if r := recover(); r != nil {
		// exit gracefully if not verbose
		logger.Error("Panic", "err", r)
		os.Exit(2)
	}
}

func newLogger(w io.Writer) *clog.Logger {
	return clog.NewWithOptions(w, clog.Options{
		Level:           clog.GetLevel(),
		ReportTimestamp: true,
	})
}

func printLogHeader(logger log.Logger) {
	logger.Debug("mangafmt", "ver", util.AppVersion, "args", os.Args)
	cwd := util.Must1(os.Getwd())("getting current directory")
	logger.Debug("current directory", "dir", cwd)
}

func main() {

	// Command-line flags
	var help bool
	var verbose bool
	var version bool
	var selectedPr string
	var bookInfo book.Info
	var bookConfig book.Config
	var formatConfig book.FormatConfig
	var grayscalePr string
	var outputFile string
	var outputFormat format.OutputFormat
	var convertOnly bool
	var parallel int
	var logToFile bool

	flag.Usage = helpUsage
	flag.BoolVar(&help, "help", false, "Show help")
	flag.BoolVar(&help, "h", false, "Show help")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&verbose, "v", false, "Verbose output")
	flag.BoolVar(&version, "version", false, "Show version")
	flag.StringVar(&formatConfig.WorkDir, "work-dir", "", "Work directory path. Unspecified or blank means using system temp path")
	flag.StringVar(&selectedPr, "pages", "1-", "Page range (Ex. '4-10, 15, 39-'). Default is all pages. Open right range means to the end.")
	flag.StringVar(&bookInfo.Title, "title", "", "Book title. This affects epub/kepub output. Unspecified or blank means using filename without extension")
	flag.StringVar(&bookInfo.Author, "author", "", "Book author. This affects epub/kepub output. Unspecified or blank means 'Anonymous'")
	flag.Float64Var(&bookConfig.Density, "density", 300.0, "Output density (DPI)")
	flag.BoolVar(&bookConfig.IsRTL, "rtl", false, "Right-to-left read direction (ex. Japanese manga)")
	flag.BoolVar(&bookConfig.IsRTL, "right-to-left", false, "Right-to-left read direction (ex. Japanese manga)")
	flag.BoolVar(&formatConfig.Trim.Enabled, "trim", true, "Enable trim edge")
	flag.Float64Var(&formatConfig.Trim.FuzzP, "fuzz", 0.1, "Color fuzz (percentage)[0.0-1.0]")
	flag.Float64Var(&formatConfig.Trim.MinSizeP, "trim-min-size", 0.85, "Minimum size after trimmed (percentage)[0.0-1.0]")
	flag.IntVar(&formatConfig.Trim.Margin, "trim-margin", 10, "Safety trim margin (pixel)")
	flag.BoolVar(&formatConfig.Spread.Enabled, "spread", true, "Enable double-page spread detection and connection")
	flag.BoolVar(&formatConfig.Spread.KeepOrientation, "spread-keep-orientation", false, "Keep the page original orientation. Do not rotate to maximize screen area")
	flag.BoolVar(&formatConfig.Spread.KeepOriginal, "spread-keep-original", false, "Keep the original left and right page")
	sd := spread.NewSpreadDetector()
	flag.IntVar(&formatConfig.Spread.EdgeWidth, "spread-edge", sd.EdgeStripWidth, "Edge width for double-page spread detection (pixel)")
	flag.Float64Var(&formatConfig.Spread.Confidence, "spread-confidence", sd.SpreadThreshold, "Confidence threshold for double-page spread detection. The higher the value, the stricter the criteria become. (percentage)[0.0-1.0]")
	flag.BoolVar(&formatConfig.Resize.Enabled, "resize", true, "Resize to aspect fit in output screen size")
	flag.UintVar(&formatConfig.Resize.ScreenSize.Width, "width", 1264, "Output screen width (pixel)")
	flag.UintVar(&formatConfig.Resize.ScreenSize.Height, "height", 1680, "Output screen heigt (pixel)")
	flag.StringVar(&grayscalePr, "grayscale", "2-", "Page range (Ex. '4-10, 15, 39-') to convert to grayscale. Default is all pages except the first page(cover). 'false' means no grayscale conversion")
	flag.UintVar(&formatConfig.Grayscale.ColorDepth, "grayscale-depth", 4, "Grayscale color depth in number of bits. Possible values are 1, 2, 4, 8, 16 bits. No upscale if source image is in lower depth.")
	flag.BoolVar(&convertOnly, "convert-only", false, "Convert from input to output format only without any modification to the pages at all")
	flag.Var(&outputFormat, "format", "Output file format. The supported formats\n\t- raw (default)\n\t- cbz\n\t- epub\n\t- kepub")
	flag.StringVar(&outputFile, "output", "", "Output file/directory. Unspecified or blank means using the same file name as input file. For multiple input files, this argument will be output directory")
	flag.IntVar(&parallel, "parallel", max(1, runtime.NumCPU()/2), "Control the number of concurrent jobs. Zero or negative means unlimit. Default is half number of available CPUs. This is application for multiple input files only")
	flag.BoolVar(&logToFile, "log-file", false, "Print logs to file in addition to console")

	// Parse command-line
	flag.Parse()
	if help {
		flag.Usage()
		os.Exit(0)
	} else if version {
		showVersion()
		os.Exit(0)
	} else if flag.Arg(0) == "" {
		flag.Usage()
		os.Exit(1)
	}

	if verbose {
		clog.SetLevel(clog.DebugLevel)
	} else {
		clog.SetLevel(clog.InfoLevel)
	}
	consoleLogger := log.Wrap(newLogger(os.Stdout))
	defer handlePanic(consoleLogger)
	printLogHeader(consoleLogger)

	// Expand input file lists
	inputFiles := make([]string, 0)
	for i, arg := range flag.Args() {
		if strings.Contains(arg, "*") {
			// Expand the wildcard pattern
			matches := util.Must1(filepath.Glob(arg))(fmt.Sprintf("expanding input file(%d)", i))
			inputFiles = append(inputFiles, matches...)
		} else {
			inputFiles = append(inputFiles, arg)
		}
	}
	if len(inputFiles) == 0 {
		flag.Usage()
		os.Exit(1)
	}
	// quick check input files
	for i, inputFile := range inputFiles {
		inputFiles[i] = util.Must1(util.IsReadableFile(inputFile))("checking input file path")
	}

	if convertOnly {
		// disable all formatting
		formatConfig.Spread.Enabled = false
		formatConfig.Trim.Enabled = false
		formatConfig.Resize.Enabled = false
		formatConfig.Grayscale.Enabled = false
	}
	formatConfig.Trim.FuzzP = max(min(formatConfig.Trim.FuzzP, 1.0), 0.0)
	formatConfig.Trim.MinSizeP = max(min(formatConfig.Trim.MinSizeP, 1.0), 0.0)
	if formatConfig.Grayscale.Enabled {
		util.Must(book.IsSupportedColorDepth(formatConfig.Grayscale.ColorDepth))("checking grayscale color depth")
	}

	// quick check output files
	outputFiles := make([]string, len(inputFiles))
	outputFile = strings.TrimSpace(outputFile)
	for i, inputFile := range inputFiles {
		if outputFile == "" {
			outputFiles[i] = util.ReplaceExt(inputFile, outputFormat.Ext())
		} else {
			if len(inputFiles) > 1 {
				outputDir := outputFile
				outputName := filepath.Base(util.ReplaceExt(inputFile, outputFormat.Ext()))
				outputFiles[i] = filepath.Join(outputDir, outputName)
				outputFiles[i] = util.Must1(util.IsWritableFile(outputFiles[i]))("checking output file path")
			} else {
				outputFiles[i] = util.Must1(util.IsWritableFile(outputFile))("checking output file path")
			}
		}
	}

	// Create work dir
	util.Must1(util.CreateWorkDir(&formatConfig.WorkDir, true))("creating work directory")
	defer os.RemoveAll(formatConfig.WorkDir)

	if len(inputFiles) > 1 {
		// multiple input files mode
		consoleLogger.Infof("Total %d books", len(inputFiles))
	}

	// create jobs
	jobs := make([]*Job, 0, len(inputFiles))
	closeJobs := func() {
		for _, job := range jobs {
			job.Close()
		}
	}
	defer closeJobs()
	for i := range inputFiles {
		var bookLogger log.Logger
		if len(inputFiles) > 1 {
			bookLogger = consoleLogger.Indent(fmt.Sprintf("> Book[%d] ", i+1))
		} else {
			bookLogger = consoleLogger.Indent("> Book ")
		}
		job, err := NewJob(inputFiles[i], bookInfo, bookConfig, formatConfig, selectedPr, grayscalePr, outputFiles[i], outputFormat, bookLogger, logToFile)
		if err != nil {
			bookLogger.Error("Error while initializing:", "err", err)
			os.Exit(1)
			return
		}
		jobs = append(jobs, job)
	}

	if len(jobs) > 1 {
		// multiple books mode

		consoleLogger.Info("Start processing books", "total", len(jobs), "parallel", parallel)
		// processing each job
		wg := &errgroup.Group{}
		if parallel > 0 {
			wg.SetLimit(parallel)
		}
		for i, job := range jobs {
			logger := consoleLogger.Indent(fmt.Sprintf("> Book[%d] ", i+1))
			wg.Go(func() error {
				logger.Info("Starting processing")
				job.Process()
				logger.Info("Finish processing")
				return nil
			})
		}

		// wait for all jobs to finish
		wg.Wait()

		success, failure := 0, 0
		for i, job := range jobs {
			logger := consoleLogger.Indent(fmt.Sprintf("> Book[%d] ", i+1))
			if job.err != nil {
				failure = failure + 1
				logger.Error("FAILURE", "file", job.Book.Filepath, "err", job.err)
			} else {
				success = success + 1
				logger.Info("SUCCESS", "file", job.Book.Filepath)
			}
		}
		consoleLogger.Info("Total Results", "success", success, "failure", failure)
		if failure > 0 {
			os.Exit(1)
		}
	} else if len(jobs) == 1 {
		// single book mode
		job := jobs[0]
		job.Process()
		if job.err != nil {
			os.Exit(1)
		}
	}
}

type Job struct {
	Book         *book.Book
	PageRange    book.PageRange
	FormatConfig book.FormatConfig
	OutputFile   string
	OutputFormat format.OutputFormat
	Logger       log.Logger
	logsBuffer   *bytes.Buffer
	logFile      *os.File

	err error
}

func NewJob(inputFile string, info book.Info, cfg book.Config, formatConfig book.FormatConfig, selectedPr string, grayscalePr string, outputFile string, outputFormat format.OutputFormat, logger log.Logger, logToFile bool) (*Job, error) {
	var err error

	job := &Job{
		FormatConfig: formatConfig,
		OutputFile:   outputFile,
		OutputFormat: outputFormat,
		Logger:       logger,
	}

	if logToFile {
		// buffered logs in addition to console logger to flush later when all jobs have been initialized
		job.logsBuffer = new(bytes.Buffer)
		bufLogger := log.Wrap(newLogger(job.logsBuffer))
		printLogHeader(bufLogger)

		job.Logger = log.MultiLogger(logger, bufLogger.Indent("> Book "))
	}

	job.Logger.Info("Loading book", "file", inputFile)
	job.Logger.Debug("Output", "file", job.OutputFile)

	// Create job work dir
	job.FormatConfig.WorkDir = filepath.Join(job.FormatConfig.WorkDir, util.NameWithoutExt(filepath.Base(inputFile)))
	err = os.MkdirAll(job.FormatConfig.WorkDir, 0750)
	if err != nil {
		return nil, fmt.Errorf("creating job work directory: %w", err)
	}
	job.Logger.Debug("Work directory", "path", job.FormatConfig.WorkDir)

	// Load input book file
	job.Book, err = book.NewBook(inputFile, info, cfg, job.Logger)
	if err != nil {
		return nil, fmt.Errorf("loading book: %w", err)
	}
	job.Logger.Infof("Total Number of Pages: %d", job.Book.PageCount)

	// Parse selected page range argument
	job.PageRange = *book.NewPageRange()
	if err := job.PageRange.Parse(selectedPr, job.Book.PageCount); err != nil {
		return nil, fmt.Errorf("parsing page range(%s): %w", selectedPr, err)
	}

	// Parse grayscale page range argument
	if formatConfig.Grayscale.Enabled && strings.ToLower(grayscalePr) != "false" {
		formatConfig.Grayscale.PageRange = *book.NewPageRange()
		if err := formatConfig.Grayscale.PageRange.Parse(grayscalePr, job.Book.PageCount); err != nil {
			return nil, fmt.Errorf("parsing grayscale page range(%s): %w", grayscalePr, err)
		}
	}

	// revert to original logger
	job.Logger = logger

	return job, nil
}

func (j *Job) Close() {
	if j.logFile != nil {
		j.logFile.Close()
		j.logFile = nil
	}
	j.logsBuffer = nil
}

func (j *Job) Process() (err error) {
	defer func() {
		if r := recover(); r != nil {
			// Log the panic or handle it as an error
			j.Logger.Error("Panic while processing:", "err", r)
			j.err = fmt.Errorf("panic while processing: %v", r)
			err = j.err
		}
	}()
	j.err = j.doProcess()
	if j.err != nil {
		j.Logger.Error("Error while processing:", "err", j.err)
	}
	return j.err
}

func (j *Job) doProcess() error {
	// Flush buffered logs to logger
	if j.logsBuffer != nil {
		// set logger output to log file only
		var err error
		logFilepath := util.ReplaceExt(j.OutputFile, "log")
		j.logFile, err = os.OpenFile(logFilepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("creating book log file: %w", err)
		}
		j.Logger = log.MultiLogger(j.Logger, log.Wrap(newLogger(j.logFile)).Indent("> Book "))

		// Write buffer directly to file
		_, err = j.logsBuffer.WriteTo(j.logFile)
		if err != nil {
			return fmt.Errorf("flushing initialization to log file: %w", err)
		}
	}

	// Format book
	formattedBook, err := j.Book.Format(j.PageRange, j.FormatConfig, j.Logger)
	if err != nil {
		return fmt.Errorf("formatting book: %w", err)
	}

	// Packaging
	switch j.OutputFormat {
	case format.RAW:
		err = format.SaveAsRaw(*formattedBook, j.OutputFile, j.Logger)
	case format.CBZ:
		err = format.SaveAsCBZ(*formattedBook, j.OutputFile, j.Logger)
	case format.EPUB:
		err = format.SaveAsEPUB(*formattedBook, j.OutputFile, j.Logger)
	case format.KEPUB:
		err = format.SaveAsKEPUB(*formattedBook, j.OutputFile, j.Logger)
	}
	if err != nil {
		return fmt.Errorf("saving in %s format: %w", j.OutputFormat, err)
	}
	j.Logger.Infof("Total Input %d page(s). Total Output %d pages(s).", j.PageRange.PageCount(), len(formattedBook.Pages))

	return nil
}
