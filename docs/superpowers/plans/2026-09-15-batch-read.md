# Batch Read Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standard-library-only Go `bread` binary and an
`efficient-codebase-navigation` skill that reduces avoidable sequential file
read turns.

**Architecture:** Keep file request parsing, reading, range slicing, budget
application, and result modeling in `internal/read`. Keep process flags,
request loading, rendering, and exit-code policy in `cmd/bread`. The skill is a
standalone Markdown artifact under `skills/` and documents the decision tree
for unknown, single, multiple, and dependent reads.

**Tech Stack:** Go standard library; JSON; Markdown.

**Spec:** `docs/superpowers/specs/2026-09-15-batch-read-design.md`

## Global Constraints

- Use only the Go standard library; do not add runtime dependencies.
- Ranges are 1-based and inclusive.
- Parallelize independent file I/O but preserve input order in every output format.
- Keep aggregate output bounded by `--max-bytes` unless the user explicitly sets it to `0`.
- Return partial results and a non-zero process status when any requested file fails.
- Do not add MCP, search, AST/LSP, editing, daemon, or remote-file behavior.

### Task 1: Repository scaffolding and core request model

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `README.md`
- Create: `cmd/bread/main.go`
- Create: `internal/read/request.go`
- Create: `internal/read/request_test.go`

**Interfaces:**
- `internal/read.ParseSpec(string) (FileRequest, error)` parses a positional
  `path`, `path:start`, or `path:start-end` request.
- `internal/read.FileRequest` contains `Path string`, `Start int`, and `End int`.
- `cmd/bread` owns process-level flags and calls the internal package; no CLI
  parsing belongs in `internal/read`.

- [ ] **Step 1: Create the Go module and repository metadata.**

  Use module path `bread` because this repository is initially local and has no
  verified remote origin. Set the minimum Go version to the installed major
  version's stable language baseline without importing third-party modules.
  Ignore compiled binaries, coverage output, and editor files.

- [ ] **Step 2: Write failing parser tests.**

  Cover an unqualified path, a start-only range, an inclusive start/end range,
  a Windows drive-letter path with a range, zero/negative values, reversed
  ranges, and malformed suffixes. Assert the exact `FileRequest` values and
  that invalid forms return errors.

- [ ] **Step 3: Implement `FileRequest` and `ParseSpec`.**

  Match only a final numeric suffix so `C:\\repo\\a.go:4-8` parses as path
  `C:\\repo\\a.go`, start `4`, end `8`. Treat an omitted range as zero-valued
  bounds for later normalization. Reject empty paths and non-positive bounds.

- [ ] **Step 4: Run the focused parser test.**

  Run `go test ./internal/read -run TestParseSpec -v` and expect all cases to
  pass.

- [ ] **Step 5: Commit the scaffolding and parser.**

  Run `git add go.mod .gitignore README.md cmd/bread/main.go internal/read` and
  commit with `feat: scaffold bread request parsing`.

### Task 2: Core reader, line ranges, and deterministic budgets

**Files:**
- Create: `internal/read/reader.go`
- Create: `internal/read/reader_test.go`

**Interfaces:**
- `internal/read.Request` contains `Files []FileRequest`, `MaxLines int`, and
  `MaxBytes int64`.
- `internal/read.Result` contains path/range metadata, `Content string`,
  `Bytes int`, `TotalLines int`, `Truncated bool`, and `Error string`.
- `internal/read.Read(ctx context.Context, req Request) []Result` reads all
  requests concurrently and returns exactly one result per input request.

- [ ] **Step 1: Write failing reader tests.**

  Use `t.TempDir()` fixtures to cover: full-file reads, inclusive subranges,
  start past EOF, maximum lines, aggregate byte truncation across two files,
  invalid UTF-8, NUL-containing binary data, missing files, duplicate paths,
  and preservation of input order despite concurrent reads.

- [ ] **Step 2: Implement file loading and validation.**

  Load each file in a goroutine, reject directories, NUL bytes, and invalid
  UTF-8, split text into logical lines without inventing an extra line after a
  final newline, and record errors in the corresponding result.

- [ ] **Step 3: Implement inclusive line slicing.**

  Normalize zero bounds to the file's first/last line, clamp the end to EOF,
  return an empty successful result when the requested start is past EOF, and
  mark `Truncated` when `MaxLines` or `MaxBytes` removes otherwise-selected
  content.

- [ ] **Step 4: Implement deterministic aggregate byte budgeting.**

  After all reads complete, walk successful results in request order and spend
  the remaining byte budget. Preserve valid UTF-8 when taking a prefix and
  retain metadata that distinguishes range selection from budget truncation.

- [ ] **Step 5: Run the core reader tests and vet.**

  Run `go test ./internal/read -v` followed by `go vet ./...`; expect both to
  succeed.

- [ ] **Step 6: Commit the core reader.**

  Run `git add internal/read` and commit with `feat: add bounded concurrent file reader`.

### Task 3: Text and JSON renderers

**Files:**
- Create: `internal/read/render.go`
- Create: `internal/read/render_test.go`

**Interfaces:**
- `internal/read.RenderText(io.Writer, []Result, bool) error` writes headers,
  metadata, and optional line-number prefixes.
- `internal/read.RenderJSON(io.Writer, []Result) error` writes a stable JSON
  document containing all result metadata and content/errors.

- [ ] **Step 1: Write failing rendering tests.**

  Assert that text output has one header per result, preserves result order,
  prefixes selected lines with their original numbers by default, and omits
  prefixes when requested. Decode JSON and assert it preserves metadata,
  content, errors, and truncation flags.

- [ ] **Step 2: Implement the text renderer.**

  Use a clear `=== path ... ===` header, print errors as an item-local error
  line, and never let one result suppress later results. Keep the renderer
  independent of terminal color or width.

- [ ] **Step 3: Implement the JSON renderer.**

  Define explicit JSON structs with stable field names and encode through
  `encoding/json.Encoder`. Do not emit logs or diagnostics on stdout.

- [ ] **Step 4: Run rendering tests.**

  Run `go test ./internal/read -run 'TestRender' -v` and expect all cases to
  pass.

- [ ] **Step 5: Commit the renderers.**

  Run `git add internal/read/render.go internal/read/render_test.go` and commit
  with `feat: add text and json batch output`.

### Task 4: CLI request loading, flags, and integration tests

**Files:**
- Modify: `cmd/bread/main.go`
- Create: `cmd/bread/main_test.go`
- Modify: `README.md`

**Interfaces:**
- Positional arguments are converted with `read.ParseSpec`.
- `--request FILE` decodes `{"files":[{"path":...,"start":...,"end":...}]}`;
  `FILE=-` reads stdin.
- Exit status is `0` when every result succeeds, `1` when any result has an
  error, and `2` for invalid flags or malformed requests.

- [ ] **Step 1: Write failing CLI tests.**

  Invoke the command with `go run ./cmd/bread` from a fixture directory and
  cover positional text output, JSON request output, malformed JSON, missing
  paths, invalid format, and `--no-line-numbers`. Assert stdout content and
  exact exit status.

- [ ] **Step 2: Implement flags and request loading.**

  Add `--format`, `--max-lines`, `--max-bytes`, `--no-line-numbers`,
  `--request`, and `--version`. Reject negative limits and unknown formats.
  Require at least one file request from either arguments or JSON.

- [ ] **Step 3: Connect the CLI to the reader and renderers.**

  Build a `read.Request`, call `read.Read` with a background context, render
  exactly once to stdout, and return the documented status. Send usage and
  malformed-input diagnostics to stderr.

- [ ] **Step 4: Document usage and JSON input.**

  Include install/build/test commands, positional examples, JSON examples,
  range semantics, default limits, exit statuses, and explicit non-goals in
  `README.md`.

- [ ] **Step 5: Run integration tests and the full Go gate.**

  Run `go test ./...`, `go vet ./...`, and `gofmt -l .`; the first two must
  succeed and the last command must print nothing.

- [ ] **Step 6: Commit the CLI.**

  Run `git add cmd/bread README.md` and commit with `feat: expose bread batch read cli`.

### Task 5: Efficient navigation skill and evaluation prompts

**Files:**
- Create: `skills/efficient-codebase-navigation/SKILL.md`
- Create: `evals/evals.json`

**Interfaces:**
- The skill frontmatter name is `efficient-codebase-navigation`.
- The skill refers to the installed executable as `bread` and treats it as an
  optional capability with a documented fallback.
- `evals/evals.json` contains three realistic prompts: known independent files,
  unknown paths requiring search, and dependency-driven sequential reading.

- [ ] **Step 1: Write the skill.**

  Include the decision tree, why batching lowers round trips, command examples,
  output-budget guidance, fallback behavior for agents without `bread`, and
  cases where sequential reads are correct. Keep it under 500 lines and put
  trigger conditions in the description frontmatter.

- [ ] **Step 2: Write the prompt eval set.**

  Add the three prompts with expected behavior descriptions but no brittle
  assertions. The expected behaviors must distinguish a single batch from
  sequential reads and must not require `bread` when it is unavailable.

- [ ] **Step 3: Run a structural skill check.**

  Verify the frontmatter has `name` and `description`, the file is below 500
  lines, the examples use `skills/` paths correctly, and `jq empty evals/evals.json`
  succeeds.

- [ ] **Step 4: Commit the skill.**

  Run `git add skills evals/evals.json` and commit with `feat: add efficient codebase navigation skill`.

### Task 6: Final verification and handoff

**Files:**
- No source changes expected; only fix issues found by verification.

- [ ] **Step 1: Run the full verification suite.**

  Run `gofmt -w cmd internal`, `go test ./...`, `go vet ./...`, and
  `gofmt -l .`. Also run smoke commands for one positional batch and one JSON
  request.

- [ ] **Step 2: Inspect the repository state.**

  Run `git status --short`, `git log --oneline --decorate -6`, and verify the
  new repository is independent of the Xross and CrossCopy worktrees.

- [ ] **Step 3: Record the final evidence.**

  Report the absolute repo path, commands that passed, CLI examples, and the
  exact skill path. Do not claim a skill-quality benchmark was run; this plan
  only creates the eval prompts, and actual with/without-skill runs require a
  compatible evaluator.
