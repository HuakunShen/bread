---
name: efficient-codebase-navigation
description: Use whenever inspecting, reviewing, debugging, modifying, or understanding a codebase may require reading two or more files or line ranges. Prefer one bounded batch read or one parallel read turn for independent targets, use search before reading unknown paths, and read sequentially only when an earlier result determines the next target. When the `bread` CLI is available, use it for known multi-file or multi-range reads instead of issuing separate sequential shell reads.
---

# Efficient codebase navigation

The goal is to give the model the relevant context with fewer
model-to-tool-to-model turns. A batch is useful when the targets are already
known and independent; it is not a reason to dump a repository or to hide a
dependency between reads.

## Choose the read shape first

Classify the next read before calling a tool:

1. **Unknown target:** search for the symbol, path, or configuration first.
   Use `rg`, a file search tool, or the host's equivalent. Once the search
   identifies the relevant paths and ranges, batch the context reads that are
   independent.
2. **One known target:** use the host's normal dedicated file-read operation
   with a focused range. `bread` is unnecessary for one file.
3. **Two or more known independent targets:** prefer one `bread` invocation.
   If `bread` is not installed, issue all native read calls in one parallel
   tool turn. If the host cannot parallelize tool calls, make one shell command
   containing all targeted reads.
4. **Dependent target:** read sequentially only when the content of the first
   result determines the path, symbol, or range needed next. This is correct
   exploration, not an inefficiency to eliminate.

Do not turn a search into a second round for every match when the relevant
context can be selected and read together. Do not batch blindly when the first
result is genuinely needed to discover the next question.

## Use `bread` for known independent reads

Check whether the executable is available in the current environment before
using it:

```bash
command -v bread
```

For positional requests, ranges are 1-based and inclusive:

```bash
bread --max-lines 800 --max-bytes 200000 \
  internal/read/reader.go:1-220 \
  internal/read/render.go:1-160 \
  cmd/bread/main.go:1-220
```

The command reads files concurrently and emits results in the order supplied.
Text output includes original line numbers, file headers, byte counts, and
truncation markers. Read the complete batch result before deciding what to do
next; do not mentally turn each file into a separate tool turn.

For paths or ranges that are awkward to express on a command line, use JSON:

```json
{
  "files": [
    {"path": "internal/read/reader.go", "start": 1, "end": 220},
    {"path": "internal/read/render.go", "start": 1, "end": 160}
  ]
}
```

Then run:

```bash
bread --format json --request request.json
```

The JSON result contains one item per request, including an error item for a
missing, binary, or invalid-UTF-8 file. A non-zero exit status does not discard
the successful items; inspect the returned items before retrying.

## Keep the batch useful

- Include only files that answer the current question. A batch is a context
  boundary, not a repository archive.
- Prefer the smallest ranges that preserve the contract and implementation
  relationship. Read imports, types, the relevant function, and nearby tests
  when those are what the decision needs.
- Keep the default line and byte limits unless the task requires more context.
  If you raise a limit, do it deliberately and explain why the extra context
  is needed.
- Exclude generated output, build directories, dependency trees, and unrelated
  vendored code unless the task explicitly concerns them. Common examples are
  `target/`, `node_modules/`, `dist/`, and generated protobuf output.
- Preserve input order so the model can compare related files predictably.
- After one batch, reason once over the complete result. Only issue a follow-up
  read for a concrete gap, not because another file happens to exist.

## Fallbacks by host

This skill does not require a new tool schema. `bread` is a portable optional
CLI, so it can be used from any shell-enabled agent host, but do not install it
mid-task just to satisfy the skill unless the user asked for installation.

- In a shell-first host such as Codex, combine the known reads in one
  `exec`/shell call, or call `bread` once.
- In a host with dedicated file tools such as OpenCode or Claude Code, send
  independent reads together in one parallel tool turn. Use `bread` when a
  single structured result, per-file ranges, or a shared output budget is more
  useful than separate tool results.
- In a host with neither `bread` nor parallel tool calls, use one carefully
  quoted shell command with explicit paths and line ranges. Set the working
  directory through the host rather than spending a separate `cd` turn.

Never claim that batching happened when the host actually performed separate
sequential calls. The important invariant is the model-turn boundary, not the
name of the tool.

## Before editing

Before changing code, batch-read the known files that define the behavior:
the caller, the owned implementation, the relevant contract or configuration,
and the focused tests. Then make the smallest coherent edit. If a search or a
read reveals that another file is required, add it to the next batch instead of
dribbling out one-file reads when the targets are already known.

Before declaring the task complete, run the narrowest relevant tests and inspect
the diff. Reading more files is not a substitute for testing the changed path.
