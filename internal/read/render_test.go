package read

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderText(t *testing.T) {
	results := []Result{
		{Path: "a.go", ActualStart: 3, ActualEnd: 4, TotalLines: 8, Bytes: 9, Content: "third\nfourth"},
		{Path: "missing.go", Error: "file does not exist"},
	}
	var output bytes.Buffer
	if err := RenderText(&output, results, true); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, expected := range []string{"=== a.go (lines 3-4 of 8, 9 bytes) ===", "3 | third", "4 | fourth", "=== missing.go (error) ===", "ERROR: file does not exist"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("text output missing %q:\n%s", expected, text)
		}
	}
}

func TestRenderTextWithoutLineNumbers(t *testing.T) {
	var output bytes.Buffer
	if err := RenderText(&output, []Result{{Path: "a", ActualStart: 5, ActualEnd: 5, TotalLines: 5, Content: "line"}}, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "5 |") || !strings.Contains(output.String(), "\nline\n") {
		t.Fatalf("unexpected line-number-free output: %q", output.String())
	}
}

func TestRenderJSON(t *testing.T) {
	var output bytes.Buffer
	want := []Result{{Path: "a.go", ActualStart: 1, ActualEnd: 1, TotalLines: 1, Bytes: 2, Content: "ok"}}
	if err := RenderJSON(&output, want); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Files []Result `json:"files"`
	}
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(decoded.Files) != 1 || decoded.Files[0].Content != "ok" {
		t.Fatalf("unexpected JSON result: %#v", decoded)
	}
}
