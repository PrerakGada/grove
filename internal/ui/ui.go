// Package ui holds Grove's terminal styling, kept in one place so output stays
// consistent across commands.
package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

var (
	Title = lipgloss.NewStyle().Bold(true)
	Dim   = lipgloss.NewStyle().Faint(true)
	Key   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	okMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).SetString("✓")
	infoMark = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).SetString("→")
	warnMark = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).SetString("!")
	errMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).SetString("✗")
)

func Ok(format string, a ...any)   { fmt.Printf("%s %s\n", okMark, fmt.Sprintf(format, a...)) }
func Info(format string, a ...any) { fmt.Printf("%s %s\n", infoMark, fmt.Sprintf(format, a...)) }
func Warn(format string, a ...any) { fmt.Printf("%s %s\n", warnMark, fmt.Sprintf(format, a...)) }

func Err(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", errMark, fmt.Sprintf(format, a...))
}
