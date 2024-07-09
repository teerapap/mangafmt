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
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/teerapap/mangafmt/internal/book"
	"github.com/teerapap/mangafmt/internal/imgutil"
	"github.com/teerapap/mangafmt/internal/log"
	"github.com/teerapap/mangafmt/internal/util"
)

func SaveAsEPUB(theBook *book.Book, pages []Page, outFile string) error {
	return save("EPUB", theBook, pages, outFile)
}

func SaveAsKEPUB(theBook *book.Book, pages []Page, outFile string) error {
	return save("KEPUB", theBook, pages, outFile)
}

func save(format string, theBook *book.Book, pages []Page, outFile string) error {
	defer log.SetIndentLevel(log.IndentLevel()) // reset indent level after return

	log.Printf("Start packaging in %s format to %s", format, outFile)
	log.Indent()

	// create epub structure
	epub, err := createEpub(theBook, pages)
	if err != nil {
		return fmt.Errorf("creating epub: %w", err)
	}

	// write epub stucture to file
	err = writeEpub(epub, outFile)
	if err != nil {
		return fmt.Errorf("creating epub file: %w", err)
	}

	return nil
}

type EpubBook struct {
	BookID           string // urn:uuid:....
	Language         string
	Title            string
	TotalPageCount   int
	IsRTL            bool
	Contributor      string
	Creator          string
	ModifiedDatetime string
	Cover            EpubPageItem
	Pages            []EpubPage
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
	Id        string
	Url       string
	MediaType string
}

func createEpub(theBook *book.Book, pages []Page) (EpubBook, error) {
	epub := EpubBook{}

	uuidstr, err := uuid.GenerateUUID()
	if err != nil {
		return EpubBook{}, fmt.Errorf("generating epub uuid: %w", err)
	}

	epub.BookID = fmt.Sprintf("urn:uuid:%s", uuidstr)
	epub.Language = "en-US"
	epub.Title = html.EscapeString(theBook.Title)
	epub.TotalPageCount = len(pages)
	epub.IsRTL = theBook.Config.IsRTL
	appVersion := fmt.Sprintf("mangafmt-%s", util.AppVersion)
	epub.Contributor = appVersion
	epub.Creator = appVersion
	epub.ModifiedDatetime = time.Now().Format(time.RFC3339)

	epub.Pages = make([]EpubPage, 0, len(pages))
	for i, page := range pages {
		if i == 0 {
			epub.Cover = EpubPageItem{
				Id:        "cover",
				Url:       fmt.Sprintf("Images/%s", filepath.Base(page.Filepath)),
				MediaType: page.MediaType,
			}
		}

		epubPage := EpubPage{}
		epubPage.Title = page.Id
		epubPage.BgColor = imgutil.ToHexString(theBook.Config.BgColor)
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
	}
	return epub, nil
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

func writeEpub(epub EpubBook, outFile string) error {

	zipFile, err := os.Create(outFile)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	log.Print("Writing metadata files...")
	err = util.WriteFileToZip(w, "mimetype", mimetypeTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}
	err = util.WriteFileToZip(w, "META-INF/container.xml", containerTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}
	err = util.WriteFileToZip(w, "OEBPS/toc.ncx", tocTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}
	err = util.WriteFileToZip(w, "OEBPS/content.opf", contentTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}
	err = util.WriteFileToZip(w, "OEBPS/nav.xhtml", navTmpl, epub)
	if err != nil {
		return fmt.Errorf("writing metadata to the output file: %w", err)
	}

	for i, page := range epub.Pages {
		log.Printf("Packaging page....(%d/%d)", i+1, epub.TotalPageCount)
		log.Indent()

		err := util.WriteFileToZip(w, fmt.Sprintf("OEBPS/%s", page.Xhtml.Url), pageTmpl, page)
		if err != nil {
			return fmt.Errorf("writing page(%d) file to the output file: %w", i+1, err)
		}
		err = util.CopyFileToZip(w, fmt.Sprintf("OEBPS/%s", page.Image.Url), page.SrcFile)
		if err != nil {
			return fmt.Errorf("copying page(%d) image file to the output file: %w", i+1, err)
		}

		log.Unindent()
	}
	log.Unindent()
	log.Printf("Done packaging.")
	return nil
}
