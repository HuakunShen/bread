package read

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// RenderText writes a stable human/agent-readable representation of every
// result. Selected lines retain their original line numbers by default.
func RenderText(writer io.Writer, results []Result, lineNumbers bool) error {
	for index, result := range results {
		if index > 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}

		if result.Error != "" {
			if _, err := fmt.Fprintf(writer, "=== %s (error) ===\nERROR: %s\n", result.Path, result.Error); err != nil {
				return err
			}
			continue
		}

		rangeLabel := "no lines"
		if result.ActualStart > 0 {
			rangeLabel = fmt.Sprintf("lines %d-%d of %d", result.ActualStart, result.ActualEnd, result.TotalLines)
		}
		truncatedLabel := ""
		if result.Truncated {
			truncatedLabel = ", truncated"
		}
		if _, err := fmt.Fprintf(writer, "=== %s (%s, %d bytes%s) ===\n", result.Path, rangeLabel, result.Bytes, truncatedLabel); err != nil {
			return err
		}
		if result.Content == "" {
			continue
		}

		lines := strings.Split(result.Content, "\n")
		for offset, line := range lines {
			if lineNumbers && result.ActualStart > 0 {
				if _, err := fmt.Fprintf(writer, "%d | %s\n", result.ActualStart+offset, line); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintln(writer, line); err != nil {
				return err
			}
		}
	}
	return nil
}

// RenderJSON writes a stable JSON envelope containing all file results.
func RenderJSON(writer io.Writer, results []Result) error {
	return json.NewEncoder(writer).Encode(struct {
		Files []Result `json:"files"`
	}{Files: results})
}
