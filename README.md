# repodex

Small CLI for indexing a TypeScript/TSX codebase and serving simple search/fetch operations.

## CLI

- `repodex init [--force]` – create `.repodex/` with default config and ignore.
- `repodex status [--json]` – show whether the index exists and if it is dirty.
- `repodex sync` – rebuild the on-disk index.
- `repodex search --q "<query>" [--top_k N]` – run ranked keyword search (caps: top_k max 20).
- `repodex fetch --ids 1,2,... [--max_lines N]` – fetch chunk text for up to 5 ids (max_lines default/capped at 120).
- `repodex serve --stdio` – start the JSONL stdio protocol server.

See the plan for details: [plan.md](plan.md).
- StdIO protocol: [docs/stdio_protocol.md](docs/stdio_protocol.md)
