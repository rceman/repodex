# plan.md - Repodex (TS/TSX index + Search/Fetch/Serve over StdIO)

## 0) What we are building

**Repodex** is a small CLI tool that can:

1) Index a TypeScript/TSX codebase into a compact on-disk index (Part 1).  
2) Expose that index via a simple agent-friendly interface (Part 2):
   - search (candidates-only, ranked, limited)
   - fetch (bounded extraction of indexed chunks)
   - serve --stdio (JSONL protocol over stdin/stdout)

Primary use case: a local "code search + snippet fetch" backend for an agent that only sends English queries, while the agent may accept Russian input and translate into English keywords client-side.

## 1) Goals and constraints

### Goals
- Deterministic, reproducible indexing for TS/TSX projects.
- Fast candidate retrieval (postings-based) and bounded snippet fetching.
- A robust `serve --stdio` mode suitable for long-running agent sessions.
- Safety: do not allow reading files outside the indexed root; avoid symlink escapes.

### Non-goals (for now)
- Advanced ranking (BM25, proximity, phrase queries, field boosts).
- Semantic embeddings or vector search.
- Multi-language indexing (RU -> EN query normalization happens client-side only).
- Incremental indexing (sync can be full rebuild for the prototype).

## 2) CLI surface (user-facing)

### Commands
- `repodex init`
  - Creates `.repodex/` and writes default config + default ignore.
- `repodex status`
  - Reports whether the index exists and whether it is "dirty".
- `repodex sync`
  - Rebuilds the entire index (prototype-friendly full rebuild).
- `repodex search --q "..."`
  - Runs candidates-only ranked search.
- `repodex fetch --ids [..] --max-lines N`
  - Fetches bounded chunk text.
- `repodex serve --stdio`
  - Runs JSONL request/response protocol on stdin/stdout.

## 3) Part 1 - Indexing (TS/TSX)

### 3.1 Inputs (project root)
- Config file (JSON): `.repodex/config.json`
- Ignore file: `.repodex/ignore`
- Files on disk under `root/`

### 3.2 Scanner rules
- Walk the root directory, applying:
  - ignore dirs (from config exclude + ignore file)
  - include extensions (TS/TSX by default)
  - exclude `.d.ts`
  - max file size cap (from config)
- Safety hardening:
  - Skip symlinks during scanning (do not follow) so scanning cannot traverse outside root via symlinks.

### 3.3 Line ending normalization (important for stable line numbering)
- When reading source files for indexing and chunking:
  - Normalize line endings so CRLF and CR are treated as LF (`\r\n` and `\r` become `\n`).
- Rationale:
  - Ensures consistent `start_line` and `end_line` computation across platforms (Windows vs Unix).

### 3.4 Tokenization and chunking
- Language plugin is selected by `ProjectType` in config (TS/TSX plugin).
- Chunking produces **ChunkEntry** records with:
  - `chunk_id`
  - `path` (relative, normalized with `/`)
  - `start_line`, `end_line`
  - `snippet` (short preview text)
- Tokenization is performed:
  - on chunk text for indexing
  - on query text for searching (same rules)

### 3.5 Index artifacts (on disk)
Stored under `.repodex/` (paths abstracted via internal store helpers), typically:
- `meta.json` (or equivalent): index version, counts, and config hash
- `files.bin`: file entries (path, size, mtime, etc.)
- `chunks.bin`: chunk entries (chunk metadata, snippet, line ranges)
- `terms.bin`: term dictionary with df + postings offsets
- `postings.bin`: postings list (chunk ids)

### 3.6 Config hashing (exact bytes)
- The config hash stored in meta must be derived from the raw config file bytes exactly as read from disk.
- Rationale:
  - Avoids mismatches caused by re-marshaling, whitespace, key ordering, or formatting changes.
  - Dirty detection should reflect what is actually on disk.

### 3.7 Dirty logic
- `status` compares:
  - existence of required artifacts
  - current config hash vs stored meta hash
  - file stats (mtime and size) vs stored file entries
- If mismatch: `Dirty=true` and the agent should call `sync`.

## 4) Part 2 - Search/Fetch/Serve over the TS/TSX index

## 4.1 Search API (candidates-only)

### Data loaded for search
- config + language plugin (project type selection)
- `chunks` (metadata and snippet)
- `terms` + `postings`

### Query tokenization
- Tokenize the query using the same TS plugin tokenizer rules as indexing:
  - Reuse the plugin tokenizer on the query string (same token rules and stopwords).

### Candidate collection
- For each unique query token:
  - lookup `TermInfo` in `terms`
  - iterate postings range to collect chunk ids
- Merge candidates across terms.

### Scoring
- Score per chunk: `score = sum(idf(term))` over matched query terms.
- `idf(term) = log(1 + N/df)` where:
  - `N` is the number of indexed chunks
  - `df` is document frequency for the term

### Ranking and caps
- Sort descending by score.
- Enforce `max_per_file = 2` (hard internal cap).
- Return `top_k` results:
  - default 20
  - maximum 20

### Output shape (per result)
- `chunk_id`
- `path`
- `start_line`, `end_line`
- `score`
- `snippet` (from chunk entry)
- `why`: matched terms that contributed (unique)

## 4.2 Fetch API (bounded extraction)

### Input limits
- `ids`: process at most 5 chunk ids
- `max_lines`: default 120, cap at 120

### Resolution logic
- chunk id -> chunk entry -> `path + line range`
- read file from disk and return line-numbered strings:
  - `"N| <line contents>"`

### Output bounding rule
- If `(end_line - start_line + 1) > max_lines`:
  - return the first `max_lines` lines of the chunk range (fixed and deterministic)

### Line ending normalization
- Treat CRLF and CR as LF for consistent numbering, same as indexing.

### Filesystem safety (required)
- Reject chunk paths if any of these are true:
  - absolute path
  - traversal segments (`..`)
  - resolves via symlinks to a location outside `root`
- Recommended approach:
  - resolve real root path (EvalSymlinks on root itself)
  - clean and join relative chunk path under root
  - EvalSymlinks on the candidate path
  - ensure the resolved path is within the resolved root prefix

## 4.3 Serve - `serve --stdio` JSONL protocol

### Transport
- One request line -> one response line.
- Each line is a single JSON object.
- Requests and responses are newline-delimited.

### Operations
- `status`
- `sync`
- `search`
- `fetch`

### Robustness requirements
The server must not exit on:
- invalid JSON
- unknown op
- oversized request line
- other validation errors

Instead it must emit an error response and continue reading subsequent lines.

### Request size limit
- Do not rely on bufio.Scanner default token limits.
- Enforce a per-line MaxRequestBytes (for example 1 MiB).
- If a line exceeds the limit:
  - emit an error response with `"request too large"`
  - discard until newline
  - continue serving

### Error response semantics
- For errors that cannot be associated with a supported operation, respond with:
  - `{ "ok": false, "op": "", "error": "..." }`
- For success:
  - `{ "ok": true, "op": "<op>", "data": <payload> }`

### In-process index cache
- In `serve --stdio`, load index artifacts once and reuse for subsequent `search` and `fetch`:
  - cfg + cfg bytes
  - plugin
  - chunks + chunkMap
  - terms + postings
- Invalidate cache after successful `sync`.

### Ignore loading semantics
- When loading ignore rules for scanning:
  - If ignore file is missing, treat as empty and continue.
  - If ignore file exists but cannot be loaded (parse/read errors), surface the error (do not silently ignore).
- Rationale:
  - Missing ignore is a normal case.
  - Other ignore errors are actionable and should not be hidden.

### JSON request schema (recommended)
Common fields:
- `op` (string, required)

For search:
- `q` (string, required, English)
- `top_k` (int, optional; default 20; cap 20)

For fetch:
- `ids` (array of uint32, required; only first 5 processed)
- `max_lines` (int, optional; default 120; cap 120)

## 5) Agent usage rules (documentation-level)

### Recommended agent flow
1) `status`
2) if `dirty=true` -> `sync`
3) `search` (candidates-only)
4) `fetch` (bounded excerpts for top candidates)

### Russian queries support
- Repodex expects English query text in `search.q`.
- If user query is Russian:
  - agent translates or extracts English keywords (compressed, code-ish)
  - sends only the English query to repodex
- This keeps tokenizer language-consistent and avoids multi-language complexity inside repodex.

## 6) Quality bar and acceptance checks

### Functional
- `sync` produces all index artifacts.
- `status` correctly reports indexed and dirty state.
- `search` returns ranked candidates with `max_per_file` enforcement.
- `fetch` respects ids and max_lines limits and stable line numbering.
- `serve --stdio` supports all ops and continues on bad inputs.

### Safety
- Scanner skips symlinks.
- Fetch cannot read outside root via traversal or symlink escape.

### Performance (baseline)
- In serve mode, repeated `search` and `fetch` does not reload index artifacts each time (cache works).

## 7) Next steps beyond current scope
- Better ranking: BM25, field boosts (filename, imports, exports), proximity.
- Incremental sync: only re-index changed files.
- Multi-language pipeline: richer RU query normalization and optional bilingual stopword handling.
- Better snippets: highlight terms, show more context, or structured snippet generation.
