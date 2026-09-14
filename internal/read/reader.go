package read

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

// Request is a batch of independent file reads. MaxLines applies per file;
// MaxBytes applies to the combined returned content bytes. A zero limit means
// unlimited.
type Request struct {
	Files    []FileRequest `json:"files"`
	MaxLines int           `json:"-"`
	MaxBytes int64         `json:"-"`
}

// Result is one stable, serializable outcome for one requested file.
type Result struct {
	Path           string `json:"path"`
	RequestedStart int    `json:"requested_start,omitempty"`
	RequestedEnd   int    `json:"requested_end,omitempty"`
	ActualStart    int    `json:"actual_start,omitempty"`
	ActualEnd      int    `json:"actual_end,omitempty"`
	TotalLines     int    `json:"total_lines"`
	Bytes          int    `json:"bytes"`
	Truncated      bool   `json:"truncated"`
	Content        string `json:"content,omitempty"`
	Error          string `json:"error,omitempty"`
}

// Read loads all requested files concurrently, while preserving request order
// in the returned slice. Budget application happens after I/O in request order
// so repeated calls produce the same result.
func Read(ctx context.Context, request Request) []Result {
	results := make([]Result, len(request.Files))
	var wait sync.WaitGroup
	wait.Add(len(request.Files))

	for index, file := range request.Files {
		go func(index int, file FileRequest) {
			defer wait.Done()
			if err := ctx.Err(); err != nil {
				results[index] = Result{Path: file.Path, Error: err.Error()}
				return
			}
			results[index] = readOne(file, request.MaxLines)
		}(index, file)
	}
	wait.Wait()

	applyByteBudget(results, request.MaxBytes)
	return results
}

func readOne(request FileRequest, maxLines int) Result {
	result := Result{
		Path:           request.Path,
		RequestedStart: request.Start,
		RequestedEnd:   request.End,
	}

	if strings.TrimSpace(request.Path) == "" {
		result.Error = "file path must not be empty"
		return result
	}
	if request.Start < 0 || request.End < 0 || (request.End > 0 && request.Start > 0 && request.End < request.Start) {
		result.Error = "invalid line range"
		return result
	}
	if maxLines < 0 {
		result.Error = "max lines must not be negative"
		return result
	}

	info, err := os.Stat(request.Path)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if info.IsDir() {
		result.Error = "path is a directory"
		return result
	}

	data, err := os.ReadFile(request.Path)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if bytes.IndexByte(data, 0) >= 0 {
		result.Error = "binary file is not supported"
		return result
	}
	if !utf8.Valid(data) {
		result.Error = "file is not valid UTF-8"
		return result
	}

	lines := splitLines(string(data))
	result.TotalLines = len(lines)
	if len(lines) == 0 {
		return result
	}

	start := request.Start
	if start == 0 {
		start = 1
	}
	if start > len(lines) {
		return result
	}
	end := request.End
	if end == 0 || end > len(lines) {
		end = len(lines)
	}
	selected := lines[start-1 : end]
	if maxLines > 0 && len(selected) > maxLines {
		selected = selected[:maxLines]
		result.Truncated = true
	}

	result.ActualStart = start
	result.ActualEnd = start + len(selected) - 1
	result.Content = strings.Join(selected, "\n")
	result.Bytes = len([]byte(result.Content))
	return result
}

func applyByteBudget(results []Result, maxBytes int64) {
	if maxBytes <= 0 {
		return
	}

	var used int64
	for index := range results {
		result := &results[index]
		if result.Error != "" {
			continue
		}
		remaining := maxBytes - used
		if remaining <= 0 {
			if result.Content != "" {
				result.Content = ""
				result.Bytes = 0
				result.Truncated = true
			}
			continue
		}

		contentBytes := []byte(result.Content)
		if int64(len(contentBytes)) > remaining {
			result.Content = utf8Prefix(result.Content, int(remaining))
			result.Bytes = len([]byte(result.Content))
			result.Truncated = true
		} else {
			result.Bytes = len(contentBytes)
		}
		used += int64(result.Bytes)
	}
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for index := range lines {
		lines[index] = strings.TrimSuffix(lines[index], "\r")
	}
	return lines
}

func utf8Prefix(text string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if maxBytes >= len(text) {
		return text
	}
	prefix := text[:maxBytes]
	for len(prefix) > 0 && !utf8.ValidString(prefix) {
		prefix = prefix[:len(prefix)-1]
	}
	return prefix
}

// ValidateLimits returns a user-facing error for invalid global limits.
func ValidateLimits(maxLines int, maxBytes int64) error {
	if maxLines < 0 {
		return fmt.Errorf("max lines must not be negative")
	}
	if maxBytes < 0 {
		return fmt.Errorf("max bytes must not be negative")
	}
	return nil
}
