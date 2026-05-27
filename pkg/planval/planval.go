// Package planval validates ralphex/quorex plan file format.
package planval

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

// ValidationError is a single format problem in a plan file.
type ValidationError struct {
	Line    int
	Message string
}

// Validate checks a plan file and returns all format errors found.
func Validate(data []byte) []ValidationError {
	var errs []ValidationError
	scanner := bufio.NewScanner(bytes.NewReader(data))

	var lineNum int
	var fenceOpen int    // line where the current open fence started (0 = not in fence)
	var fenceLang string // language tag of the open fence

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// track code fences (``` markers)
		if marker, ok := strings.CutPrefix(line, "```"); ok {
			if fenceOpen == 0 {
				fenceOpen = lineNum
				fenceLang = marker
			} else {
				fenceOpen = 0
				fenceLang = ""
			}
			continue
		}

		if fenceOpen > 0 {
			continue // inside fence: skip heading checks
		}

		// all ## headings in a plan must use "## Task: <name>" format with non-empty name.
		if content, ok := strings.CutPrefix(line, "## "); ok {
			taskName, hasPrefix := strings.CutPrefix(content, "Task: ")
			if !hasPrefix || strings.TrimSpace(taskName) == "" {
				errs = append(errs, ValidationError{
					Line: lineNum,
					Message: "task section missing required header format\n  Expected: ## Task: <name>\n  Found:    ## " + content,
				})
			}
		}
	}

	// unclosed fence
	if fenceOpen > 0 {
		errs = append(errs, ValidationError{
			Line: fenceOpen,
			Message: fmt.Sprintf(
				"fence block not closed\n  Opened at line %d (```%s), never closed before end of file",
				fenceOpen, fenceLang,
			),
		})
	}

	return errs
}

// FormatErrors produces the user-facing error message with a fix suggestion.
func FormatErrors(filename string, errs []ValidationError) string {
	if len(errs) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Error: invalid plan file %q\n\n", filename)
	for _, e := range errs {
		fmt.Fprintf(&b, "  Line %d: %s\n\n", e.Line, e.Message)
	}
	fmt.Fprintf(&b, "Fix manually or run:\n  quorex plans fix %s\n", filename)
	return b.String()
}
