package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPositionalBatch(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.go")
	second := filepath.Join(dir, "second.go")
	if err := os.WriteFile(first, []byte("package first\nfunc A() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("package second\nfunc B() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--max-lines", "1", first, second}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	text := stdout.String()
	if !strings.Contains(text, "1 | package first") || !strings.Contains(text, "1 | package second") || !strings.Contains(text, "truncated") {
		t.Fatalf("unexpected positional output:\n%s", text)
	}
}

func TestRunJSONRequestAndErrors(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.txt")
	if err := os.WriteFile(valid, []byte("hello\nworld"), 0o600); err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(map[string]any{"files": []map[string]any{{"path": valid, "start": 2, "end": 2}, {"path": filepath.Join(dir, "missing")}}})
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--format", "json", "--request", "-"}, bytes.NewReader(request), &stdout, &stderr)
	if code != 1 || stderr.Len() != 0 {
		t.Fatalf("run code=%d stderr=%q", code, stderr.String())
	}
	var decoded struct {
		Files []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
			Error   string `json:"error"`
		} `json:"files"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Files) != 2 || decoded.Files[0].Content != "world" || decoded.Files[1].Error == "" {
		t.Fatalf("unexpected JSON output: %#v", decoded)
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{}, {"--format", "xml", "a"}, {"--max-bytes", "-1", "a"}} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, strings.NewReader(""), &stdout, &stderr); code != 2 {
			t.Errorf("run(%v) code=%d, want 2", args, code)
		}
	}
}
