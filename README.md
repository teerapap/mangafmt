# mangafmt

**mangafmt** is a Go-based command-line program to format a manga file and optimized for reading on E-Ink reader (and possibly other devices). 

## Features

* Detect double-page spread (a big scene that covers two facing pages) heuristically and connect them into one landscape page. 
* Trim blank spaces around the edges for better.
* Resize/rotate page to fit specific screen size.
* Reduce file size by reducing colors to grayscale (except the cover page or configured otherwise).
* Handle right-to-left (RTL) read direction.
* Convert to EPUB/KEPUB/CBZ format. 

### Supported Formats

* Input File
  * PDF
* Output File
  * EPUB
  * KEPUB
  * CBZ
  * RAW (a directory of image files)

### Command Usage

```
./mangafmt [options] <input_pdf_file>
  -background string
        background color (default "white")
  -density float
        output density (DPI) (default 300)
  -format value
        output file format. The supported formats
                - raw (default)
                - cbz
                - epub
                - kepub
  -fuzz float
        color fuzz (percentage)[0.0-1.0] (default 0.1)
  -grayscale string
        page range (Ex. '4-10, 15, 39-') to convert to grayscale. Default is all pages except the first page(cover). 'false' means no grayscale conversion (default "2-")
  -h    show help
  -height uint
        output screen heigt (pixel) (default 1680)
  -help
        show help
  -output string
        output file. Unspecified or blank means using the same file name as input file
  -pages string
        page range (Ex. '4-10, 15, 39-'). Default is all pages. Open right range means to the end. (default "1-")
  -right-to-left
        right-to-left read direction (ex. Japanese manga)
  -rtl
        right-to-left read direction (ex. Japanese manga)
  -spread
        enable double-page spread detection and connection (default true)
  -spread-bg-distortion float
        a page is considered a single page if the distortion between its edge and background color are less than this threshold (percentage)[0.0-1.0] (default 0.4)
  -spread-edge uint
        edge width for double-page spread detection (pixel) (default 2)
  -spread-lr-distortion float
        two pages are considered double-page spread if the distortion between their edges are less than this threshold (percentage)[0.0-1.0] (default 0.4)
  -spread-margin uint
        safety margin before edge width (pixel) (default 2)
  -title string
        Book title. This affects epub/kepub output. Unspecified or blank means using filename without extension
  -trim
        enable trim edge (default true)
  -trim-margin int
        safety trim margin (pixel) (default 10)
  -trim-min-size float
        minimum size after trimmed (percentage)[0.0-1.0] (default 0.85)
  -v    verbose output
  -verbose
        verbose output
  -version
        show version
  -width uint
        output screen width (pixel) (default 1264)
  -work-dir string
        work directory path. Unspecified or blank means using system temp path
```

## Install

TBA

### Supported Platform and Dependencies

* Linux (Tested on Ubuntu only)
* ImageMagick >= 6.9.1-7

## Build

### Ubuntu

```
sudo apt-get install libmagickwand-dev
go build
```

## Notes

I developed this tool for my personal use so all default values cater for my own usage.  However, I'd be happy if this tool is useful for other fellow manga readers too. :smile:

I strongly condemn manga piracy and hope you won't use this tool for any illegal purposes. Please remember to always support and respect the hard work of manga artists.

If you found any bugs, please feel free to report an issue and I'll fix it when time permits. 
