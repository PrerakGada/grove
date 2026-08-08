package ui

import (
	"fmt"
	"strings"
)

// Table renders aligned columns, sizing each one to its widest cell.
//
// This exists because the alternative — a `%-10s` guess per column — is only
// correct until one value is longer than the guess, and then it does not
// truncate, it pushes every later column right. The result reads as a table for
// short names and as noise for long ones, which is exactly when you are looking
// at it. Grove's own `ports` output did this: worktree names are user-chosen and
// routinely exceed ten characters.
type Table struct {
	header []string
	rows   [][]string
	// dimCol renders a column faint. Applied at print time so the escape codes
	// never count toward the measured width.
	dimCol map[int]bool
}

// NewTable starts a table with the given column headings.
func NewTable(header ...string) *Table {
	return &Table{header: header, dimCol: map[int]bool{}}
}

// Row appends one row. Short rows are padded, long ones are kept whole — losing
// a value silently would be worse than a ragged edge.
func (t *Table) Row(cells ...string) *Table {
	t.rows = append(t.rows, cells)
	return t
}

// Dim marks a column to render faint, e.g. a filesystem path beside real data.
func (t *Table) Dim(col int) *Table {
	t.dimCol[col] = true
	return t
}

// String renders the table with a dim header, two spaces of indent and two
// between columns. The final column is never padded, so a long trailing path
// cannot leave a wall of trailing whitespace.
func (t *Table) String() string {
	cols := len(t.header)
	for _, r := range t.rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	if cols == 0 {
		return ""
	}

	width := make([]int, cols)
	measure := func(cells []string) {
		for i, c := range cells {
			if w := len([]rune(c)); w > width[i] {
				width[i] = w
			}
		}
	}
	measure(t.header)
	for _, r := range t.rows {
		measure(r)
	}

	var b strings.Builder
	if len(t.header) > 0 {
		b.WriteString(Dim.Render(t.line(t.header, width, false)))
		b.WriteByte('\n')
	}
	for _, r := range t.rows {
		b.WriteString(t.line(r, width, true))
		b.WriteByte('\n')
	}
	return b.String()
}

func (t *Table) line(cells []string, width []int, style bool) string {
	var b strings.Builder
	b.WriteString("  ")
	for i, c := range cells {
		pad := width[i] - len([]rune(c))
		out := c
		if style && t.dimCol[i] {
			out = Dim.Render(c)
		}
		b.WriteString(out)
		if i < len(cells)-1 {
			b.WriteString(strings.Repeat(" ", pad+2))
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// Print writes the table to stdout.
func (t *Table) Print() { fmt.Print(t.String()) }
