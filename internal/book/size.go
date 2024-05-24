//
// size.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"

	"github.com/teerapap/mangafmt/internal/util"
)

type Orientation int

const (
	Portrait = iota
	Landscape
	Square
)

func (o Orientation) String() string {
	switch o {
	case Portrait:
		return "portrait"
	case Landscape:
		return "landscape"
	default:
		return "square"
	}
}

type Size struct {
	Width  uint
	Height uint
}

func (s Size) String() string {
	return fmt.Sprintf("%dx%d", s.Width, s.Height)
}

func (s Size) ScaleBy(f float64) Size {
	util.Assert(f >= 0, "scale factor cannot be negative")
	return Size{uint(float64(s.Width) * f), uint(float64(s.Height) * f)}
}

func (s Size) Rotate() Size {
	return Size{s.Height, s.Width}
}

func (s Size) CanFitIn(box Size) bool {
	return s.Width <= box.Width && s.Height <= box.Height
}

func (s Size) AspectFitIn(box Size, enlarge bool) Size {
	if s.CanFitIn(box) && !enlarge {
		return s
	}

	rW := float64(box.Width) / float64(s.Width)
	rH := float64(box.Height) / float64(s.Height)
	if rW < rH {
		return s.ScaleBy(rW)
	} else {
		return s.ScaleBy(rH)
	}
}

func (s Size) Orientation() Orientation {
	switch {
	case s.Width < s.Height:
		return Portrait
	case s.Width > s.Height:
		return Landscape
	default:
		return Square
	}
}

type Point struct {
	X int
	Y int
}

func (p Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.X, p.Y)
}

func (p Point) TranslateBy(dx int, dy int) Point {
	return Point{p.X + dx, p.Y + dy}
}

type Rect struct {
	origin Point
	size   Size
}

func (r Rect) String() string {
	return fmt.Sprintf("%s+%d+%d", r.size, r.origin.X, r.origin.Y)
}

func (r Rect) MinX() int {
	return r.origin.X
}

func (r Rect) MinY() int {
	return r.origin.Y
}

func (r Rect) MaxX() int {
	return r.origin.X + int(r.size.Width)
}

func (r Rect) MaxY() int {
	return r.origin.Y + int(r.size.Height)
}

func (r Rect) TranslateBy(dx int, dy int) Rect {
	return Rect{r.origin.TranslateBy(dx, dy), r.size}
}

func (r Rect) LeftEdge(width uint, margin uint) Rect {
	remWidth := max(0, int(r.size.Width)-int(margin))
	x := int(min(margin, r.size.Width))
	edgeWidth := min(uint(remWidth), width)
	return Rect{Point{x, 0}, Size{edgeWidth, r.size.Height}}
}

func (r Rect) RightEdge(width uint, margin uint) Rect {
	remWidth := max(0, int(r.size.Width)-int(margin))
	x := max(0, remWidth-int(width))
	edgeWidth := min(uint(remWidth), width)
	return Rect{Point{x, 0}, Size{edgeWidth, r.size.Height}}
}

func (r Rect) InsetBy(dx int, dy int) Rect {
	origin := Point{
		X: r.origin.X + dx,
		Y: r.origin.Y + dy,
	}
	size := Size{
		Width:  uint(max(0, int(r.size.Width)-2*dx)),
		Height: uint(max(0, int(r.size.Height)-2*dy)),
	}
	return Rect{origin, size}
}

func (r Rect) BoundBy(frame Rect) Rect {
	nr := r
	nr.origin.X = max(r.MinX(), frame.MinX())
	nr.origin.Y = max(r.MinY(), frame.MinY())
	nr.size.Width = uint(min(nr.MaxX(), frame.MaxX()) - nr.origin.X)
	nr.size.Height = uint(min(nr.MaxY(), frame.MaxY()) - nr.origin.Y)
	return nr
}

func (r Rect) MoveInside(frame Rect) Rect {
	nr := r
	if nr.MinX() < frame.MinX() { // move origin x within frame
		nr.origin.X = frame.origin.X
	}
	if nr.MaxX() > frame.MaxX() { //move origin x back to fit max x within frame
		nr.origin.X -= nr.MaxX() - frame.MaxX()
	}
	if nr.MinY() < frame.MinY() { // move origin y within frame
		nr.origin.Y = frame.origin.Y
	}
	if nr.MaxY() > frame.MaxY() { //move origin y back to fit max y within frame
		nr.origin.Y -= nr.MaxY() - frame.MaxY()
	}
	return nr.BoundBy(frame) // finally bounds by frame
}
