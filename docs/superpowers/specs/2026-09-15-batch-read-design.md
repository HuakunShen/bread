# Batch Read Design

**Date:** 2026-09-15

## Goal

Provide a small, dependency-free Go binary named `bread` that lets a coding
agent read several known files or line ranges in one command and receive one
deterministic, bounded result. The repository also ships a reusable skill that
teaches agents when batching is useful and when it is not.

## Scope

The first release supports:

- positional file requests such as `src/a.go` and `src/a.go:10-80`;
- a JSON request file for callers that need an unambiguous structured input;
- 1-based, inclusive line ranges;
- parallel file reads with output emitted in request order;
- text and JSON output formats;
- line numbers in text output;
- per-file line limits and an aggregate byte budget;
- UTF-8 validation and binary-file refusal;
- partial results when one request fails, with a non-zero process exit status;
- tests for parsing, line slicing, truncation, ordering, formatting, and CLI
  behavior;
- `skills/efficient-codebase-navigation/SKILL.md` and its prompt eval set.

The implementation uses only the Go standard library. The core read package is
independent of the CLI so that a future MCP adapter can reuse the semantics
without changing the file-reading contract.

## Non-goals

This release does not include MCP, repository search, AST/LSP symbol lookup,
`.gitignore` traversal, token counting, remote files, editing, or a daemon.
Those may be considered later only after the simple batch-read path has been
used and measured.

## CLI contract

```text
bread [flags] path[:start[-end]] ...
bread --request request.json [flags]
```

Paths are interpreted relative to the process working directory unless they
are absolute. The range parser uses the final `:<number>` or
`:<number>-<number>` suffix, so a Windows drive letter remains part of the
path. An omitted start means line 1; an omitted end means the end of the file.
Ranges and limits are inclusive and 1-based.

Important flags:

| Flag | Default | Meaning |
| --- | ---: | --- |
| `--format` | `text` | `text` or `json` |
| `--max-lines` | `2000` | maximum lines returned per file; `0` means unlimited |
| `--max-bytes` | `200000` | maximum aggregate content bytes; `0` means unlimited |
| `--no-line-numbers` | false | omit line-number prefixes in text output |
| `--request` | empty | read a JSON request from a file, or `-` for stdin |
| `--version` | false | print the binary version |

The JSON request is:

```json
{
  "files": [
    {"path": "src/a.go", "start": 1, "end": 80},
    {"path": "src/b.go"}
  ]
}
```

`start` and `end` are optional positive integers. CLI flags still control the
output format and limits when a JSON request is used.

## Result semantics

Every requested file produces one result in input order. A successful result
includes its path, requested range, actual returned range, total line count,
content byte count, and whether it was truncated by a line or byte limit. A
failed result includes a stable human-readable error and no content. Text
output uses clear per-file headers; JSON output contains the same metadata and
is valid even when some files fail.

The reader loads and validates each requested file concurrently, then applies
the aggregate byte budget in input order. This keeps I/O parallel while making
the result reproducible. If the budget cuts through a UTF-8 string, the content
is shortened to a valid UTF-8 boundary.

## Skill behavior

The skill is intentionally a navigation policy rather than a replacement for
an agent's native tools:

1. If the target files are unknown, search first.
2. If one known file is needed, use the normal dedicated read operation.
3. If two or more independent known files/ranges are needed, use one `bread`
   invocation when available; otherwise use one parallel tool turn or one
   shell command containing all reads.
4. Read sequentially only when the result of an earlier read determines the
   next target.
5. Keep batches relevant and bounded; do not dump a repository or generated
   directories merely because a batch command exists.

## Verification

The repository is complete for this design when `gofmt -l .` prints nothing,
`go vet ./...` succeeds, `go test ./...` succeeds, and manual smoke commands
demonstrate both positional and JSON requests.
