package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// OutputWriter handles formatted output for commands.
type OutputWriter struct {
	w      io.Writer
	isJSON bool
}

// NewOutputWriter creates a new OutputWriter.
func NewOutputWriter(w io.Writer, isJSON bool) *OutputWriter {
	return &OutputWriter{w: w, isJSON: isJSON}
}

// WriteJSON writes data as formatted JSON.
func (o *OutputWriter) WriteJSON(data interface{}) error {
	enc := json.NewEncoder(o.w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// WriteTable writes data as a formatted table.
func (o *OutputWriter) WriteTable(headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print headers
	for i, h := range headers {
		fmt.Fprintf(o.w, "%-*s  ", widths[i], strings.ToUpper(h))
	}
	fmt.Fprintln(o.w)

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				fmt.Fprintf(o.w, "%-*s  ", widths[i], cell)
			}
		}
		fmt.Fprintln(o.w)
	}
}

// WriteResult writes the result in the appropriate format.
func (o *OutputWriter) WriteResult(data interface{}) error {
	if o.isJSON {
		return o.WriteJSON(data)
	}
	// For non-JSON output, commands should format their own output
	return nil
}

// FormatTime formats a time for display.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

// FormatDuration formats a duration for display.
func FormatDuration(minutes int) string {
	if minutes == 0 {
		return "-"
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := minutes / 60
	mins := minutes % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, mins)
}

// FormatNumber formats a number with comma separators.
func FormatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return insertCommas(fmt.Sprintf("%d", n))
}

func insertCommas(s string) string {
	n := len(s)
	if n <= 3 {
		return s
	}
	var result strings.Builder
	for i, c := range s {
		if i > 0 && (n-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}
	return result.String()
}

// Truncate truncates a string to the given length.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// PrintSection prints a section header.
func (o *OutputWriter) PrintSection(title string) {
	fmt.Fprintf(o.w, "\n%s\n", title)
	fmt.Fprintln(o.w, strings.Repeat("-", len(title)))
}

// PrintKeyValue prints a key-value pair.
func (o *OutputWriter) PrintKeyValue(key, value string) {
	fmt.Fprintf(o.w, "%-20s %s\n", key+":", value)
}

// PrintLine prints a line of text.
func (o *OutputWriter) PrintLine(format string, args ...interface{}) {
	fmt.Fprintf(o.w, format+"\n", args...)
}

// PrintError prints an error message.
func PrintError(w io.Writer, format string, args ...interface{}) {
	fmt.Fprintf(w, "Error: "+format+"\n", args...)
}
