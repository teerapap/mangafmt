//
// log.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package log

import (
	"fmt"
	"os"
	"strings"
)

var verbose bool

func SetVerbose(enabled bool) {
	verbose = enabled
}

var indentLevel int = 0
var indent string

func IndentLevel() int {
	return indentLevel
}

func SetIndentLevel(level int) {
	indentLevel = level
	indent = strings.Repeat(" ", int(max(0, level))*4)
}

func Indent() {
	SetIndentLevel(indentLevel + 1)
}

func Unindent() {
	SetIndentLevel(indentLevel - 1)
}

func Verbose(str string) {
	Verbosef(str)
}

func Verbosef(format string, v ...any) {
	if verbose {
		fmt.Fprintf(os.Stdout, "%sVerbose: %s\n", indent, fmt.Sprintf(format, v...))
	}
}

func Print(str string) {
	Printf(str)
}

func Printf(format string, v ...any) {
	fmt.Fprintf(os.Stdout, "%s%s\n", indent, fmt.Sprintf(format, v...))
}

func Error(str string) {
	Errorf(str)
}

func Errorf(format string, v ...any) {
	fmt.Fprintf(os.Stderr, "%sError: %s\n", indent, fmt.Sprintf(format, v...))
}

func Panic(str string) {
	Panicf(str)
}

func Panicf(format string, v ...any) {
	s := fmt.Sprintf(format, v...)
	fmt.Fprintf(os.Stderr, "%sError: %s\n", indent, s)
	panic(s)
}
