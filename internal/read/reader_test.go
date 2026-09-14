package read

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRangesAndLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\nfour\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	results := Read(context.Background(), Request{
		Files:    []FileRequest{{Path: path, Start: 2, End: 4}},
		MaxLines: 2,
	})
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	got := results[0]
	if got.Content != "two\nthree" || got.ActualStart != 2 || got.ActualEnd != 3 || got.TotalLines != 4 || !got.Truncated {
		t.Fatalf("unexpected range result: %#v", got)
	}
}

func TestReadAppliesAggregateByteBudgetInRequestOrder(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")
	for path, content := range map[string]string{first: "abc", second: "你好"} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	results := Read(context.Background(), Request{
		Files:    []FileRequest{{Path: first}, {Path: second}},
		MaxBytes: 6,
	})
	if results[0].Content != "abc" || results[0].Bytes != 3 || results[0].Truncated {
		t.Fatalf("first result unexpectedly changed: %#v", results[0])
	}
	if results[1].Content != "你" || results[1].Bytes != len([]byte("你")) || !results[1].Truncated {
		t.Fatalf("second result did not preserve UTF-8 budget: %#v", results[1])
	}
}

func TestReadReportsErrorsWithoutDroppingOtherResults(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.txt")
	if err := os.WriteFile(valid, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}

	results := Read(context.Background(), Request{
		Files: []FileRequest{{Path: filepath.Join(dir, "missing.txt")}, {Path: valid}},
	})
	if len(results) != 2 || results[0].Error == "" || results[1].Error != "" || results[1].Content != "ok" {
		t.Fatalf("unexpected partial result: %#v", results)
	}
}

func TestReadRejectsBinaryAndInvalidUTF8(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "binary")
	invalid := filepath.Join(dir, "invalid")
	if err := os.WriteFile(binary, []byte{'a', 0, 'b'}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalid, []byte{0xff, 0xfe}, 0o600); err != nil {
		t.Fatal(err)
	}

	results := Read(context.Background(), Request{Files: []FileRequest{{Path: binary}, {Path: invalid}}})
	if !strings.Contains(results[0].Error, "binary") || !strings.Contains(results[1].Error, "UTF-8") {
		t.Fatalf("unexpected validation errors: %#v", results)
	}
}

func TestReadEmptyAndPastEOF(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	short := filepath.Join(dir, "short")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(short, []byte("one\ntwo"), 0o600); err != nil {
		t.Fatal(err)
	}

	results := Read(context.Background(), Request{Files: []FileRequest{{Path: empty}, {Path: short, Start: 3}}})
	if results[0].Error != "" || results[0].TotalLines != 0 || results[1].Error != "" || results[1].Content != "" {
		t.Fatalf("unexpected empty/past-EOF results: %#v", results)
	}
}
