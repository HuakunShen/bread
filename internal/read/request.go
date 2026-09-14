package read

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// FileRequest identifies one file and an optional 1-based, inclusive line
// range. A zero bound means that the bound was not specified.
type FileRequest struct {
	Path  string `json:"path"`
	Start int    `json:"start,omitempty"`
	End   int    `json:"end,omitempty"`
}

var rangeSuffix = regexp.MustCompile(`^(.*):([0-9]+)(?:-([0-9]+))?$`)

// ParseSpec parses path, path:start, or path:start-end. The range suffix is
// matched from the final colon so a Windows drive letter remains in the path.
func ParseSpec(spec string) (FileRequest, error) {
	if strings.TrimSpace(spec) == "" {
		return FileRequest{}, fmt.Errorf("file request must not be empty")
	}

	request := FileRequest{Path: spec}
	if match := rangeSuffix.FindStringSubmatch(spec); match != nil {
		if strings.TrimSpace(match[1]) == "" {
			return FileRequest{}, fmt.Errorf("file path must not be empty")
		}

		start, err := strconv.Atoi(match[2])
		if err != nil || start < 1 {
			return FileRequest{}, fmt.Errorf("invalid start line in %q", spec)
		}
		request.Path = match[1]
		request.Start = start

		if match[3] != "" {
			end, parseErr := strconv.Atoi(match[3])
			if parseErr != nil || end < 1 {
				return FileRequest{}, fmt.Errorf("invalid end line in %q", spec)
			}
			if end < start {
				return FileRequest{}, fmt.Errorf("end line must be at least start line in %q", spec)
			}
			request.End = end
		}
	}

	return request, nil
}
