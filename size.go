//
// size.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package main

import (
	"fmt"
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
	width  uint
	height uint
}

func (s Size) String() string {
	return fmt.Sprintf("%dx%d", s.width, s.height)
}

func (s Size) ScaleBy(f float64) Size {
	assert(f >= 0, "scale factor cannot be negative")
	return Size{uint(float64(s.width) * f), uint(float64(s.height) * f)}
}

func (s Size) Rotate() Size {
	return Size{s.height, s.width}
}

func (s Size) CanFitIn(box Size) bool {
	return s.width <= box.width && s.height <= box.height
}

func (s Size) AspectFitIn(box Size, enlarge bool) Size {
	if s.CanFitIn(box) && !enlarge {
		return s
	}

	rW := float64(box.width) / float64(s.width)
	rH := float64(box.height) / float64(s.height)
	if rW < rH {
		return s.ScaleBy(rW)
	} else {
		return s.ScaleBy(rH)
	}
}

func (s Size) Orientation() Orientation {
	switch {
	case s.width < s.height:
		return Portrait
	case s.width > s.height:
		return Landscape
	default:
		return Square
	}
}

type Point struct {
	x int
	y int
}

func (p Point) String() string {
	return fmt.Sprintf("(%d, %d)", p.x, p.y)
}

func (p Point) TranslateBy(dx int, dy int) Point {
	return Point{p.x + dx, p.y + dy}
}

type Rect struct {
	origin Point
	size   Size
}

func (r Rect) String() string {
	return fmt.Sprintf("%s+%d+%d", r.size, r.origin.x, r.origin.y)
}

func (r Rect) MinX() int {
	return r.origin.x
}

func (r Rect) MinY() int {
	return r.origin.y
}

func (r Rect) MaxX() int {
	return r.origin.x + int(r.size.width)
}

func (r Rect) MaxY() int {
	return r.origin.y + int(r.size.height)
}

func (r Rect) TranslateBy(dx int, dy int) Rect {
	return Rect{r.origin.TranslateBy(dx, dy), r.size}
}

func (r Rect) InsetBy(dx int, dy int) Rect {
	origin := Point{
		x: r.origin.x + dx,
		y: r.origin.y + dy,
	}
	size := Size{
		width:  uint(max(0, int(r.size.width)-2*dx)),
		height: uint(max(0, int(r.size.height)-2*dy)),
	}
	return Rect{origin, size}
}

func (r Rect) BoundBy(frame Rect) Rect {
	nr := r
	nr.origin.x = max(r.MinX(), frame.MinX())
	nr.origin.y = max(r.MinY(), frame.MinY())
	nr.size.width = uint(min(nr.MaxX(), frame.MaxX()) - nr.origin.x)
	nr.size.height = uint(min(nr.MaxY(), frame.MaxY()) - nr.origin.y)
	return nr
}

func (r Rect) MoveInside(frame Rect) Rect {
	nr := r
	if nr.MinX() < frame.MinX() { // move origin x within frame
		nr.origin.x = frame.origin.x
	}
	if nr.MaxX() > frame.MaxX() { //move origin x back to fit max x within frame
		nr.origin.x -= nr.MaxX() - frame.MaxX()
	}
	if nr.MinY() < frame.MinY() { // move origin y within frame
		nr.origin.y = frame.origin.y
	}
	if nr.MaxY() > frame.MaxY() { //move origin y back to fit max y within frame
		nr.origin.y -= nr.MaxY() - frame.MaxY()
	}
	return nr.BoundBy(frame) // finally bounds by frame
}
