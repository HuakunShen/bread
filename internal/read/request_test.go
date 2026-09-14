package read

import "testing"

func TestParseSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    FileRequest
		wantErr bool
	}{
		{name: "whole file", spec: "src/main.go", want: FileRequest{Path: "src/main.go"}},
		{name: "start only", spec: "src/main.go:12", want: FileRequest{Path: "src/main.go", Start: 12}},
		{name: "inclusive range", spec: "src/main.go:12-30", want: FileRequest{Path: "src/main.go", Start: 12, End: 30}},
		{name: "windows drive path", spec: `C:\repo\main.go:4-8`, want: FileRequest{Path: `C:\repo\main.go`, Start: 4, End: 8}},
		{name: "colon in ordinary path", spec: "fixtures/a:b.txt", want: FileRequest{Path: "fixtures/a:b.txt"}},
		{name: "empty", spec: "", wantErr: true},
		{name: "zero start", spec: "a.go:0", wantErr: true},
		{name: "reversed", spec: "a.go:8-4", wantErr: true},
		{name: "malformed range", spec: "a.go:8-", want: FileRequest{Path: "a.go:8-"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseSpec(test.spec)
			if test.wantErr {
				if err == nil {
					t.Fatalf("ParseSpec(%q) error = nil, want error", test.spec)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSpec(%q) error = %v", test.spec, err)
			}
			if got != test.want {
				t.Fatalf("ParseSpec(%q) = %#v, want %#v", test.spec, got, test.want)
			}
		})
	}
}
