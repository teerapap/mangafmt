# CHANGELOG.md

## Unreleased

Features:

* Support EPUB/KEPUB file as input with the help from Claude Code.
  * Only manga epub file(a volume of image pages) is supported. A spine item without an image is skipped and the file is rejected when it has no image page at all.
  * Neither ImageMagick nor VIPS is required for epub input.
  * The metadata and the table of contents of the input file are kept in the epub/kepub output file.
  * The read direction is read from the input file. The `--rtl` flag takes precedence over it.
  * The volume title and author are read from the input file. The `--title` and `--author` flags take precedence over them.

Bug Fixes:

* Stop before formatting when the output file is the same file as an input file, instead of overwriting the input file.

## v0.7.0 (2026-06-09)

Features:

* Support multiple input files.
* Support `--parallel` flag to control the number of files to format concurrently.
* Support `--log-file` flag to write log messages to file instead of standard output.
* Support `--progress-bar` flag to show progress bar.
* Support `--resize` flag to enable/disable resize step.
* Support `--convert-only` flag to disable all formatting steps.

Bug Fixes:

* Update golang.org/x/image to fix reported vulnerability.

Improvements:

* Improve logging with structured messages with colors.
* Reduce memory footprint.
* Change term from `book` to `volume`.
* Return wrapped error and log as fatal instead of panic.
* Improve help usage format

## v0.6.0 (2026-05-24)

Features:

* Detect background automatically when trimming margin borders.
  * Remove `-background` flag.
* Support `-author` for EPUB/KEPUB format output.

Bug Fixes:

* Fix ImageMagick6/7 page extraction to apply auto-orientation and cropbox functions in pdf.

Improvements:

* Improve double-page spread detection by adding more heuristic signals with the help from Claude code.
  * Add `-spread-confidence` flag.
  * Remove `-spread-margin` flag.
  * Remove `-spread-bg-distortion` flag.
  * Remove `-spread-lr-distortion` flag.
* Speed-up loading page by using LRU cache.

## v0.5.0 (2026-03-09)

Features:

* Support `-spread-keep-orientation` and `-spread-keep-original` flag.

## v0.4.0 (2024-07-24)

Features:

* Support multiple background colors for `--background`.
  * Some manga may have both white and black background so using single background color may lead to incorrect double-page spread detection for some pages.

Bug Fixes:

* Fix wrong background hex color in EPUB output format.

Improvements:

* Print version and full command arguments when `--verbose` is enabled for debugging.

## v0.3.0 (2024-07-14)

Features:

* Support Windows and OSX in addition to Linux.
* Reduce memory consumption by ~75%
* Faster processing time up to ~70% improvments (using `libvips`)

Bug Fixes:

* Fix missing `style.css` in EPUB output format.

Functional Changes:

* Require `ImageMagick6` or `ImageMagick7` or `libvips` commands during runtime for PDF input.
* Do not require `libmagickwand` as build or runtime dependencies.
* `--background` now support only hex format.

Improvements:

* Pure Go code without cgo-linked dependencies
* Improve edge trimming.
* Use static app version and do not rely on `debug.BuildInfo`.
* Replace `google/uuid` with `hashicorp/go-uuid`.

## v0.2.1 (2024-06-03)

Bug Fixes:

* Fix malformed epub/kepub output due to html/template bug.

## v0.2.0 (2024-06-03)

Features:

* Improve grayscale color depth reduction to reduce output file size substantially.
* Add `--grayscale-depth` command-line argument.

## v0.1.0 (2024-06-02)

First public release

Features:

* Detect double-page spread (a big scene that covers two facing pages) heuristically and connect them into one landscape page.
* Trim blank spaces around the edges for better.
* Resize/rotate page to fit specific screen size.
* Reduce file size by reducing colors to grayscale (except the cover page or configured otherwise).
* Handle right-to-left (RTL) read direction.
* Convert to EPUB/KEPUB/CBZ format.

