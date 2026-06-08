//
// util.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package util

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/teerapap/mangafmt/internal/log"
)

const AppVersion = "v0.6.0"

func PrintFlagsUsage(output io.Writer) {
	type flagHelp struct {
		names []string
		flag  *flag.Flag
	}

	helps := make([]flagHelp, 0)
	flag.VisitAll(func(f *flag.Flag) {
		name := f.Name
		if len(name) == 1 {
			//short flag
			name = "-" + name
		} else {
			name = "--" + name
		}
		for i := range helps {
			if helps[i].flag.Usage == f.Usage {
				// merge multiple flags with the same usage together
				helps[i].names = append(helps[i].names, name)
				return
			}
		}

		helps = append(helps, flagHelp{
			names: []string{name},
			flag:  f,
		})
	})

	// format help usage for each flag
	for _, help := range helps {
		fmt.Fprintf(output, "  %s", strings.Join(help.names, ", "))

		tname, _ := flag.UnquoteUsage(help.flag)
		if tname == "value" {
			tname = "string"
		}
		switch tname {
		case "":
			// boolean
			if help.flag.DefValue != "false" {
				fmt.Fprintf(output, "=true|false (default: %v)", help.flag.DefValue)
			}
		case "string":
			if help.flag.DefValue != "" {
				fmt.Fprintf(output, " %s (default: %q)", tname, help.flag.DefValue)
			} else {
				fmt.Fprintf(output, " %s", tname)
			}
		default:
			fmt.Fprintf(output, " %s (default: %s)", tname, help.flag.DefValue)
		}
		fmt.Fprintf(output, "\n\t%s\n\n", help.flag.Usage)

	}
}

func Must(err error) func(doing string, logger log.Logger) {
	return func(doing string, logger log.Logger) {
		if err != nil {
			logger.Fatal(fmt.Sprintf("Error while %s:", doing), "err", err)
		}
	}
}

func Must1[T any](obj T, err error) func(doing string, logger log.Logger) T {
	return func(doing string, logger log.Logger) T {
		Must(err)(doing, logger)
		return obj
	}
}

func Must2[T1 any, T2 any](obj1 T1, obj2 T2, err error) func(doing string, logger log.Logger) (T1, T2) {
	return func(doing string, logger log.Logger) (T1, T2) {
		Must(err)(doing, logger)
		return obj1, obj2
	}
}

func Assert(cond bool, errMsg string) {
	if !cond {
		panic(errors.New(errMsg))
	}
}

func CreateWorkDir(path *string, clean bool) (bool, error) {
	if strings.TrimSpace(*path) == "" {
		// create temp directory
		tmpDir, err := os.MkdirTemp("", "mangafmt-")
		if err != nil {
			return true, fmt.Errorf("creating temp directory: %w", err)
		}
		*path = tmpDir
		return true, nil
	} else {
		if clean {
			// clean existing directory
			err := os.RemoveAll(*path)
			if err != nil {
				return false, fmt.Errorf("removing existing work directory: %w", err)
			}
		}
		err := os.MkdirAll(*path, 0750)
		if err != nil {
			return false, err
		}
		return false, nil
	}
}

func IsReadableFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return path, err
	}
	defer f.Close()
	return filepath.Abs(path)
}

func IsWritableFile(path string) (string, error) {
	return filepath.Abs(path)
}

func NameWithoutExt(filename string) string {
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

func ReplaceExt(path string, ext string) string {
	if ext == "" {
		return NameWithoutExt(path)
	} else {
		return fmt.Sprintf("%s.%s", NameWithoutExt(path), ext)
	}
}

func CreateTemplate(name string, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}

func WriteFileToZip(w *zip.Writer, path string, tm *template.Template, data any) error {
	out, err := w.Create(path)
	if err != nil {
		return fmt.Errorf("creating file(%s): %w", path, err)
	}

	err = tm.Execute(out, data)
	if err != nil {
		return fmt.Errorf("generating file(%s) from template: %w", path, err)
	}

	return nil
}

func CopyFileToZip(w *zip.Writer, dst string, src string) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening src file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("getting src file info: %w", err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("converting src file info to file header: %w", err)
	}
	header.Method = zip.Deflate
	if dst != "" {
		header.Name = dst
	}

	out, err := w.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("creating file entry in the zip file: %w", err)
	}

	_, err = io.Copy(out, f)
	if err != nil {
		return fmt.Errorf("copy src file into the zip file: %w", err)
	}
	return nil
}

func DigitCount(total int) int {
	d := 1
	for ; total >= 10; total = total / 10 {
		d += 1
	}
	return d
}
