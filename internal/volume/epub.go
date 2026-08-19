//
// epub.go
// Copyright (C) 2026 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package volume

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"io"
	"net/url"
	"path"
	"strings"

	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/volume/format"
)

const (
	epubContainerPath  = "META-INF/container.xml"
	epubEncryptionPath = "META-INF/encryption.xml"
	opfMediaType       = "application/oebps-package+xml"
	ncxMediaType       = "application/x-dtbncx+xml"
	dcNamespace        = "http://purl.org/dc/elements/1.1/"
	opfNamespace       = "http://www.idpf.org/2007/opf"
)

// fontObfuscationAlgorithms are the algorithms which scramble the embedded
// font files. They are not DRM and no page depends on them.
var fontObfuscationAlgorithms = []string{
	"http://www.idpf.org/2008/embedding",
	"http://ns.adobe.com/pdf/enc#RC",
}

// epubSource reads pages from a manga epub input file - an epub file whose
// spine documents are images. The images are read directly from the epub zip
// archive so no external tool is required.
type epubSource struct {
	filepath string
	pages    []epubPage
	skipped  SkippedPages
	metadata SourceMetadata
}

// epubPage is an image page resolved from a spine item
type epubPage struct {
	zipPath   string // path of the image file inside the epub zip archive
	mediaType string
}

// epubSpineDoc is a spine item and the page it is resolved into. It is used to
// find the page an entry of the table of content lands on.
type epubSpineDoc struct {
	path   string
	pageNo int // zero when the spine item has no image page
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
	// Prefix is the vocabulary prefix declaration. A metadata property of a
	// vocabulary which is not reserved(ex. ibooks:) does not resolve without it.
	Prefix   string `xml:"prefix,attr"`
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

// epubEncryption is META-INF/encryption.xml. Both the DRM-protected epub file
// and the epub file with the obfuscated fonts have it.
type epubEncryption struct {
	Data []epubEncryptedData `xml:"EncryptedData"`
}

type epubEncryptedData struct {
	Method struct {
		Algorithm string `xml:"Algorithm,attr"`
	} `xml:"EncryptionMethod"`
	References []struct {
		URI string `xml:"URI,attr"`
	} `xml:"CipherData>CipherReference"`
}

func newEpubSource(filePath string, skipUnreadable bool, logger log.Logger) (*epubSource, error) {
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

	opfSrc, err := readZipFile(files[opfPath])
	if err != nil {
		return nil, fmt.Errorf("reading epub package document(%s): %w", opfPath, err)
	}

	var pkg epubPackage
	if err := unmarshalXml(opfSrc, &pkg); err != nil {
		return nil, fmt.Errorf("parsing epub package document(%s): %w", opfPath, err)
	}

	encrypted := readEncryptedPaths(files, logger)

	pages, spine, skipped, err := resolveEpubPages(files, opfPath, pkg, encrypted, skipUnreadable, logger)
	if err != nil {
		return nil, err
	}
	logger.Debug("Resolved epub pages -", "total_pages", len(pages), "total_spine_items", len(pkg.Spine.ItemRefs))

	metadata := readEpubMetadata(pkg, opfSrc, logger)
	metadata.Epub.TableOfContents = readEpubToc(files, opfPath, pkg, spine, encrypted, logger)

	return &epubSource{
		filepath: filePath,
		pages:    pages,
		skipped:  skipped,
		metadata: metadata,
	}, nil
}

func (s *epubSource) Name() string {
	return "EPUB"
}

func (s *epubSource) PageCount() int {
	return len(s.pages)
}

func (s *epubSource) Skipped() SkippedPages {
	return s.skipped
}

func (s *epubSource) Metadata() SourceMetadata {
	return s.metadata
}

// readEpubMetadata reads the volume information from the package document.
// The metadata elements which are regenerated in the output file are taken out
// and the remaining ones are kept as-is.
func readEpubMetadata(pkg epubPackage, opfSrc []byte, logger log.Logger) SourceMetadata {
	var meta SourceMetadata

	// the read direction of the volume
	switch strings.ToLower(strings.TrimSpace(pkg.Spine.Direction)) {
	case "rtl":
		meta.IsRTL = boolPtr(true)
	case "ltr":
		meta.IsRTL = boolPtr(false)
	}

	meta.Epub.Prefix = strings.TrimSpace(pkg.Prefix)

	nsAttrs, entries, err := readOpfMetadataEntries(opfSrc)
	if err != nil {
		// best effort. the metadata read before the error is still kept
		logger.Warn("Cannot read all metadata in the input epub file. Some of them may be missing in the output file -", "err", err)
	}
	meta.Epub.Namespaces = formatNamespaces(nsAttrs)

	names := make([]string, len(entries))
	regenerated := make(map[string]bool, len(entries)) // ids of the regenerated entries
	for i, entry := range entries {
		names[i] = regeneratedMetadataName(entry)
		if names[i] == "" {
			continue
		}
		// the value is regenerated in the output file from the volume info.
		// only the first one is taken when the input file has more than one
		if names[i] == "dc:title" && meta.Title == "" {
			meta.Title = entry.Text()
		} else if names[i] == "dc:creator" && meta.Author == "" {
			meta.Author = entry.Text()
		} else if names[i] == "dc:language" && meta.Language == "" {
			meta.Language = entry.Text()
		} else if names[i] == "dc:identifier" && meta.Identifier == "" {
			meta.Identifier = entry.Text()
		}
		if id := attrValue(entry.Attrs, "id"); id != "" {
			regenerated[id] = true
		}
	}

	meta.Epub.Entries = make([]format.EpubMetadataEntry, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for i, entry := range entries {
		if name := names[i]; name != "" {
			// the output file has only one of each regenerated element
			if seen[name] {
				continue
			}
			seen[name] = true
			meta.Epub.Entries = append(meta.Epub.Entries, format.EpubMetadataEntry{Name: name})
			continue
		}

		// drop the entries refining a regenerated entry
		refines := strings.TrimPrefix(strings.TrimSpace(attrValue(entry.Attrs, "refines")), "#")
		if refines != "" && regenerated[refines] {
			continue
		}
		meta.Epub.Entries = append(meta.Epub.Entries, format.EpubMetadataEntry{Raw: entry.Raw})
	}
	logger.Debug("Read metadata from the input epub file -", "entries", len(meta.Epub.Entries), "total_entries", len(entries))

	return meta
}

// regeneratedMetadataName is the name of the metadata element when mangafmt
// regenerates it in the output file. It is empty when the element is kept from
// the input file as-is.
func regeneratedMetadataName(entry epubMetadataEntry) string {
	if isDCElement(entry.Name) {
		local := strings.ToLower(entry.Name.Local)
		switch local {
		// mangafmt puts itself as the contributor of the output file
		case "title", "creator", "language", "identifier", "contributor":
			return "dc:" + local
		}
		return ""
	}

	if strings.EqualFold(entry.Name.Local, "meta") {
		property := strings.ToLower(strings.TrimSpace(attrValue(entry.Attrs, "property")))
		if property == "dcterms:modified" || strings.HasPrefix(property, "rendition:") {
			return property
		}
		if strings.EqualFold(strings.TrimSpace(attrValue(entry.Attrs, "name")), "cover") {
			return "cover"
		}
	}
	return ""
}

// epubMetadataEntry is a metadata element in the package document
type epubMetadataEntry struct {
	Name  xml.Name
	Attrs []xml.Attr
	Raw   string // the element exactly as it appears in the input file
}

func (e epubMetadataEntry) Text() string {
	var value struct {
		Text string `xml:",chardata"`
	}
	if err := unmarshalXml([]byte(e.Raw), &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value.Text)
}

// readOpfMetadataEntries reads the metadata elements of the package document
// as they appear in the input file, together with the xml namespaces they use.
//
// The elements are sliced out of the source instead of being marshalled back,
// so that their namespace prefixes are kept exactly as the input file has
// them.
func readOpfMetadataEntries(opfSrc []byte) ([]xml.Attr, []epubMetadataEntry, error) {
	decoder := newXmlDecoder(bytes.NewReader(opfSrc))

	nsAttrs := make([]xml.Attr, 0, 4)
	entries := make([]epubMetadataEntry, 0, 16)

	depth := 0
	metadataDepth := -1
	var current *epubMetadataEntry
	var currentStart int64

	for {
		// the offset before the token is where the token starts
		tokenStart := decoder.InputOffset()
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nsAttrs, entries, err
		}
		tokenEnd := decoder.InputOffset()

		switch t := token.(type) {
		case xml.StartElement:
			local := strings.ToLower(t.Name.Local)
			if local == "package" || local == "metadata" {
				nsAttrs = append(nsAttrs, t.Attr...)
			}
			if metadataDepth < 0 && local == "metadata" {
				metadataDepth = depth + 1
			} else if depth == metadataDepth && current == nil {
				// a direct child of the metadata element
				current = &epubMetadataEntry{Name: t.Name, Attrs: t.Attr}
				currentStart = tokenStart
			}
			depth++
		case xml.EndElement:
			depth--
			if current != nil && depth == metadataDepth {
				current.Raw = strings.TrimSpace(string(opfSrc[currentStart:tokenEnd]))
				entries = append(entries, *current)
				current = nil
			} else if metadataDepth >= 0 && depth == metadataDepth-1 {
				// the end of the metadata element
				metadataDepth = -1
			}
		}
	}
	return nsAttrs, entries, nil
}

// formatNamespaces builds the xml namespace declarations for the metadata
// element of the output file. The declarations of the input file are kept so
// that its metadata entries still resolve.
func formatNamespaces(attrs []xml.Attr) string {
	namespaces := map[string]string{
		"dc":  dcNamespace,
		"opf": opfNamespace,
	}
	prefixes := []string{"dc", "opf"}

	for _, attr := range attrs {
		// the default namespace(xmlns=) is declared by the package element
		if attr.Name.Space != "xmlns" || attr.Name.Local == "" {
			continue
		}
		if _, found := namespaces[attr.Name.Local]; found {
			continue
		}
		namespaces[attr.Name.Local] = attr.Value
		prefixes = append(prefixes, attr.Name.Local)
	}

	decls := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		decls = append(decls, fmt.Sprintf("xmlns:%s=%q", prefix, namespaces[prefix]))
	}
	return strings.Join(decls, " ")
}

func isDCElement(name xml.Name) bool {
	// the prefix is used as-is when the input file does not declare it
	return name.Space == dcNamespace || strings.EqualFold(name.Space, "dc")
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

	src, err := readZipFile(f)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", epubContainerPath, err)
	}

	var container epubContainer
	if err := unmarshalXml(src, &container); err != nil {
		return "", fmt.Errorf("parsing %s: %w", epubContainerPath, err)
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

// readEncryptedPaths reads the paths of the encrypted files inside the epub zip
// archive. The obfuscated fonts are left out because they are not DRM and no
// page depends on them. It returns nothing if the input file is not encrypted.
func readEncryptedPaths(files map[string]*zip.File, logger log.Logger) map[string]bool {
	file, found := files[epubEncryptionPath]
	if !found {
		return nil
	}

	src, err := readZipFile(file)
	if err != nil {
		logger.Warn("Cannot read the encryption information of the input epub file -", "path", epubEncryptionPath, "err", err)
		return nil
	}

	var enc epubEncryption
	if err := unmarshalXml(src, &enc); err != nil {
		logger.Warn("Cannot parse the encryption information of the input epub file -", "path", epubEncryptionPath, "err", err)
		return nil
	}

	encrypted := make(map[string]bool, len(enc.Data))
	for _, data := range enc.Data {
		if isFontObfuscation(data.Method.Algorithm) {
			continue
		}
		for _, ref := range data.References {
			if strings.TrimSpace(ref.URI) == "" {
				continue
			}
			// the uri is relative to the root of the zip archive
			encrypted[resolveHref("", ref.URI)] = true
		}
	}
	if len(encrypted) > 0 {
		logger.Debug("The input epub file is encrypted -", "total_encrypted_files", len(encrypted))
	}
	return encrypted
}

// isFontObfuscation checks if the encryption algorithm only scrambles an
// embedded font file
func isFontObfuscation(algorithm string) bool {
	algorithm = strings.TrimSpace(algorithm)
	for _, obfuscation := range fontObfuscationAlgorithms {
		if strings.EqualFold(algorithm, obfuscation) {
			return true
		}
	}
	return false
}

// resolveEpubPages resolves each spine item into an image page in reading
// order. A spine item without an image or with an encrypted(DRM-protected)
// image stops the formatting unless the page is configured to be skipped. The
// volume must have at least one image page.
func resolveEpubPages(files map[string]*zip.File, opfPath string, pkg epubPackage, encrypted map[string]bool, skipUnreadable bool, logger log.Logger) ([]epubPage, []epubSpineDoc, SkippedPages, error) {
	items := make(map[string]epubManifestItem, len(pkg.Manifest.Items))
	for _, item := range pkg.Manifest.Items {
		items[item.Id] = item
	}

	var skipped SkippedPages
	pages := make([]epubPage, 0, len(pkg.Spine.ItemRefs))
	spine := make([]epubSpineDoc, 0, len(pkg.Spine.ItemRefs))

	// skip either stops the formatting or records the page which cannot be
	// read, depending on the configuration
	skip := func(page string, reason SkipReason) error {
		pageNo := len(spine) + 1
		if !skipUnreadable {
			return fmt.Errorf("page %d(%s) in the input epub file cannot be read because %s. Use --skip-unreadable-page to skip the page and format the rest", pageNo, page, reason)
		}
		logger.Warn("Skipping the page which cannot be read -", "page_no", pageNo, "page", page, "reason", reason)
		skipped = append(skipped, SkippedPage{PageNo: pageNo, Reason: reason})
		return nil
	}

	for _, ref := range pkg.Spine.ItemRefs {
		item, found := items[ref.IdRef]
		if !found {
			logger.Warn("The spine item is not found in the manifest -", "idref", ref.IdRef)
			if err := skip(ref.IdRef, SkipNoImage); err != nil {
				return nil, nil, skipped, err
			}
			continue
		}
		if isNavItem(item) {
			// the table of content document is not a page
			continue
		}

		itemPath := resolveHref(opfPath, item.Href)
		mediaType := strings.ToLower(strings.TrimSpace(item.MediaType))

		var page *epubPage
		reason := NotSkipped
		switch {
		case encrypted[itemPath]:
			reason = SkipEncrypted
		case strings.HasPrefix(mediaType, "image/"):
			// the spine item is the image itself
			file, found := files[itemPath]
			if !found {
				logger.Warn("The image file is not found in the input epub file -", "page", item.Href)
				reason = SkipNoImage
				break
			}
			page = &epubPage{zipPath: path.Clean(file.Name), mediaType: mediaType}
		case isDocumentMediaType(mediaType):
			var resolved epubPage
			resolved, reason = resolveEpubPage(files, itemPath, encrypted, logger)
			if reason == NotSkipped {
				page = &resolved
			}
		default:
			logger.Warn("The spine item is neither an image nor a document -", "page", item.Href, "media_type", item.MediaType)
			reason = SkipNoImage
		}
		if reason != NotSkipped {
			if err := skip(item.Href, reason); err != nil {
				return nil, nil, skipped, err
			}
		}

		doc := epubSpineDoc{path: itemPath}
		if page != nil {
			pages = append(pages, *page)
			doc.pageNo = len(pages)
		}
		spine = append(spine, doc)
	}

	if len(pages) == 0 {
		if skipped.Count(SkipEncrypted) > 0 {
			return nil, nil, skipped, fmt.Errorf("input epub file is encrypted(DRM-protected) so its pages cannot be read. Only DRM-free epub file is supported")
		}
		return nil, nil, skipped, fmt.Errorf("input epub file has no image page. Only manga epub file(a volume of image pages) is supported")
	}
	return pages, spine, skipped, nil
}

// resolveEpubPage resolves a spine document into the image page it shows. If
// the document shows more than one image, the largest image is used.
func resolveEpubPage(files map[string]*zip.File, docPath string, encrypted map[string]bool, logger log.Logger) (epubPage, SkipReason) {
	doc, found := files[docPath]
	if !found {
		logger.Warn("The page document is not found in the input epub file -", "page", docPath)
		return epubPage{}, SkipNoImage
	}

	refs, err := findImageRefs(doc)
	if err != nil {
		// best effort. the images found before the error are still used
		logger.Debug("Error while parsing the page document -", "page", docPath, "err", err)
	}

	hasEncryptedImage := false
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

		if encrypted[imgPath] {
			logger.Debug("Ignoring encrypted(DRM-protected) image in the page document -", "page", docPath, "image", imgPath)
			hasEncryptedImage = true
			continue
		}

		file, found := files[imgPath]
		if !found {
			logger.Debug("Ignoring image which is not found in the input epub file -", "page", docPath, "image", imgPath)
			continue
		}
		candidates = append(candidates, file)
	}

	if len(candidates) == 0 {
		if hasEncryptedImage {
			logger.Warn("The image of the page in the input epub file is encrypted(DRM-protected) -", "page", docPath)
			return epubPage{}, SkipEncrypted
		}
		logger.Warn("The page in the input epub file has no image -", "page", docPath)
		return epubPage{}, SkipNoImage
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
	}, NotSkipped
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
			ref = attrValue(start.Attr, "src")
		case "image": // svg image
			ref = attrValue(start.Attr, "href")
		case "object":
			ref = attrValue(start.Attr, "data")
		}
		if strings.TrimSpace(ref) != "" {
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, attr := range attrs {
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

// readEpubToc reads the table of content of the input file. The epub3
// navigation document is preferred over the epub2 ncx document. Each entry
// points to the page number in the input file it lands on.
func readEpubToc(files map[string]*zip.File, opfPath string, pkg epubPackage, spine []epubSpineDoc, encrypted map[string]bool, logger log.Logger) []format.EpubTocEntry {
	// the page an entry lands on. an entry pointing to a page which is not an
	// image page moves forward to the next image page
	pageOf := func(basePath string, href string) int {
		if strings.TrimSpace(href) == "" || isRemoteHref(href) {
			return 0
		}
		target := resolveHref(basePath, href)
		for i, doc := range spine {
			if doc.path != target {
				continue
			}
			for _, next := range spine[i:] {
				if next.pageNo > 0 {
					return next.pageNo
				}
			}
			return 0
		}
		return 0
	}

	// the epub3 navigation document is preferred over the epub2 ncx document
	for _, wantNcx := range []bool{false, true} {
		for _, item := range pkg.Manifest.Items {
			isNcx := strings.EqualFold(strings.TrimSpace(item.MediaType), ncxMediaType)
			if !isNavItem(item) || isNcx != wantNcx {
				continue
			}

			tocPath := resolveHref(opfPath, item.Href)
			if encrypted[tocPath] {
				logger.Debug("Ignoring the encrypted(DRM-protected) table of content of the input epub file -", "path", tocPath)
				continue
			}
			file, found := files[tocPath]
			if !found {
				continue
			}
			src, err := readZipFile(file)
			if err != nil {
				logger.Debug("Cannot read the table of content of the input epub file -", "path", tocPath, "err", err)
				continue
			}

			var root xmlNode
			if err := unmarshalXml(src, &root); err != nil {
				logger.Debug("Cannot parse the table of content of the input epub file -", "path", tocPath, "err", err)
				continue
			}

			var toc []format.EpubTocEntry
			if isNcx {
				toc = readNcxToc(root, tocPath, pageOf)
			} else {
				toc = readNavToc(root, tocPath, pageOf)
			}
			if len(toc) > 0 {
				logger.Debug("Found the table of content in the input epub file -", "path", tocPath, "entries", len(toc))
				return toc
			}
		}
	}
	return nil
}

// readNavToc reads the table of content from an epub3 navigation document
func readNavToc(root xmlNode, navPath string, pageOf func(string, string) int) []format.EpubTocEntry {
	nav := root.findDescendant(func(n xmlNode) bool {
		return strings.EqualFold(n.XMLName.Local, "nav") &&
			strings.EqualFold(strings.TrimSpace(attrValue(n.Attrs, "type")), "toc")
	})
	if nav == nil {
		return nil
	}
	list := nav.findChild("ol")
	if list == nil {
		return nil
	}
	return readNavList(*list, navPath, pageOf)
}

func readNavList(list xmlNode, navPath string, pageOf func(string, string) int) []format.EpubTocEntry {
	entries := make([]format.EpubTocEntry, 0, len(list.Children))
	for _, item := range list.findChildren("li") {
		var entry format.EpubTocEntry
		if link := item.findChild("a"); link != nil {
			entry.Label = link.TextContent()
			entry.PageNo = pageOf(navPath, attrValue(link.Attrs, "href"))
		} else if span := item.findChild("span"); span != nil {
			// an entry without a link
			entry.Label = span.TextContent()
		}
		if sublist := item.findChild("ol"); sublist != nil {
			entry.Children = readNavList(*sublist, navPath, pageOf)
		}
		if entry.Label == "" && len(entry.Children) == 0 {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// readNcxToc reads the table of content from an epub2 ncx document
func readNcxToc(root xmlNode, ncxPath string, pageOf func(string, string) int) []format.EpubTocEntry {
	navMap := root.findDescendant(func(n xmlNode) bool {
		return strings.EqualFold(n.XMLName.Local, "navMap")
	})
	if navMap == nil {
		return nil
	}
	return readNcxNavPoints(*navMap, ncxPath, pageOf)
}

func readNcxNavPoints(parent xmlNode, ncxPath string, pageOf func(string, string) int) []format.EpubTocEntry {
	points := parent.findChildren("navPoint")
	entries := make([]format.EpubTocEntry, 0, len(points))
	for _, point := range points {
		var entry format.EpubTocEntry
		if label := point.findChild("navLabel"); label != nil {
			if text := label.findChild("text"); text != nil {
				entry.Label = text.TextContent()
			}
		}
		if content := point.findChild("content"); content != nil {
			entry.PageNo = pageOf(ncxPath, attrValue(content.Attrs, "src"))
		}
		entry.Children = readNcxNavPoints(point, ncxPath, pageOf)
		if entry.Label == "" && len(entry.Children) == 0 {
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

// xmlNode is a generic xml element. It is used to walk the documents whose
// structure differs between epub files.
type xmlNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Text     string     `xml:",chardata"`
	Children []xmlNode  `xml:",any"`
}

func (n xmlNode) findChild(local string) *xmlNode {
	for i := range n.Children {
		if strings.EqualFold(n.Children[i].XMLName.Local, local) {
			return &n.Children[i]
		}
	}
	return nil
}

func (n xmlNode) findChildren(local string) []xmlNode {
	children := make([]xmlNode, 0, len(n.Children))
	for _, child := range n.Children {
		if strings.EqualFold(child.XMLName.Local, local) {
			children = append(children, child)
		}
	}
	return children
}

func (n xmlNode) findDescendant(match func(xmlNode) bool) *xmlNode {
	for i := range n.Children {
		if match(n.Children[i]) {
			return &n.Children[i]
		}
		if found := n.Children[i].findDescendant(match); found != nil {
			return found
		}
	}
	return nil
}

// TextContent is the text of the element and of all its children
func (n xmlNode) TextContent() string {
	var sb strings.Builder
	sb.WriteString(n.Text)
	for _, child := range n.Children {
		sb.WriteString(" ")
		sb.WriteString(child.TextContent())
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("opening %s in the input epub file: %w", f.Name, err)
	}
	defer rc.Close()

	src, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("reading %s in the input epub file: %w", f.Name, err)
	}
	return src, nil
}

func unmarshalXml(src []byte, v any) error {
	return newXmlDecoder(bytes.NewReader(src)).Decode(v)
}
