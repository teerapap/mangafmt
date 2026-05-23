# mangafmt

**mangafmt** is a Go-based command-line program to format a manga file and optimized for reading on E-Ink reader (and possibly other devices). 

## Features

* Detect double-page spread (a big scene that covers two facing pages) heuristically and connect them into one landscape page. 
* Trim blank spaces around the edges for better.
* Resize/rotate page to fit specific screen size.
* Reduce file size by reducing colors to grayscale (except the cover page or configured otherwise).
* Handle right-to-left (RTL) read direction.
* Convert to EPUB/KEPUB/CBZ format.
* Support Windows/OSX/Linux

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
  -author string
        Book author. This affects epub/kepub output. Unspecified or blank means 'Anonymous'
  -density float
        Output density (DPI) (default 300)
  -format value
        Output file format. The supported formats
                - raw (default)
                - cbz
                - epub
                - kepub
  -fuzz float
        Color fuzz (percentage)[0.0-1.0] (default 0.1)
  -grayscale string
        Page range (Ex. '4-10, 15, 39-') to convert to grayscale. Default is all pages except the first page(cover). 'false' means no grayscale conversion (default "2-")
  -grayscale-depth uint
        Grayscale color depth in number of bits. Possible values are 1, 2, 4, 8, 16 bits. No upscale if source image is in lower depth. (default 4)
  -h    Show help
  -height uint
        Output screen heigt (pixel) (default 1680)
  -help
        Show help
  -output string
        Output file. Unspecified or blank means using the same file name as input file
  -pages string
        Page range (Ex. '4-10, 15, 39-'). Default is all pages. Open right range means to the end. (default "1-")
  -right-to-left
        Right-to-left read direction (ex. Japanese manga)
  -rtl
        Right-to-left read direction (ex. Japanese manga)
  -spread
        Enable double-page spread detection and connection (default true)
  -spread-confidence float
        Confidence threshold for double-page spread detection. The higher the value, the stricter the criteria become. (percentage)[0.0-1.0] (default 0.55)
  -spread-edge uint
        Edge width for double-page spread detection (pixel) (default 30)
  -spread-keep-orientation
        Keep the page original orientation. Do not rotate to maximize screen area
  -spread-keep-original
        Keep the original left and right page
  -title string
        Book title. This affects epub/kepub output. Unspecified or blank means using filename without extension
  -trim
        Enable trim edge (default true)
  -trim-margin int
        Safety trim margin (pixel) (default 10)
  -trim-min-size float
        Minimum size after trimmed (percentage)[0.0-1.0] (default 0.85)
  -v    Verbose output
  -verbose
        Verbose output
  -version
        Show version
  -width uint
        Output screen width (pixel) (default 1264)
  -work-dir string
        Work directory path. Unspecified or blank means using system temp path
```

## Install

There are two ways to install.

### Pre-built binary

Download the pre-built binary from [Releases Page](https://github.com/teerapap/mangafmt/releases)

### Using `go install`

```
go install github.com/teerapap/mangafmt@latest
```

## Runtime Dependencies

For PDF input, you need to install one of these options.

* [libvips](https://www.libvips.org/) (**Fastest**)
  * For Windows, you need to install the version with `-all` suffix and configure `PATH` environment variable to see `vips.exe` command.
* [ImageMagick7](https://imagemagick.org/) and [Ghostscript](https://www.ghostscript.com/)
* [ImageMagick6](https://legacy.imagemagick.org/) and [Ghostscript](https://www.ghostscript.com/)
  * Not recommended for Windows because its `convert.exe` may clash with the system `convert.exe`

## Build

The code is pure Go so simply run

```
go build
```

## Future Works

* Support raw images in a directory input
* Support CBZ input format
* Support EPUB/KEPUB input format

## Notes

I developed this tool for my personal use so all default values cater for my own usage.  However, I'd be happy if this tool is useful for other fellow manga readers too. :smile:

I strongly condemn manga piracy and hope you won't use this tool for any illegal purposes. Please remember to always support and respect the hard work of manga artists.

If you found any bugs, please feel free to report an issue and I'll fix it when time permits. 
