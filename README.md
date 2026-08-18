# mangafmt

**mangafmt** is a Go-based command-line program to format a manga file and optimized for reading on E-Ink reader (and possibly other devices). 

## Features

* Detect double-page spread (a big scene that covers two facing pages) heuristically and connect them into one landscape page. 
* Trim blank spaces around the edges for better.
* Resize/rotate page to fit specific screen size.
* Reduce file size by reducing colors to grayscale (except the cover page or configured otherwise).
* Handle right-to-left (RTL) read direction.
* Convert to EPUB/KEPUB/CBZ format.
* Keep the metadata and the table of contents of the input file (EPUB input to EPUB/KEPUB output).
* Support Windows/OSX/Linux

### Supported Formats

* Input File
  * PDF
  * EPUB/KEPUB (implemented with the help from Claude Code)
    * A DRM-protected file is not supported because its pages cannot be read.
* Output File
  * EPUB
  * KEPUB
  * CBZ
  * RAW (a directory of image files)

### Command Usage

```
mangafmt [options] <input_file> [input_file2...]

Options:
  --author string
	Volume author. This affects epub/kepub output. Unspecified or blank means using the author in the input file or 'Anonymous'

  --convert-only
	Convert from input to output format only without any modification to the pages at all

  --density float (default: 300)
	Output density (DPI). This affects pdf input only

  --format string (default: "raw")
	Output file format. The supported formats
	- raw
	- cbz
	- epub
	- kepub

  --fuzz float (default: 0.1)
	Color fuzz (percentage)[0.0-1.0]

  --grayscale string (default: "2-")
	Page range (Ex. '4-10, 15, 39-') to convert to grayscale. Default is all pages except the first page(cover). 'false' means no grayscale conversion

  --grayscale-depth uint (default: 4)
	Grayscale color depth in number of bits. Possible values are 1, 2, 4, 8, 16 bits. No upscale if source image is in lower depth.

  -h, --help
	Show help

  --height uint (default: 1680)
	Output screen heigt (pixel)

  --log-file
	Print logs to file in addition to console

  --output string
	Output file/directory. Unspecified or blank means using the same file name as input file. For multiple input files, this argument will be output directory

  --pages string (default: "1-")
	Page range (Ex. '4-10, 15, 39-'). Default is all pages. Open right range means to the end.

  --parallel int (default: 2)
	Control the number of concurrent jobs. Zero or negative means unlimit. This is application for multiple input files only

  --progress-bar
	Show progress bar instead of logs (experimental)

  --resize=true|false (default: true)
	Enable/disable resize to aspect fit in output screen size

  --right-to-left, --rtl
	Right-to-left read direction (ex. Japanese manga). Unspecified means using the read direction in the input file if it has one

  --spread=true|false (default: true)
	Enable/disable double-page spread detection and connection

  --spread-confidence float (default: 0.55)
	Confidence threshold for double-page spread detection. The higher the value, the stricter the criteria become. (percentage)[0.0-1.0]

  --spread-edge int (default: 30)
	Edge width for double-page spread detection (pixel)

  --spread-keep-orientation
	Keep the page original orientation. Do not rotate to maximize screen area

  --spread-keep-original
	Keep the original left and right page

  --title string
	Volume title. This affects epub/kepub output. Unspecified or blank means using the title in the input file or the filename without extension

  --trim=true|false (default: true)
	Enable/disable edge trimming

  --trim-margin int (default: 10)
	Safety trim margin (pixel)

  --trim-min-size float (default: 0.85)
	Minimum size after trimmed (percentage)[0.0-1.0]

  -v, --verbose
	Verbose output

  --version
	Show version

  --width uint (default: 1264)
	Output screen width (pixel)

  --work-dir string
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

For PDF input, you need to install one of these options. Other input formats need none of them.

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

## Notes

I developed this tool for my personal use so all default values cater for my own usage.  However, I'd be happy if this tool is useful for other fellow manga readers too. :smile:

I strongly condemn manga piracy and hope you won't use this tool for any illegal purposes. Please remember to always support and respect the hard work of manga artists.

If you found any bugs, please feel free to report an issue and I'll fix it when time permits. 
