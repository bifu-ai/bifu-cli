// Package output provides unified output formatting for bifu-cli.
// Supports table (default), JSON, and plain output modes, with ANSI-aware
// tables (numeric columns auto right-align, semantic coloring of PnL / side /
// status applied at render time so JSON output stays clean).
package output

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
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

// PrintTable renders rows as a table, or as a JSON array of objects when the
// format is JSON. headers are used as object keys in JSON mode.
//
// In table mode: columns whose every non-empty cell parses as a number are
// right-aligned, and columns recognised as PnL / side / status are colored by
// value (only when writing to a terminal — fatih/color disables itself for
// pipes, keeping piped output clean).
func (p *Printer) PrintTable(headers []string, rows [][]string) {
	p.PrintTableWithFooter(headers, rows, nil)
}

// PrintTableWithFooter is PrintTable with an optional footer row (e.g. totals).
// The footer is ignored in JSON mode.
func (p *Printer) PrintTableWithFooter(headers []string, rows [][]string, footer []string) {
	if p.Format == FormatJSON {
		out := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			obj := make(map[string]string, len(headers))
			for i, h := range headers {
				if i < len(row) {
					obj[h] = row[i]
				}
			}
			out = append(out, obj)
		}
		p.PrintJSON(out)
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(p.Out)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false
	t.Style().Options.SeparateHeader = true
	t.Style().Options.SeparateRows = false
	t.Style().Box.PaddingLeft = ""
	t.Style().Box.PaddingRight = "   "
	t.Style().Format.Header = text.FormatUpper
	// go-pretty colors do not auto-disable for non-terminals the way
	// fatih/color does, so only enable them when color is on.
	if !color.NoColor {
		t.Style().Color.Header = text.Colors{text.Bold}
	}

	hdr := make(table.Row, len(headers))
	colorers := make([]func(string) string, len(headers))
	cfgs := make([]table.ColumnConfig, 0, len(headers))
	for i, h := range headers {
		hdr[i] = h
		colorers[i] = colorizerFor(h)
		cc := table.ColumnConfig{Number: i + 1}
		if isNumericColumn(rows, i) {
			cc.Align = text.AlignRight
			cc.AlignHeader = text.AlignRight
		}
		cfgs = append(cfgs, cc)
	}
	t.AppendHeader(hdr)
	t.SetColumnConfigs(cfgs)

	for _, row := range rows {
		r := make(table.Row, len(row))
		for i, cell := range row {
			if i < len(colorers) && colorers[i] != nil {
				r[i] = colorers[i](cell)
			} else {
				r[i] = cell
			}
		}
		t.AppendRow(r)
	}
	if len(footer) > 0 {
		fr := make(table.Row, len(footer))
		for i, c := range footer {
			fr[i] = c
		}
		t.AppendFooter(fr)
	}
	t.Render()
}

// ── Semantic coloring ───────────────────────────────────────────────────────

// colorizerFor returns a per-cell color function based on the column header,
// or nil when the column should not be colored.
func colorizerFor(header string) func(string) string {
	h := strings.ToUpper(strings.TrimSpace(header))
	switch {
	case strings.Contains(h, "PNL") || strings.Contains(h, "P&L") ||
		strings.Contains(h, "PROFIT") || strings.Contains(h, "ROI"):
		return ColorSigned
	case h == "SIDE" || strings.Contains(h, "ORDER_SIDE") || strings.Contains(h, "ORDER SIDE") ||
		strings.Contains(h, "POSITION SIDE") || strings.Contains(h, "POSITION_SIDE"):
		return ColorSide
	case h == "STATUS":
		return ColorStatus
	default:
		return nil
	}
}

// ColorSigned colors a numeric string green when positive, red when negative.
func ColorSigned(s string) string {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return s
	}
	switch {
	case v > 0:
		return Success(s)
	case v < 0:
		return ErrText(s)
	default:
		return s
	}
}

// ColorSide colors a trade side: BUY/LONG green, SELL/SHORT red.
func ColorSide(s string) string {
	switch u := strings.ToUpper(strings.TrimSpace(s)); {
	case u == "":
		return s
	case strings.HasPrefix(u, "BUY") || u == "LONG":
		return Success(s)
	case strings.HasPrefix(u, "SELL") || u == "SHORT":
		return ErrText(s)
	default:
		return s
	}
}

// ColorStatus colors an order/account status by its meaning.
func ColorStatus(s string) string {
	u := strings.ToUpper(strings.TrimSpace(s))
	switch {
	case u == "":
		return s
	case strings.Contains(u, "FILL") || u == "SUCCESS" || u == "ACTIVE" || u == "DONE" || u == "NORMAL":
		return Success(s)
	case strings.Contains(u, "CANCEL"):
		return Dim(s)
	case strings.Contains(u, "REJECT") || strings.Contains(u, "FAIL") || strings.Contains(u, "EXPIRE"):
		return ErrText(s)
	case strings.Contains(u, "PENDING") || strings.Contains(u, "NEW") || strings.Contains(u, "PART"):
		return Warn(s)
	default:
		return s
	}
}

// isNumericColumn reports whether every non-empty cell in column i parses as a
// number (so the column can be right-aligned).
func isNumericColumn(rows [][]string, i int) bool {
	seen := false
	for _, row := range rows {
		if i >= len(row) {
			continue
		}
		c := strings.TrimSpace(row[i])
		if c == "" || c == "-" {
			continue
		}
		if _, err := strconv.ParseFloat(c, 64); err != nil {
			return false
		}
		seen = true
	}
	return seen
}

// PrintKV renders key-value pairs as aligned text, or as a JSON object
// when the format is JSON.
func (p *Printer) PrintKV(pairs []KV) {
	if p.Format == FormatJSON {
		obj := make(map[string]string, len(pairs))
		for _, kv := range pairs {
			obj[kv.Key] = kv.Value
		}
		p.PrintJSON(obj)
		return
	}
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

// Log prints a message only when verbose mode is enabled.
func (p *Printer) Log(msg string, args ...interface{}) {
	if p.Verbose {
		fmt.Fprintln(p.Out, Dim("  "+fmt.Sprintf(msg, args...)))
	}
}

// Header prints a section header. Suppressed in JSON mode.
func (p *Printer) Header(msg string) {
	if p.Format == FormatJSON {
		return
	}
	fmt.Fprintln(p.Out, "\n"+Bold(msg))
}

// Line prints a plain line. Suppressed in JSON mode.
func (p *Printer) Line(msg string, args ...interface{}) {
	if p.Format == FormatJSON {
		return
	}
	fmt.Fprintln(p.Out, fmt.Sprintf(msg, args...))
}

// Confirm asks the user a yes/no question on stderr and returns true only on an
// explicit yes. In JSON mode it returns false (callers should require --yes for
// non-interactive use). Returns false when stdin is not a terminal and no
// default is given.
func (p *Printer) Confirm(prompt string) bool {
	fmt.Fprintf(p.ErrOut, "%s %s ", Warn("?"), prompt+" [y/N]:")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	a := strings.ToLower(strings.TrimSpace(line))
	return a == "y" || a == "yes"
}
