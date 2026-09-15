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

func TestRunSkillPrintsGuide(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"skill"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("run skill code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "name: efficient-codebase-navigation") {
		t.Fatalf("guide does not contain the skill frontmatter:\n%s", stdout.String())
	}
}

func TestRunSkillAddInstallsIntoExplicitDirectory(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"skill", "--add", "--dir", dir}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("run skill --add code=%d stderr=%q", code, stderr.String())
	}
	path := filepath.Join(dir, "efficient-codebase-navigation", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("installed manifest missing: %v", err)
	}
	if !strings.Contains(string(content), "name: efficient-codebase-navigation") {
		t.Fatalf("installed manifest is not the skill guide:\n%s", content)
	}
	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("output does not report the installed path:\n%s", stdout.String())
	}
}

func TestRunSkillHelpExitsZero(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"skill", "--help"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("run skill --help code=%d, want 0", code)
	}
}

func TestRunSkillRejectsInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"skill", "--add", "--target", "vim"},
		{"skill", "--project"},
		{"skill", "--target", "claude"},
		{"skill", "--add", "--dir", t.TempDir(), "--project"},
		{"skill", "--add", "--dir", t.TempDir(), "--target", "claude"},
		{"skill", "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(context.Background(), args, strings.NewReader(""), &stdout, &stderr); code != 2 {
			t.Errorf("run(%v) code=%d, want 2", args, code)
		}
	}
}
