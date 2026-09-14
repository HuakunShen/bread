# bread

`bread` is a small Go command-line tool for reading several known files or
line ranges in one bounded result. It is designed for coding-agent workflows:
independent reads happen in parallel, while output remains in the order the
caller requested.

## Build and test

```bash
go build -o bread ./cmd/bread
go test ./...
go vet ./...
```

The project uses only the Go standard library.

## Usage

```bash
./bread src/a.go:1-120 src/b.go:400-520
./bread --format json --request request.json
```

Ranges are 1-based and inclusive. An omitted range reads the whole file; a
start without an end reads from that line to the end of the file. The final
colon is treated as the range separator, so a Windows drive letter is not
mistaken for one.

Text output includes original line numbers by default:

```text
=== src/a.go (lines 1-2 of 20, 42 bytes) ===
1 | package example
2 | func main() {}
```

Use `--no-line-numbers` when another consumer needs plain content. JSON output
has one result per input file and retains partial failures:

```json
{
  "files": [
    {
      "path": "src/a.go",
      "actual_start": 1,
      "actual_end": 2,
      "total_lines": 20,
      "bytes": 42,
      "truncated": false,
      "content": "package example\nfunc main() {}"
    }
  ]
}
```

Request JSON uses this shape:

```json
{
  "files": [
    {"path": "src/a.go", "start": 1, "end": 120},
    {"path": "src/b.go", "start": 400, "end": 520}
  ]
}
```

Useful flags:

| Flag | Default | Description |
| --- | ---: | --- |
| `--format` | `text` | `text` or `json` |
| `--max-lines` | `2000` | maximum returned lines per file; `0` is unlimited |
| `--max-bytes` | `200000` | aggregate returned content bytes; `0` is unlimited |
| `--no-line-numbers` | off | omit line-number prefixes from text output |
| `--request` | none | JSON file, or `-` for stdin |
| `--version` | off | print the version |

Exit status is `0` when all files succeed, `1` when at least one file fails,
and `2` for invalid flags or malformed input. Binary files and invalid UTF-8
are reported as per-file errors instead of being dumped into agent context.

## Navigation skill

The repository includes
[`skills/efficient-codebase-navigation/SKILL.md`](skills/efficient-codebase-navigation/SKILL.md).
It explains when an agent should use `bread`, a native parallel read, a search,
or a sequential read whose next target depends on earlier output.

## Deliberate non-goals

The first release does not provide MCP, repository search, AST/LSP symbol
lookup, `.gitignore` traversal, token counting, editing, remote files, or a
daemon. Those are separate decisions and should be justified by observed use
rather than added to the basic reader prematurely.
