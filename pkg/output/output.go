// Package output provides unified output formatting for bifu-cli.
// Supports table (default) and JSON output modes.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

// Format controls how command output is rendered.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatPlain Format = "plain"
)

var (
	Success = color.New(color.FgGreen).SprintFunc()
	Warn    = color.New(color.FgYellow).SprintFunc()
	ErrText = color.New(color.FgRed).SprintFunc()
	Bold    = color.New(color.Bold).SprintFunc()
	Dim     = color.New(color.Faint).SprintFunc()
	Cyan    = color.New(color.FgCyan).SprintFunc()
)

// Printer is the shared output printer for a command.
type Printer struct {
	Format  Format
	Verbose bool
	Out     io.Writer
	ErrOut  io.Writer
}

// NewPrinter creates a Printer using stdout/stderr.
func NewPrinter(format Format, verbose bool) *Printer {
	return &Printer{
		Format:  format,
		Verbose: verbose,
		Out:     os.Stdout,
		ErrOut:  os.Stderr,
	}
}

// PrintJSON serialises v as indented JSON.
func (p *Printer) PrintJSON(v interface{}) {
	enc := json.NewEncoder(p.Out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// PrintTable renders rows as an ASCII table.
// headers: column titles; rows: each row is a []string.
func (p *Printer) PrintTable(headers []string, rows [][]string) {
	tbl := tablewriter.NewWriter(p.Out)
	tbl.SetHeader(headers)
	tbl.SetBorder(false)
	tbl.SetHeaderLine(true)
	tbl.SetRowLine(false)
	tbl.SetColumnSeparator("  ")
	tbl.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	tbl.SetAlignment(tablewriter.ALIGN_LEFT)
	tbl.SetTablePadding("  ")
	tbl.SetNoWhiteSpace(true)
	for _, row := range rows {
		tbl.Append(row)
	}
	tbl.Render()
}

// PrintKV renders key-value pairs (for single object display).
func (p *Printer) PrintKV(pairs []KV) {
	maxKey := 0
	for _, kv := range pairs {
		if len(kv.Key) > maxKey {
			maxKey = len(kv.Key)
		}
	}
	for _, kv := range pairs {
		padding := strings.Repeat(" ", maxKey-len(kv.Key))
		fmt.Fprintf(p.Out, "  %s%s  %s\n", Cyan(kv.Key), padding, kv.Value)
	}
}

// KV is a key-value display pair.
type KV struct {
	Key   string
	Value string
}

// OK prints a success message.
func (p *Printer) OK(msg string, args ...interface{}) {
	fmt.Fprintln(p.Out, Success("✓ ")+fmt.Sprintf(msg, args...))
}

// Err prints an error message to stderr.
func (p *Printer) Err(msg string, args ...interface{}) {
	fmt.Fprintln(p.ErrOut, ErrText("✗ ")+fmt.Sprintf(msg, args...))
}

// Verbose prints a message only when verbose mode is enabled.
func (p *Printer) Log(msg string, args ...interface{}) {
	if p.Verbose {
		fmt.Fprintln(p.Out, Dim("  "+fmt.Sprintf(msg, args...)))
	}
}

// Header prints a section header.
func (p *Printer) Header(msg string) {
	fmt.Fprintln(p.Out, "\n"+Bold(msg))
}

// Line prints a plain line.
func (p *Printer) Line(msg string, args ...interface{}) {
	fmt.Fprintln(p.Out, fmt.Sprintf(msg, args...))
}
