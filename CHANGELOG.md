# CHANGELOG.md

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

