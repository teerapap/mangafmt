//
// epub.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package format

import (
	"archive/zip"
	_ "embed"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/util"
)

// defaultEpubNamespaces is used when the input file has no metadata namespace
// declaration to keep
const defaultEpubNamespaces = `xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf"`

// EpubMetadata is the metadata of an epub volume. It is read from an epub
// input file and it is only used when the output format is epub/kepub.
type EpubMetadata struct {
	// TableOfContents is the table of content of the volume
	TableOfContents []EpubTocEntry
	// Namespaces is the xml namespace declarations used by Entries
	Namespaces string
	// Prefix is the vocabulary prefix declaration used by the properties in
	// Entries. It is empty when the input file declares none.
	Prefix string
	// Entries is the metadata elements of the volume in their original order
	Entries []EpubMetadataEntry
}

// EpubTocEntry is an entry in the table of content of an epub volume
type EpubTocEntry struct {
	Label string
	// PageNo is the page number(1-based) of the volume the entry points to. It
	// is zero when the entry points to no page of the volume.
	PageNo   int
	Children []EpubTocEntry
}

// EpubMetadataEntry is one metadata element of an epub volume
type EpubMetadataEntry struct {
	// Name is the name of the metadata element(ex. dc:title) when the element
	// is regenerated in the output file. The regenerated element takes the
	// place of this entry so that the metadata stays in the order of the input
	// file.
	Name string
	// Raw is the element as it appears in the input file. It is empty when
	// Name is set.
	Raw string
}

func SaveAsEPUB(volume Volume, outFile string, logger log.Logger, progress PackagingProgressFunc) error {
	return save("EPUB", volume, outFile, logger, progress)
}

func SaveAsKEPUB(volume Volume, outFile string, logger log.Logger, progress PackagingProgressFunc) error {
	return save("KEPUB", volume, outFile, logger, progress)
}

func save(format string, volume Volume, outFile string, logger log.Logger, progress PackagingProgressFunc) error {
	logger = logger.Indent("> Package > " + format + " ")
	logger.Info("Start packaging", "file", outFile)

	// create epub structure
	epub, err := createEpub(volume, func(completed float64) {
		// 20%
		progress(completed * 0.2)
	})
	if err != nil {
		return fmt.Errorf("creating epub: %w", err)
	}

	// write epub stucture to file
	err = writeEpub(epub, outFile, logger, func(completed float64) {
		// 80%
		progress(0.2 + completed*0.8)
	})
	if err != nil {
		return fmt.Errorf("creating epub file: %w", err)
	}

	return nil
}

type EpubVolume struct {
	VolumeID         string // urn:uuid:....
	Language         string
	Title            string
	TotalPageCount   int
	IsRTL            bool
	Contributor      string
	Creator          string
	ModifiedDatetime string
	Namespaces       string   // xml namespace declarations of the metadata element
	Prefix           string   // vocabulary prefix declarations of the package element
	MetadataEntries  []string // metadata entries kept from the input file
	Cover            EpubPageItem
	Pages            []EpubPage

	TableOfContents      []EpubTocItem
	TableOfContentsDepth int
}

// EpubTocItem is an entry in the table of content of the output file
type EpubTocItem struct {
	Id        string
	PlayOrder int
	Label     string
	Url       string
	Children  []EpubTocItem
}

type EpubPage struct {
	Title   string
	BgColor string
	Width   uint
	Height  uint
	SrcFile string
	Xhtml   EpubPageItem
	Image   EpubPageItem
}

type EpubPageItem struct {
	Id         string
	Properties string
	Url        string
	MediaType  string
}

func createEpub(volume Volume, progress PackagingProgressFunc) (EpubVolume, error) {
	epub := EpubVolume{}

	// keep the identifier of the input file so that the output file is still
	// the same volume
	epub.VolumeID = strings.TrimSpace(volume.Identifier)
	if epub.VolumeID == "" {
		uuidstr, err := uuid.GenerateUUID()
		if err != nil {
			return EpubVolume{}, fmt.Errorf("generating epub uuid: %w", err)
		}
		epub.VolumeID = fmt.Sprintf("urn:uuid:%s", uuidstr)
	}
	epub.VolumeID = html.EscapeString(epub.VolumeID)

	epub.Language = html.EscapeString(strings.TrimSpace(volume.Language))
	if epub.Language == "" {
		epub.Language = "en-US"
	}
	epub.Title = html.EscapeString(volume.Title)
	epub.TotalPageCount = len(volume.Pages)
	epub.IsRTL = volume.IsRTL
	epub.Contributor = fmt.Sprintf("mangafmt-%s", util.AppVersion)
	if volume.Author == "" {
		epub.Creator = "Anonymous"
	} else {
		epub.Creator = html.EscapeString(volume.Author)
	}
	epub.Namespaces = strings.TrimSpace(volume.Epub.Namespaces)
	if epub.Namespaces == "" {
		epub.Namespaces = defaultEpubNamespaces
	}
	// the kept metadata entries use the vocabularies of the input file
	epub.Prefix = html.EscapeString(strings.TrimSpace(volume.Epub.Prefix))
	epub.ModifiedDatetime = time.Now().Format(time.RFC3339)

	pageCount := len(volume.Pages)
	epub.Pages = make([]EpubPage, 0, pageCount)
	progress(0.0)
	for i, page := range volume.Pages {
		if i == 0 {
			epub.Cover = EpubPageItem{
				Id:        "cover",
				Url:       fmt.Sprintf("Images/%s", filepath.Base(page.Filepath)),
				MediaType: page.MediaType,
			}
		}

		epubPage := EpubPage{}
		epubPage.Title = page.Id
		epubPage.BgColor = "#FFFFFF"
		epubPage.Width = page.Size.Width
		epubPage.Height = page.Size.Height
		epubPage.SrcFile = page.Filepath

		epubPage.Xhtml = EpubPageItem{}
		epubPage.Xhtml.Id = fmt.Sprintf("xhtml_%s", page.Id)
		epubPage.Xhtml.Url = fmt.Sprintf("Text/%s.xhtml", page.Id)
		epubPage.Xhtml.MediaType = "application/xhtml+xml"

		epubPage.Image = EpubPageItem{}
		epubPage.Image.Id = fmt.Sprintf("img_%s", page.Id)
		epubPage.Image.Url = fmt.Sprintf("Images/%s", filepath.Base(page.Filepath))
		epubPage.Image.MediaType = page.MediaType

		epub.Pages = append(epub.Pages, epubPage)
		progress(float64(i+1) / float64(pageCount))
	}

	// the cover and the page urls are only known after the pages are created
	epub.MetadataEntries = buildMetadataEntries(epub, volume.Epub.Entries)
	epub.TableOfContents, epub.TableOfContentsDepth = buildTableOfContents(volume.Epub.TableOfContents, epub)

	return epub, nil
}

// buildTableOfContents builds the table of content of the output file from the
// one kept from the input file. The volume title pointing at the first page is
// used when the input file has no table of content.
func buildTableOfContents(entries []EpubTocEntry, epub EpubVolume) ([]EpubTocItem, int) {
	if len(epub.Pages) == 0 {
		return nil, 0
	}

	playOrder := 0
	var build func(entries []EpubTocEntry, depth int) ([]EpubTocItem, int)
	build = func(entries []EpubTocEntry, depth int) ([]EpubTocItem, int) {
		items := make([]EpubTocItem, 0, len(entries))
		maxDepth := depth - 1
		for _, entry := range entries {
			if entry.PageNo < 1 || entry.PageNo > len(epub.Pages) {
				continue
			}
			playOrder++
			order := playOrder // the children take the numbers after this one

			children, childrenDepth := build(entry.Children, depth+1)
			items = append(items, EpubTocItem{
				Id:        fmt.Sprintf("toc-%d", order),
				PlayOrder: order,
				Label:     html.EscapeString(entry.Label),
				Url:       epub.Pages[entry.PageNo-1].Xhtml.Url,
				Children:  children,
			})
			maxDepth = max(maxDepth, depth, childrenDepth)
		}
		return items, maxDepth
	}

	items, depth := build(entries, 1)
	if len(items) == 0 {
		// the input file has no table of content
		items = []EpubTocItem{{
			Id:        "toc-1",
			PlayOrder: 1,
			Label:     epub.Title, // already escaped
			Url:       epub.Pages[0].Xhtml.Url,
		}}
		depth = 1
	}
	return items, depth
}

// buildMetadataEntries builds the metadata elements of the output file. Each
// regenerated element takes the place of the same element of the input file so
// that the metadata stays in the order of the input file. The elements which
// the input file does not have are written after them.
func buildMetadataEntries(epub EpubVolume, inputEntries []EpubMetadataEntry) []string {
	// in the order they are written when the input file has none of them
	regenerated := []EpubMetadataEntry{
		{Name: "dc:title", Raw: fmt.Sprintf("<dc:title>%s</dc:title>", epub.Title)},
		{Name: "dc:language", Raw: fmt.Sprintf("<dc:language>%s</dc:language>", epub.Language)},
		{Name: "dc:identifier", Raw: fmt.Sprintf(`<dc:identifier id="VolumeID">%s</dc:identifier>`, epub.VolumeID)},
		{Name: "dc:contributor", Raw: fmt.Sprintf(`<dc:contributor id="contributor">%s</dc:contributor>`, epub.Contributor)},
		{Name: "dc:creator", Raw: fmt.Sprintf("<dc:creator>%s</dc:creator>", epub.Creator)},
		{Name: "dcterms:modified", Raw: fmt.Sprintf(`<meta property="dcterms:modified">%s</meta>`, epub.ModifiedDatetime)},
		{Name: "cover", Raw: fmt.Sprintf(`<meta name="%s" content="cover"/>`, epub.Cover.Id)},
		{Name: "rendition:orientation", Raw: `<meta property="rendition:orientation">auto</meta>`},
		{Name: "rendition:spread", Raw: `<meta property="rendition:spread">auto</meta>`},
		{Name: "rendition:layout", Raw: `<meta property="rendition:layout">pre-paginated</meta>`},
	}
	byName := make(map[string]string, len(regenerated))
	for _, entry := range regenerated {
		byName[entry.Name] = entry.Raw
	}

	entries := make([]string, 0, len(inputEntries)+len(regenerated))
	written := make(map[string]bool, len(regenerated))
	for _, entry := range inputEntries {
		if entry.Name == "" {
			// kept from the input file as-is
			entries = append(entries, entry.Raw)
			continue
		}
		raw, found := byName[entry.Name]
		if !found || written[entry.Name] {
			continue
		}
		written[entry.Name] = true
		entries = append(entries, raw)
	}

	// the elements which the input file does not have
	for _, entry := range regenerated {
		if !written[entry.Name] {
			entries = append(entries, entry.Raw)
		}
	}
	return entries
}

//go:embed templates/epub/mimetype
var mimetypeTmplStr string
var mimetypeTmpl = util.CreateTemplate("epub/mimetype", mimetypeTmplStr)

//go:embed templates/epub/META-INF/container.xml
var containerTmplStr string
var containerTmpl = util.CreateTemplate("epub/META-INF/container.xml", containerTmplStr)

//go:embed templates/epub/OEBPS/toc.ncx
var tocTmplStr string
var tocTmpl = util.CreateTemplate("epub/OEBPS/toc.ncx", tocTmplStr)

//go:embed templates/epub/OEBPS/content.opf
var contentTmplStr string
var contentTmpl = util.CreateTemplate("epub/OEBPS/content.opf", contentTmplStr)

//go:embed templates/epub/OEBPS/nav.xhtml
var navTmplStr string
var navTmpl = util.CreateTemplate("epub/OEBPS/nav.xhtml", navTmplStr)

//go:embed templates/epub/OEBPS/Text/style.css
var styleTmplStr string
var styleTmpl = util.CreateTemplate("epub/OEBPS/Text/style.css", styleTmplStr)

//go:embed templates/epub/OEBPS/Text/page.xhtml
var pageTmplStr string
var pageTmpl = util.CreateTemplate("epub/OEBPS/Text/page.xhtml", pageTmplStr)

func writeEpub(epub EpubVolume, outFile string, logger log.Logger, progress PackagingProgressFunc) error {

	zipFile, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	totalProgress := float64(6 + len(epub.Pages)*2)
	progress(0)

	logger.Info("Writing metadata files...")
	err = util.WriteFileToZip(w, "mimetype", mimetypeTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}
	progress(1.0 / totalProgress)
	err = util.WriteFileToZip(w, "META-INF/container.xml", containerTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}
	progress(2.0 / totalProgress)
	err = util.WriteFileToZip(w, "OEBPS/toc.ncx", tocTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}
	progress(3.0 / totalProgress)
	err = util.WriteFileToZip(w, "OEBPS/content.opf", contentTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}
	progress(4.0 / totalProgress)
	err = util.WriteFileToZip(w, "OEBPS/nav.xhtml", navTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}
	progress(5.0 / totalProgress)
	err = util.WriteFileToZip(w, "OEBPS/Text/style.css", styleTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}
	progress(6.0 / totalProgress)

	for i, page := range epub.Pages {
		logger.Infof("Packaging page....(%d/%d)", i+1, epub.TotalPageCount)

		err := util.WriteFileToZip(w, fmt.Sprintf("OEBPS/%s", page.Xhtml.Url), pageTmpl, page)
		if err != nil {
			return fmt.Errorf("writing page(%d) file to the output file: %w", i+1, err)
		}
		progress(float64(i*2+1+6) / totalProgress)
		err = util.CopyFileToZip(w, fmt.Sprintf("OEBPS/%s", page.Image.Url), page.SrcFile)
		if err != nil {
			return fmt.Errorf("copying page(%d) image file to the output file: %w", i+1, err)
		}
		progress(float64(i*2+2+6) / totalProgress)
	}
	logger.Info("Done packaging")
	return nil
}
