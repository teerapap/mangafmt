package volume

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"strings"

	"github.com/teerapap/mangafmt/internal/log"
	"rsc.io/pdf"
)

// pdfSource reads pages from a pdf input file. Each page is rasterized into a
// temporary image file by an external tool before it is loaded.
type pdfSource struct {
	filepath  string
	pageCount int
	extractor PdfPageExtractor
}

func newPdfSource(path string, logger log.Logger) (*pdfSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening input pdf file: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("checking input pdf file size: %w", err)
	}
	r, err := pdf.NewReader(f, fi.Size())
	if err != nil {
		return nil, fmt.Errorf("reading input pdf file: %w", err)
	}

	extractor, err := FindPdfExtractor(logger)
	if err != nil {
		return nil, err
	}

	return &pdfSource{
		filepath:  path,
		pageCount: r.NumPage(),
		extractor: extractor,
	}, nil
}

func (s *pdfSource) Name() string {
	return "PDF"
}

func (s *pdfSource) PageCount() int {
	return s.pageCount
}

func (s *pdfSource) LoadImage(pageNo int, cfg Config, workDir string, logger log.Logger) (image.Image, error) {
	// create temp file
	tmpFile, err := os.CreateTemp(workDir, "mangafmt-*.jpg")
	if err != nil {
		return nil, fmt.Errorf("create tmp file for input file(%s) at page %d: %w", s.filepath, pageNo, err)
	}
	filename := tmpFile.Name()
	defer os.RemoveAll(filename)
	defer tmpFile.Close()

	// extract page from pdf file
	logger.Debug("Extracting page -", "page_no", pageNo, "tool", s.extractor.Name())
	if err = s.extractor.Extract(s.filepath, pageNo, cfg.Density, filename, logger); err != nil {
		return nil, fmt.Errorf("extracting pdf page to tmp file %s: %w", filename, err)
	}

	// load image file
	img, format, err := image.Decode(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("loading tmp image file %s: %w", filename, err)
	}
	logger.Debug("Extracted page -", "page_no", pageNo, "file", filename, "format", format)

	return img, nil
}

type PdfPageExtractor interface {
	Name() string
	Detect() error
	Extract(inputFile string, page int, dpi float64, outputFile string, logger log.Logger) error
}

func FindPdfExtractor(logger log.Logger) (PdfPageExtractor, error) {
	extractors := []PdfPageExtractor{vips{}, imagemagick7{}, imagemagick6{}}

	for _, ext := range extractors {
		if err := ext.Detect(); err != nil {
			logger.Debug("Cannot find tool -", "name", ext.Name(), "error", err)
		} else {
			// found the extractor
			logger.Debug("Found tool installed", "name", ext.Name())
			return ext, nil
		}
	}

	return nil, fmt.Errorf("either ImageMagick or VIPS(libvips) is required to extract page from pdf file")
}

type imagemagick6 struct {
}

func (i imagemagick6) Name() string {
	return "ImageMagick6"
}

func (i imagemagick6) Detect() error {
	path, err := exec.LookPath("convert")
	if strings.Contains(strings.ToLower(path), "system32") {
		// Windows system convert.exe
		return fmt.Errorf("ImageMagick6 convert utility is not found but convert.exe is found at %s", path)
	}
	return err
}

func (i imagemagick6) Extract(inputFile string, page int, dpi float64, outputFile string, logger log.Logger) error {
	logger = logger.Indent("> Load      ")
	pageFile := fmt.Sprintf("%s[%d]", inputFile, page-1)
	cmd := exec.Command("convert", "-density", fmt.Sprintf("%0.2f", dpi), "-define", "pdf:use-cropbox=true", "-auto-orient", pageFile, outputFile)
	out, err := cmd.CombinedOutput()
	logger.Debug("Run", "name", i.Name(), "cmd", cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	} else {
		logger.Debug("Done", "name", i.Name(), "output", cmd)
	}
	return nil
}

type imagemagick7 struct {
}

func (i imagemagick7) Name() string {
	return "ImageMagick7"
}

func (i imagemagick7) Detect() error {
	_, err := exec.LookPath("magick")
	return err
}

func (i imagemagick7) Extract(inputFile string, page int, dpi float64, outputFile string, logger log.Logger) error {
	logger = logger.Indent("> Load      ")
	pageFile := fmt.Sprintf("%s[%d]", inputFile, page-1)
	cmd := exec.Command("magick", "-density", fmt.Sprintf("%0.2f", dpi), "-define", "pdf:use-cropbox=true", "-auto-orient", pageFile, outputFile)
	out, err := cmd.CombinedOutput()
	logger.Debug("Run", "name", i.Name(), "cmd", cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	} else {
		logger.Debug("Done", "name", i.Name(), "output", cmd)
	}
	return nil
}

type vips struct {
}

func (v vips) Name() string {
	return "VIPS"
}

func (v vips) Detect() error {
	_, err := exec.LookPath("vips")
	return err
}

func (v vips) Extract(inputFile string, page int, dpi float64, outputFile string, logger log.Logger) error {
	logger = logger.Indent("> Load      ")
	pageFile := fmt.Sprintf("%s[page=%d,dpi=%0.2f]", inputFile, page-1, dpi)
	cmd := exec.Command("vips", "copy", pageFile, outputFile)
	out, err := cmd.CombinedOutput()
	logger.Debug("Run", "name", v.Name(), "cmd", cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	} else {
		logger.Debug("Done", "name", v.Name(), "output", cmd)
	}
	return nil
}
