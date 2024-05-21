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

type Size struct {
	width  uint
	height uint
}

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

func (s Size) String() string {
	return fmt.Sprintf("%dx%d", s.width, s.height)
}

func (s *Size) TranslateBy(dx int, dy int) {
	s.width = uint(max(0, int(s.width)+dx))
	s.height = uint(max(0, int(s.height)+dy))
}

func (s Size) ScaleBy(f float64) Size {
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
