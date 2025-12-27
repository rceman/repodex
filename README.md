# repodex

Small CLI for indexing TypeScript/TSX or Go codebases and serving simple search/fetch operations.

## CLI

- `repodex init [--force]` – create `.repodex/` with default config and ignore (auto-detects profiles when possible).
- `repodex status [--json]` – show whether the index exists and if it is dirty.
- `repodex sync` – rebuild the on-disk index.
- `repodex search --q "<query>" [--top_k N] [--score] [--no-format] [--json] [--color auto|always|never] [--no-color]` ??" run ranked keyword search (caps: top_k max 20).
- `repodex fetch --ids 1,2,... [--max_lines N]` – fetch chunk text for up to 5 ids (max_lines default/capped at 120).
- `repodex serve --stdio` – start the JSONL stdio protocol server.

## Profiles

Supported profiles: `ts_js`, `go`. `repodex init` auto-detects profiles based on repo contents when possible.

## Search Output

Default search output (non-JSON) is grouped by `why`, then by file, then by chunk. Snippets are emitted with indentation.

```
>why=profile
-internal/app/app.go
 @3:1-31!44
  "github.com/memkit/repodex/internal/profile"
```

Use `--no-format` to remove indentation and escape snippet lines starting with `>`, `-`, or `@` by prefixing `\`. Use `--score` to include `~<score>` on the hit line. Use `--json` for the raw JSON array. Color output defaults to `--color auto`, respects `NO_COLOR`, and is disabled for `--no-format` and `--json`.

See the plan for details: [plan.md](plan.md).
- StdIO protocol: [docs/stdio_protocol.md](docs/stdio_protocol.md)
