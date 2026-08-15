package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	clog "charm.land/log/v2"
	"golang.org/x/sync/errgroup"

	"github.com/aquilax/truncate"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/spread"
	"github.com/teerapap/mangafmt/internal/util"
	"github.com/teerapap/mangafmt/internal/volume"
	"github.com/teerapap/mangafmt/internal/volume/format"
)

func showVersion() {
	fmt.Printf("mangafmt-%s\n", util.AppVersion)
}

// Helper functions

func newLogger(w io.Writer) *clog.Logger {
	return clog.NewWithOptions(w, clog.Options{
		Level:           clog.GetLevel(),
		ReportTimestamp: true,
	})
}

func printLogHeader(logger log.Logger) error {
	logger.Debug("mangafmt", "ver", util.AppVersion, "args", os.Args)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting current directory: %w", err)
	}
	logger.Debug("current directory", "dir", cwd)
	return nil
}

func main() {
	ec := run()
	if ec != 0 {
		os.Exit(ec)
	}
}

func run() (ec int) {
	// Command-line flags
	var help bool
	var verbose bool
	var version bool
	var selectedPr string
	var volumeInfo volume.Info
	var volumeConfig volume.Config
	var formatConfig volume.FormatConfig
	var grayscalePr string
	var outputFile string
	var outputFormat format.OutputFormat
	var convertOnly bool
	var parallel int
	var logToFile bool
	var showProgressBar bool

	flag.Usage = func() {
		output := flag.CommandLine.Output()
		fmt.Fprintf(output, "%s [options] <input_file> [input_file2...]\n\n", os.Args[0])
		fmt.Fprintf(output, "Options:\n")
		util.PrintFlagsUsage(output)
	}
	flag.BoolVar(&help, "help", false, "Show help")
	flag.BoolVar(&help, "h", false, "Show help")
	flag.BoolVar(&verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&verbose, "v", false, "Verbose output")
	flag.BoolVar(&version, "version", false, "Show version")
	flag.StringVar(&formatConfig.WorkDir, "work-dir", "", "Work directory path. Unspecified or blank means using system temp path")
	flag.StringVar(&selectedPr, "pages", "1-", "Page range (Ex. '4-10, 15, 39-'). Default is all pages. Open right range means to the end.")
	flag.StringVar(&volumeInfo.Title, "title", "", "Volume title. This affects epub/kepub output. Unspecified or blank means using filename without extension")
	flag.StringVar(&volumeInfo.Author, "author", "", "Volume author. This affects epub/kepub output. Unspecified or blank means 'Anonymous'")
	flag.Float64Var(&volumeConfig.Density, "density", 300.0, "Output density (DPI). This affects pdf input only")
	flag.BoolVar(&volumeConfig.IsRTL, "rtl", false, "Right-to-left read direction (ex. Japanese manga)")
	flag.BoolVar(&volumeConfig.IsRTL, "right-to-left", false, "Right-to-left read direction (ex. Japanese manga)")
	flag.BoolVar(&formatConfig.Trim.Enabled, "trim", true, "Enable/disable edge trimming")
	flag.Float64Var(&formatConfig.Trim.FuzzP, "fuzz", 0.1, "Color fuzz (percentage)[0.0-1.0]")
	flag.Float64Var(&formatConfig.Trim.MinSizeP, "trim-min-size", 0.85, "Minimum size after trimmed (percentage)[0.0-1.0]")
	flag.IntVar(&formatConfig.Trim.Margin, "trim-margin", 10, "Safety trim margin (pixel)")
	flag.BoolVar(&formatConfig.Spread.Enabled, "spread", true, "Enable/disable double-page spread detection and connection")
	flag.BoolVar(&formatConfig.Spread.KeepOrientation, "spread-keep-orientation", false, "Keep the page original orientation. Do not rotate to maximize screen area")
	flag.BoolVar(&formatConfig.Spread.KeepOriginal, "spread-keep-original", false, "Keep the original left and right page")
	sd := spread.NewSpreadDetector()
	flag.IntVar(&formatConfig.Spread.EdgeWidth, "spread-edge", sd.EdgeStripWidth, "Edge width for double-page spread detection (pixel)")
	flag.Float64Var(&formatConfig.Spread.Confidence, "spread-confidence", sd.SpreadThreshold, "Confidence threshold for double-page spread detection. The higher the value, the stricter the criteria become. (percentage)[0.0-1.0]")
	flag.BoolVar(&formatConfig.Resize.Enabled, "resize", true, "Enable/disable resize to aspect fit in output screen size")
	flag.UintVar(&formatConfig.Resize.ScreenSize.Width, "width", 1264, "Output screen width (pixel)")
	flag.UintVar(&formatConfig.Resize.ScreenSize.Height, "height", 1680, "Output screen heigt (pixel)")
	flag.StringVar(&grayscalePr, "grayscale", "2-", "Page range (Ex. '4-10, 15, 39-') to convert to grayscale. Default is all pages except the first page(cover). 'false' means no grayscale conversion")
	flag.UintVar(&formatConfig.Grayscale.ColorDepth, "grayscale-depth", 4, "Grayscale color depth in number of bits. Possible values are 1, 2, 4, 8, 16 bits. No upscale if source image is in lower depth.")
	flag.BoolVar(&convertOnly, "convert-only", false, "Convert from input to output format only without any modification to the pages at all")
	flag.Var(&outputFormat, "format", "Output file format. The supported formats\n\t- raw\n\t- cbz\n\t- epub\n\t- kepub")
	flag.StringVar(&outputFile, "output", "", "Output file/directory. Unspecified or blank means using the same file name as input file. For multiple input files, this argument will be output directory")
	flag.IntVar(&parallel, "parallel", 2, "Control the number of concurrent jobs. Zero or negative means unlimit. This is application for multiple input files only")
	flag.BoolVar(&logToFile, "log-file", false, "Print logs to file in addition to console")
	flag.BoolVar(&showProgressBar, "progress-bar", false, "Show progress bar instead of logs (experimental)")

	// Parse command-line
	flag.Parse()
	if help {
		flag.Usage()
		return 0
	} else if version {
		showVersion()
		return 0
	} else if flag.Arg(0) == "" {
		flag.Usage()
		return 1
	}

	if verbose {
		clog.SetLevel(clog.DebugLevel)
	} else {
		clog.SetLevel(clog.InfoLevel)
	}
	consoleLogger := log.Wrap(newLogger(os.Stdout))
	util.Must(printLogHeader(consoleLogger))("printing log header", consoleLogger)

	// Expand input file lists
	inputFiles := make([]string, 0)
	for i, arg := range flag.Args() {
		if strings.Contains(arg, "*") {
			// Expand the wildcard pattern
			matches := util.Must1(filepath.Glob(arg))(fmt.Sprintf("expanding input file(%d)", i), consoleLogger)
			inputFiles = append(inputFiles, matches...)
		} else {
			inputFiles = append(inputFiles, arg)
		}
	}
	if len(inputFiles) == 0 {
		flag.Usage()
		return 1
	}
	// quick check input files
	for i, inputFile := range inputFiles {
		inputFiles[i] = util.Must1(util.IsReadableFile(inputFile))("checking input file path", consoleLogger)
	}

	if convertOnly {
		// disable all formatting
		formatConfig.Spread.Enabled = false
		formatConfig.Trim.Enabled = false
		formatConfig.Resize.Enabled = false
		formatConfig.Grayscale.Enabled = false
	} else {
		formatConfig.Grayscale.Enabled = true
	}
	formatConfig.Trim.FuzzP = max(min(formatConfig.Trim.FuzzP, 1.0), 0.0)
	formatConfig.Trim.MinSizeP = max(min(formatConfig.Trim.MinSizeP, 1.0), 0.0)
	if formatConfig.Grayscale.Enabled {
		util.Must(volume.IsSupportedColorDepth(formatConfig.Grayscale.ColorDepth))("checking grayscale color depth", consoleLogger)
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
				outputFiles[i] = util.Must1(util.IsWritableFile(outputFiles[i]))("checking output file path", consoleLogger)
			} else {
				outputFiles[i] = util.Must1(util.IsWritableFile(outputFile))("checking output file path", consoleLogger)
			}
		}
	}

	// Create work dir
	util.Must1(util.CreateWorkDir(&formatConfig.WorkDir, true))("creating work directory", consoleLogger)
	defer os.RemoveAll(formatConfig.WorkDir)

	if len(inputFiles) > 1 {
		// multiple input files mode
		consoleLogger.Infof("Total %d volumes", len(inputFiles))
	}

	// Create jobs from input files
	jobs := make([]*Job, 0, len(inputFiles))
	closeJobs := func() {
		for _, job := range jobs {
			job.Close()
		}
	}
	defer closeJobs()
	var progress *mpb.Progress
	refreshRate := 500 * time.Millisecond
	if showProgressBar {
		progress = mpb.New(
			mpb.WithAutoRefresh(),
			mpb.WithRefreshRate(refreshRate),
		)
	}

	for i := range inputFiles {
		var volumeLogger log.Logger
		if len(inputFiles) > 1 {
			volumeLogger = consoleLogger.Indent(fmt.Sprintf("> Volume[%d] ", i+1))
		} else {
			volumeLogger = consoleLogger.Indent("> Volume ")
		}
		job, err := NewJob(inputFiles[i], volumeInfo, volumeConfig, formatConfig, selectedPr, grayscalePr, outputFiles[i], outputFormat, volumeLogger, logToFile)
		if err != nil {
			volumeLogger.Error("Error while initializing:", "err", err)
			return 1
		}
		jobs = append(jobs, job)
	}

	// Process each jobs
	if len(jobs) > 1 {
		// multiple volumes mode
		runJobsInParallel(jobs, parallel, consoleLogger, progress)
		if progress != nil {
			time.Sleep(refreshRate) // wait for progress bar to render the last time
			progress.Wait()
		}

		success, failure := 0, 0
		for i, job := range jobs {
			logger := consoleLogger.Indent(fmt.Sprintf("> Volume[%d] ", i+1))
			if job.err != nil {
				failure = failure + 1
				logger.Error("FAILURE", "input", job.Volume.Filepath, "err", job.err)
			} else {
				success = success + 1
				logger.Info("SUCCESS", "input", job.Volume.Filepath, "output", job.OutputFile)
			}
		}
		consoleLogger.Info("Total Results", "success", success, "failure", failure)
		if failure > 0 {
			return 1
		}
	} else if len(jobs) == 1 {
		// single volume mode
		job := jobs[0]
		job.Process(progress)
		if progress != nil {
			time.Sleep(refreshRate) // wait for progress bar to render the last time
			progress.Wait()
		}
		if job.err != nil {
			return 1
		}
	}

	return 0
}

func runJobsInParallel(jobs []*Job, parallel int, logger log.Logger, progress *mpb.Progress) {
	logger.Info("Start processing volumes", "total", len(jobs), "parallel", parallel)

	// processing each job
	wg := &errgroup.Group{}
	if parallel > 0 {
		wg.SetLimit(parallel)
	}
	for i, job := range jobs {
		jobLogger := logger.Indent(fmt.Sprintf("> Volume[%d] ", i+1))
		if parallel <= 0 || i < parallel {
			job.decorateProgressBar(progress)
		}
		wg.Go(func() error {
			if progress == nil {
				jobLogger.Info("Starting processing")
			}
			job.Process(progress)
			if progress == nil {
				jobLogger.Info("Finish processing")
			}
			return nil
		})
	}

	// wait for all jobs to finish
	wg.Wait()
}

type Job struct {
	Volume        *volume.Volume
	PageRange     volume.PageRange
	FormatConfig  volume.FormatConfig
	OutputFile    string
	OutputFormat  format.OutputFormat
	Logger        log.Logger
	logsBuffer    *bytes.Buffer
	logFile       *os.File
	progressTotal int64
	progressBar   *mpb.Bar

	status atomic.Int32 // JobStatus
	err    error
}

type JobStatus int32

const (
	JobStatusWaiting JobStatus = iota
	JobStatusInitializing
	JobStatusFormatting
	JobStatusPackaging
	JobStatusError
	JobStatusDone
)

func (s JobStatus) String() string {
	switch s {
	case JobStatusWaiting:
		return "Waiting"
	case JobStatusInitializing:
		return "Initializing"
	case JobStatusFormatting:
		return "Formatting"
	case JobStatusPackaging:
		return "Packaging"
	case JobStatusError:
		return "Error"
	case JobStatusDone:
		return "Done"
	default:
		return "Unknown"
	}
}

func NewJob(inputFile string, info volume.Info, cfg volume.Config, formatConfig volume.FormatConfig, selectedPr string, grayscalePr string, outputFile string, outputFormat format.OutputFormat, logger log.Logger, logToFile bool) (*Job, error) {
	var err error

	job := &Job{
		FormatConfig: formatConfig,
		OutputFile:   outputFile,
		OutputFormat: outputFormat,
		Logger:       logger,
	}
	job.setStatus(JobStatusWaiting)

	if logToFile {
		// buffered logs in addition to console logger to flush later when all jobs have been initialized
		job.logsBuffer = new(bytes.Buffer)
		bufLogger := log.Wrap(newLogger(job.logsBuffer))
		printLogHeader(bufLogger)

		job.Logger = log.MultiLogger(logger, bufLogger.Indent("> Volume "))
	}

	job.Logger.Info("Loading volume", "file", inputFile)
	job.Logger.Debug("Output", "file", job.OutputFile)

	// Create job work dir
	job.FormatConfig.WorkDir = filepath.Join(job.FormatConfig.WorkDir, util.NameWithoutExt(filepath.Base(inputFile)))
	err = os.MkdirAll(job.FormatConfig.WorkDir, 0750)
	if err != nil {
		return nil, fmt.Errorf("creating job work directory: %w", err)
	}
	job.Logger.Debug("Work directory", "path", job.FormatConfig.WorkDir)

	// Load input volume file
	job.Volume, err = volume.NewVolume(inputFile, info, cfg, job.Logger)
	if err != nil {
		return nil, fmt.Errorf("loading volume: %w", err)
	}
	job.Logger.Infof("Total Number of Pages: %d", job.Volume.PageCount)

	// Parse selected page range argument
	job.PageRange = *volume.NewPageRange()
	if err := job.PageRange.Parse(selectedPr, job.Volume.PageCount); err != nil {
		return nil, fmt.Errorf("parsing page range(%s): %w", selectedPr, err)
	}

	// Parse grayscale page range argument
	if job.FormatConfig.Grayscale.Enabled && strings.ToLower(grayscalePr) != "false" {
		job.FormatConfig.Grayscale.PageRange = *volume.NewPageRange()
		if err := job.FormatConfig.Grayscale.PageRange.Parse(grayscalePr, job.Volume.PageCount); err != nil {
			return nil, fmt.Errorf("parsing grayscale page range(%s): %w", grayscalePr, err)
		}
	}

	// revert to default logger
	job.Logger = logger

	return job, nil
}

func (j *Job) Status() JobStatus {
	return JobStatus(j.status.Load())
}

func (j *Job) setStatus(status JobStatus) {
	j.status.Store(int32(status))
}

func (j *Job) decorateProgressBar(progress *mpb.Progress) {
	if j.progressBar != nil || progress == nil {
		return
	}
	// turn off default logger
	j.Logger = log.Wrap(newLogger(io.Discard))

	title := truncate.Truncate(j.Volume.Info.Title, 40, "...", truncate.PositionMiddle)
	j.progressTotal = 10000
	j.progressBar = progress.AddBar(j.progressTotal,
		mpb.PrependDecorators(
			decor.Name(title, decor.WC{C: decor.DSyncWidthR}),
		),
		mpb.AppendDecorators(
			decor.NewPercentage("%.1f", decor.WC{C: decor.DSyncWidth, W: 6}),
			decor.Name("|", decor.WC{C: decor.DSyncSpace}),
			decor.Elapsed(decor.ET_STYLE_GO, decor.WC{C: decor.DSyncSpace}),
			decor.Name("|", decor.WC{C: decor.DSyncSpace}),
			decor.Any(func(s decor.Statistics) string {
				return " " + j.Status().String()
			}, decor.WC{C: decor.DSyncWidthR, W: 13}),
		),
	)
}

func (j *Job) Close() {
	if j.logFile != nil {
		j.logFile.Close()
		j.logFile = nil
	}
	j.logsBuffer = nil
}

func (j *Job) Process(progress *mpb.Progress) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// Log the panic or handle it as an error
			j.Logger.Error("Panic while processing:", "err", r)
			j.err = fmt.Errorf("panic while processing: %v", r)
			if j.progressBar != nil {
				j.progressBar.Abort(false)
			}
			j.setStatus(JobStatusError)
			err = j.err
		}
	}()
	j.err = j.doProcess(progress)
	if j.err != nil {
		j.setStatus(JobStatusError)
		if j.progressBar != nil {
			j.progressBar.Abort(false)
		}
		j.Logger.Error("Error while processing:", "err", j.err)
	} else {
		j.setStatus(JobStatusDone)
		if j.progressBar != nil {
			j.progressBar.SetCurrent(j.progressTotal)
		}
	}

	return j.err
}

func (j *Job) doProcess(progress *mpb.Progress) error {
	// Setup progress bar if any
	j.decorateProgressBar(progress)

	// Flush buffered logs to logger
	if j.logsBuffer != nil {
		// set logger output to log file only
		var err error
		logFilepath := util.ReplaceExt(j.OutputFile, "log")
		j.logFile, err = os.OpenFile(logFilepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("creating volume log file: %w", err)
		}
		j.Logger = log.MultiLogger(j.Logger, log.Wrap(newLogger(j.logFile)).Indent("> Volume "))

		// Write buffer directly to file
		_, err = j.logsBuffer.WriteTo(j.logFile)
		if err != nil {
			return fmt.Errorf("flushing initialization to log file: %w", err)
		}
	}

	// Format volume
	j.setStatus(JobStatusFormatting)
	formattedVolume, err := j.Volume.Format(j.PageRange, j.FormatConfig, j.Logger, func(v *volume.Volume, completed float64, lastPageNo int) {
		if j.progressBar != nil {
			// 80%
			total := 0.8
			comp := min(total, total*completed)
			j.progressBar.SetCurrent(int64(comp * float64(j.progressTotal)))
		}
	})
	if err != nil {
		return fmt.Errorf("formatting volume: %w", err)
	}

	// Packaging
	j.setStatus(JobStatusPackaging)
	packagingProgress := func(completed float64) {
		if j.progressBar != nil {
			// 20%
			total := 0.2
			comp := min(total, total*completed) + 0.8
			j.progressBar.SetCurrent(int64(comp * float64(j.progressTotal)))
		}
	}
	switch j.OutputFormat {
	case format.OutputFormatRaw:
		err = format.SaveAsRaw(*formattedVolume, j.OutputFile, j.Logger, packagingProgress)
	case format.OutputFormatCbz:
		err = format.SaveAsCBZ(*formattedVolume, j.OutputFile, j.Logger, packagingProgress)
	case format.OutputFormatEpub:
		err = format.SaveAsEPUB(*formattedVolume, j.OutputFile, j.Logger, packagingProgress)
	case format.OutputFormatKepub:
		err = format.SaveAsKEPUB(*formattedVolume, j.OutputFile, j.Logger, packagingProgress)
	}
	if err != nil {
		return fmt.Errorf("saving in %s format: %w", j.OutputFormat, err)
	}
	j.Logger.Infof("Total Input %d page(s). Total Output %d pages(s).", j.PageRange.PageCount(), len(formattedVolume.Pages))

	return nil
}
