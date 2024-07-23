# CHANGELOG.md

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

