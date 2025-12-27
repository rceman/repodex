# repodex

Small CLI for indexing TypeScript/TSX or Go codebases and serving simple search/fetch operations.

## CLI

- `repodex init [--force]` – create `.repodex/` with default config and ignore (auto-detects profiles when possible).
- `repodex status [--json]` – show whether the index exists and if it is dirty.
- `repodex sync` – rebuild the on-disk index.
- `repodex search --q "<query>" [--top_k N]` – run ranked keyword search (caps: top_k max 20).
- `repodex fetch --ids 1,2,... [--max_lines N]` – fetch chunk text for up to 5 ids (max_lines default/capped at 120).
- `repodex serve --stdio` – start the JSONL stdio protocol server.

## Profiles

Supported profiles: `ts_js`, `go`. `repodex init` auto-detects profiles based on repo contents when possible.

See the plan for details: [plan.md](plan.md).
- StdIO protocol: [docs/stdio_protocol.md](docs/stdio_protocol.md)
