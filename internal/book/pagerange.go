//
// pagerange.go
// Copyright (C) 2024 Teerapap Changwichukarn <teerapap.c@gmail.com>
//
// Distributed under terms of the MIT license.
//

package book

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type PageRange struct {
	indexes map[int]bool
}

func NewPageRange() *PageRange {
	var pr PageRange
	pr.indexes = make(map[int]bool)
	return &pr
}

func toInt(str string, ret *int) error {
	n, err := strconv.Atoi(strings.TrimSpace(str))
	if err != nil {
		return fmt.Errorf("'%s' is not page number", str)
	}
	if n <= 0 {
		return fmt.Errorf("%d is invalid. Page number must be positive number", n)
	}
	*ret = n
	return nil
}

func (pr *PageRange) Parse(str string, total int) error {
	str = strings.TrimSpace(str)
	parts := strings.Split(str, ",")
	for _, part := range parts {
		pair := strings.SplitN(part, "-", 2)
		start := 0
		end := 0
		if err := toInt(pair[0], &start); err != nil {
			return err
		}
		if start > total {
			return fmt.Errorf("%s is beyond total number of pages(%d)", part, total)
		}
		if len(pair) == 2 {
			if strings.TrimSpace(pair[1]) == "" {
				end = total
			} else {
				if err := toInt(pair[1], &end); err != nil {
					return err
				}
				if end > total {
					return fmt.Errorf("%s is beyond total number of pages(%d)", part, total)
				}
			}
		} else {
			end = start
		}
		if start > end {
			return fmt.Errorf("%s is invalid range", part)
		}
		pr.Add(start, end)
	}
	return nil
}

func (pr *PageRange) Add(start int, end int) {
	for i := start; i <= end; i++ {
		pr.indexes[i] = true
	}
}

func (pr PageRange) Contains(page int) bool {
	return pr.indexes[page]
}

func (pr PageRange) All() []int {
	pages := make([]int, 0, len(pr.indexes))
	for p := range pr.indexes {
		pages = append(pages, p)
	}
	slices.Sort(pages)
	return pages
}

func (pr PageRange) First() int {
	if pr.PageCount() == 0 {
		return -1
	}
	return pr.All()[0]
}

func (pr PageRange) Last() int {
	if pr.PageCount() == 0 {
		return -1
	}
	pages := pr.All()
	return pages[len(pages)-1]
}

func (pr PageRange) PageCount() int {
	return len(pr.indexes)
}

func (pr PageRange) String() string {
	var sb strings.Builder

	pages := pr.All()
	if len(pages) == 0 {
		return "[]"
	}

	inRange := false
	prev := -1
	sb.WriteString("[")
	for _, p := range pages {
		if p == prev+1 {
			// continue in range
			inRange = true
		} else {
			if inRange {
				// close range
				fmt.Fprintf(&sb, "-%d", prev)
				inRange = false
			}
			if sb.Len() > 1 {
				fmt.Fprint(&sb, ", ")
			}
			fmt.Fprintf(&sb, "%d", p)
		}
		prev = p
	}
	if inRange {
		// close range at last
		fmt.Fprintf(&sb, "-%d", prev)
	}
	sb.WriteString("]")

	return sb.String()
}
