//
// epub.go
// Copyright (C) 2026 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package volume

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/teerapap/mangafmt/internal/log"
)

const (
	epubContainerPath = "META-INF/container.xml"
	opfMediaType      = "application/oebps-package+xml"
	ncxMediaType      = "application/x-dtbncx+xml"
)

// epubSource reads pages from a manga epub input file - an epub file whose
// spine documents are images. The images are read directly from the epub zip
// archive so no external tool is required.
type epubSource struct {
	filepath string
	pages    []epubPage
}

// epubPage is an image page resolved from a spine item
type epubPage struct {
	zipPath   string // path of the image file inside the epub zip archive
	mediaType string
}

// epubContainer is META-INF/container.xml
type epubContainer struct {
	Rootfiles []epubRootfile `xml:"rootfiles>rootfile"`
}

type epubRootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

// epubPackage is the epub package document(.opf)
type epubPackage struct {
	Manifest struct {
		Items []epubManifestItem `xml:"item"`
	} `xml:"manifest"`
	Spine epubSpine `xml:"spine"`
}

type epubManifestItem struct {
	Id         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	MediaType  string `xml:"media-type,attr"`
	Properties string `xml:"properties,attr"`
}

type epubSpine struct {
	Direction string `xml:"page-progression-direction,attr"`
	Toc       string `xml:"toc,attr"`
	ItemRefs  []struct {
		IdRef string `xml:"idref,attr"`
	} `xml:"itemref"`
}

func newEpubSource(filePath string, logger log.Logger) (*epubSource, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening input epub file: %w", err)
	}
	defer r.Close()

	files := make(map[string]*zip.File, len(r.File))
	for _, f := range r.File {
		files[path.Clean(f.Name)] = f
	}

	opfPath, err := readOpfPath(files)
	if err != nil {
		return nil, err
	}
	logger.Debug("Found epub package document -", "path", opfPath)

	var pkg epubPackage
	if err := unmarshalZipFile(files[opfPath], &pkg); err != nil {
		return nil, fmt.Errorf("reading epub package document(%s): %w", opfPath, err)
	}

	pages, err := resolveEpubPages(files, opfPath, pkg, logger)
	if err != nil {
		return nil, err
	}
	logger.Debug("Resolved epub pages -", "total_pages", len(pages), "total_spine_items", len(pkg.Spine.ItemRefs))

	return &epubSource{
		filepath: filePath,
		pages:    pages,
	}, nil
}

func (s *epubSource) Name() string {
	return "EPUB"
}

func (s *epubSource) PageCount() int {
	return len(s.pages)
}

func (s *epubSource) LoadImage(pageNo int, cfg Config, workDir string, logger log.Logger) (image.Image, error) {
	if pageNo < 1 || pageNo > len(s.pages) {
		return nil, fmt.Errorf("page %d is out of range(1-%d)", pageNo, len(s.pages))
	}
	page := s.pages[pageNo-1]

	r, err := zip.OpenReader(s.filepath)
	if err != nil {
		return nil, fmt.Errorf("opening input epub file: %w", err)
	}
	defer r.Close()

	var file *zip.File
	for _, f := range r.File {
		if path.Clean(f.Name) == page.zipPath {
			file = f
			break
		}
	}
	if file == nil {
		return nil, fmt.Errorf("image file(%s) is not found in the input epub file", page.zipPath)
	}

	rc, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("opening image file(%s) in the input epub file: %w", page.zipPath, err)
	}
	defer rc.Close()

	img, format, err := image.Decode(rc)
	if err != nil {
		return nil, fmt.Errorf("decoding image file(%s in %s format) in the input epub file: %w", page.zipPath, page.mediaType, err)
	}
	logger.Debug("Extracted page -", "page_no", pageNo, "file", page.zipPath, "format", format)

	return img, nil
}

// readOpfPath reads the path of the epub package document(.opf) from the
// container file
func readOpfPath(files map[string]*zip.File) (string, error) {
	f, found := files[epubContainerPath]
	if !found {
		return "", fmt.Errorf("input epub file has no %s. It is not a valid epub file", epubContainerPath)
	}

	var container epubContainer
	if err := unmarshalZipFile(f, &container); err != nil {
		return "", fmt.Errorf("reading %s: %w", epubContainerPath, err)
	}

	for _, rootfile := range container.Rootfiles {
		if rootfile.MediaType != "" && rootfile.MediaType != opfMediaType {
			continue
		}
		opfPath := resolveHref("", rootfile.FullPath)
		if _, found := files[opfPath]; !found {
			return "", fmt.Errorf("epub package document(%s) is not found in the input epub file", opfPath)
		}
		return opfPath, nil
	}
	return "", fmt.Errorf("input epub file has no package document in %s. It is not a valid epub file", epubContainerPath)
}

// resolveEpubPages resolves each spine item into an image page in reading
// order. A spine item without an image is skipped with a warning. The volume
// must have at least one image page.
func resolveEpubPages(files map[string]*zip.File, opfPath string, pkg epubPackage, logger log.Logger) ([]epubPage, error) {
	items := make(map[string]epubManifestItem, len(pkg.Manifest.Items))
	for _, item := range pkg.Manifest.Items {
		items[item.Id] = item
	}

	pages := make([]epubPage, 0, len(pkg.Spine.ItemRefs))
	for _, ref := range pkg.Spine.ItemRefs {
		item, found := items[ref.IdRef]
		if !found {
			logger.Warn("Skipping page. The spine item is not found in the manifest -", "idref", ref.IdRef)
			continue
		}
		if isNavItem(item) {
			// the table of content document is not a page
			continue
		}

		itemPath := resolveHref(opfPath, item.Href)
		mediaType := strings.ToLower(strings.TrimSpace(item.MediaType))

		switch {
		case strings.HasPrefix(mediaType, "image/"):
			// the spine item is the image itself
			file, found := files[itemPath]
			if !found {
				logger.Warn("Skipping page. The image file is not found in the input epub file -", "page", item.Href)
				continue
			}
			pages = append(pages, epubPage{zipPath: path.Clean(file.Name), mediaType: mediaType})
		case isDocumentMediaType(mediaType):
			page, found := resolveEpubPage(files, itemPath, logger)
			if !found {
				continue
			}
			pages = append(pages, page)
		default:
			logger.Warn("Skipping page. The spine item is neither an image nor a document -", "page", item.Href, "media_type", item.MediaType)
		}
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("input epub file has no image page. Only manga epub file(a volume of image pages) is supported")
	}
	return pages, nil
}

// resolveEpubPage resolves a spine document into the image page it shows. If
// the document shows more than one image, the largest image is used.
func resolveEpubPage(files map[string]*zip.File, docPath string, logger log.Logger) (epubPage, bool) {
	doc, found := files[docPath]
	if !found {
		logger.Warn("Skipping page. The page document is not found in the input epub file -", "page", docPath)
		return epubPage{}, false
	}

	refs, err := findImageRefs(doc)
	if err != nil {
		// best effort. the images found before the error are still used
		logger.Debug("Error while parsing the page document -", "page", docPath, "err", err)
	}

	candidates := make([]*zip.File, 0, len(refs))
	seen := make(map[string]bool, len(refs))
	for _, ref := range refs {
		if isRemoteHref(ref) {
			logger.Debug("Ignoring remote image in the page document -", "page", docPath, "image", ref)
			continue
		}
		imgPath := resolveHref(docPath, ref)
		if seen[imgPath] {
			continue
		}
		seen[imgPath] = true

		file, found := files[imgPath]
		if !found {
			logger.Debug("Ignoring image which is not found in the input epub file -", "page", docPath, "image", imgPath)
			continue
		}
		candidates = append(candidates, file)
	}

	if len(candidates) == 0 {
		logger.Warn("Skipping page. The page in the input epub file has no image -", "page", docPath)
		return epubPage{}, false
	}

	largest := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.UncompressedSize64 > largest.UncompressedSize64 {
			largest = candidate
		}
	}
	if len(candidates) > 1 {
		logger.Warn("The page in the input epub file has more than one image. Using the largest image -", "page", docPath, "total_images", len(candidates), "using", largest.Name)
	}

	return epubPage{
		zipPath:   path.Clean(largest.Name),
		mediaType: mediaTypeByExt(largest.Name),
	}, true
}

// findImageRefs finds all image references in the document
func findImageRefs(f *zip.File) ([]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("opening %s in the input epub file: %w", f.Name, err)
	}
	defer rc.Close()

	decoder := newXmlDecoder(rc)
	refs := make([]string, 0, 1)
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return refs, err
		}

		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		var ref string
		switch strings.ToLower(start.Name.Local) {
		case "img":
			ref = attrValue(start, "src")
		case "image": // svg image
			ref = attrValue(start, "href")
		case "object":
			ref = attrValue(start, "data")
		}
		if strings.TrimSpace(ref) != "" {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

func attrValue(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if strings.EqualFold(attr.Name.Local, name) {
			return attr.Value
		}
	}
	return ""
}

// isNavItem checks if the manifest item is a table of content document
func isNavItem(item epubManifestItem) bool {
	if strings.EqualFold(strings.TrimSpace(item.MediaType), ncxMediaType) {
		return true
	}
	for _, prop := range strings.Fields(item.Properties) {
		if strings.EqualFold(prop, "nav") {
			return true
		}
	}
	return false
}

func isDocumentMediaType(mediaType string) bool {
	switch mediaType {
	case "application/xhtml+xml", "text/html", "application/x-dtbook+xml", "image/svg+xml":
		return true
	default:
		return false
	}
}

func isRemoteHref(href string) bool {
	return strings.Contains(href, "://") || strings.HasPrefix(href, "data:")
}

func mediaTypeByExt(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// resolveHref resolves a href referenced in the file at basePath into a path
// inside the epub zip archive
func resolveHref(basePath string, href string) string {
	// drop the fragment and query part
	if i := strings.IndexAny(href, "#?"); i >= 0 {
		href = href[:i]
	}
	// hrefs are url-encoded but zip entry names are not
	if unescaped, err := url.PathUnescape(href); err == nil {
		href = unescaped
	}

	if strings.HasPrefix(href, "/") {
		return path.Clean(strings.TrimPrefix(href, "/"))
	}
	return path.Clean(path.Join(path.Dir(basePath), href))
}

func newXmlDecoder(r io.Reader) *xml.Decoder {
	decoder := xml.NewDecoder(r)
	// epub documents in the wild are not always well-formed. Strict=false
	// invents the missing end tags. AutoClose is not set on purpose because
	// it breaks self-closing xhtml tags such as <meta/>
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		// epub documents are utf-8 encoded. read as-is for the others
		return input, nil
	}
	return decoder
}

func unmarshalZipFile(f *zip.File, v any) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening %s in the input epub file: %w", f.Name, err)
	}
	defer rc.Close()

	if err := newXmlDecoder(rc).Decode(v); err != nil {
		return fmt.Errorf("parsing %s: %w", f.Name, err)
	}
	return nil
}
