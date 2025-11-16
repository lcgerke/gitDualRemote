package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	FormatHuman OutputFormat = "human"
	FormatJSON  OutputFormat = "json"
)

// Output handles formatted output to the user
type Output struct {
	writer      io.Writer
	format      OutputFormat
	autoDetect  bool
	colorEnabled bool
}

// NewOutput creates a new Output instance
func NewOutput(writer io.Writer) *Output {
	o := &Output{
		writer:     writer,
		autoDetect: true,
	}
	o.detectFormat()
	return o
}

// detectFormat automatically detects if output should be human or JSON
func (o *Output) detectFormat() {
	if !o.autoDetect {
		return
	}

	// Check if output is a TTY
	if file, ok := o.writer.(*os.File); ok {
		fileInfo, err := file.Stat()
		if err == nil && (fileInfo.Mode()&os.ModeCharDevice) != 0 {
			// It's a TTY - use human format
			o.format = FormatHuman
			o.colorEnabled = true
		} else {
			// Piped or redirected - use JSON
			o.format = FormatJSON
			o.colorEnabled = false
		}
	} else {
		// Not a file, default to human
		o.format = FormatHuman
		o.colorEnabled = false
	}
}

// SetFormat manually sets the output format
func (o *Output) SetFormat(format OutputFormat) {
	o.format = format
	o.autoDetect = false
	o.colorEnabled = (format == FormatHuman)
}

// SetColorEnabled manually enables/disables colors
func (o *Output) SetColorEnabled(enabled bool) {
	o.colorEnabled = enabled
}

// IsJSON returns true if output format is JSON
func (o *Output) IsJSON() bool {
	return o.format == FormatJSON
}

// Success prints a success message
func (o *Output) Success(message string) {
	if o.format == FormatJSON {
		o.printJSON(map[string]interface{}{
			"status":  "success",
			"message": message,
		})
		return
	}

	if o.colorEnabled {
		fmt.Fprintf(o.writer, "%s %s\n", color.GreenString("✓"), message)
	} else {
		fmt.Fprintf(o.writer, "✓ %s\n", message)
	}
}

// Error prints an error message
func (o *Output) Error(message string) {
	if o.format == FormatJSON {
		o.printJSON(map[string]interface{}{
			"status":  "error",
			"message": message,
		})
		return
	}

	if o.colorEnabled {
		fmt.Fprintf(o.writer, "%s %s\n", color.RedString("✗"), message)
	} else {
		fmt.Fprintf(o.writer, "✗ %s\n", message)
	}
}

// Warning prints a warning message
func (o *Output) Warning(message string) {
	if o.format == FormatJSON {
		o.printJSON(map[string]interface{}{
			"status":  "warning",
			"message": message,
		})
		return
	}

	if o.colorEnabled {
		fmt.Fprintf(o.writer, "%s %s\n", color.YellowString("⚠"), message)
	} else {
		fmt.Fprintf(o.writer, "⚠ %s\n", message)
	}
}

// Info prints an informational message
func (o *Output) Info(message string) {
	if o.format == FormatJSON {
		o.printJSON(map[string]interface{}{
			"status":  "info",
			"message": message,
		})
		return
	}

	fmt.Fprintf(o.writer, "%s\n", message)
}

// Header prints a header (only in human format)
func (o *Output) Header(title string) {
	if o.format == FormatJSON {
		return // Don't print headers in JSON
	}

	if o.colorEnabled {
		fmt.Fprintf(o.writer, "\n%s\n", color.New(color.Bold).Sprint(title))
	} else {
		fmt.Fprintf(o.writer, "\n%s\n", title)
	}
}

// Separator prints a separator line (only in human format)
func (o *Output) Separator() {
	if o.format == FormatJSON {
		return
	}

	fmt.Fprintln(o.writer, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// JSON outputs arbitrary data as JSON
func (o *Output) JSON(data interface{}) error {
	return o.printJSON(data)
}

// printJSON encodes and prints JSON data
func (o *Output) printJSON(data interface{}) error {
	encoder := json.NewEncoder(o.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Infof prints a formatted info message
func (o *Output) Infof(format string, args ...interface{}) {
	o.Info(fmt.Sprintf(format, args...))
}

// Successf prints a formatted success message
func (o *Output) Successf(format string, args ...interface{}) {
	o.Success(fmt.Sprintf(format, args...))
}

// Errorf prints a formatted error message
func (o *Output) Errorf(format string, args ...interface{}) {
	o.Error(fmt.Sprintf(format, args...))
}

// Warningf prints a formatted warning message
func (o *Output) Warningf(format string, args ...interface{}) {
	o.Warning(fmt.Sprintf(format, args...))
}
