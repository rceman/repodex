# plan.md — Repodex (TS/TSX index + Search/Fetch/Serve over StdIO)

## 0) What we are building

**Repodex** is a small CLI tool that can:

1) **Index** a TypeScript/TSX codebase into a compact on-disk index (part 1).  
2) Expose that index via a simple agent-friendly interface:
   - **search** (candidates-only, ranked, limited)
   - **fetch** (bounded extraction of indexed chunks)
   - **serve --stdio** (JSONL protocol over stdin/stdout) (part 2)

Primary use case: a local “code search + snippet fetch” backend for an agent that can only send English queries, while the agent may accept Russian input and translate into English keywords client-side.

---

## 1) Goals and constraints

### Goals
- Deterministic, reproducible indexing for TS/TSX projects.
- Fast candidate retrieval (postings-based) and bounded snippet fetching.
- A robust `serve --stdio` mode suitable for long-running agent sessions.
- Safety: do not allow reading files outside the indexed root; avoid symlink escapes.

### Non-goals (for now)
- Full-text retrieval with advanced ranking (BM25, proximity, phrase queries).
- Semantic embeddings / vector search.
- Multi-language indexing (we do RU->EN query normalization on the client/agent side only).
- Incremental indexing (sync can be full rebuild for prototype).

---

## 2) CLI surface (user-facing)

### Commands
- `repodex init`
  - Creates `.repodex/` and writes default config + default ignore.
- `repodex status`
  - Reports whether the index exists and whether it is “dirty”.
- `repodex sync`
  - Rebuilds the entire index (prototype-friendly).
- `repodex search --q "..."`
  - Runs candidates-only ranked search.
- `repodex fetch --ids [..] --max-lines N`
  - Fetches bounded chunk text.
- `repodex serve --stdio`
  - Runs JSONL request/response protocol on stdin/stdout.

---

## 3) Part 1 — Indexing (TS/TSX)

### 3.1 Inputs (project root)
- Config file (JSON): `.repodex/config.json`
- Ignore file: `.repodex/ignore`
- Files on disk under `root/`

### 3.2 Scanner rules
- Walk the root directory, applying:
  - ignore dirs (from config exclude + ignore file)
  - include extensions (TS/TSX by default)
  - exclude `.d.ts`
  - max file size (code bytes cap from config)
- **Safety hardening (required):**
  - Skip symlinks during scanning (do not follow).
  - Do not index content via paths that could escape the root.

### 3.3 Tokenization / chunking
- Language plugin is selected by `ProjectType` in config (TS/TSX plugin).
- Chunking produces **ChunkEntry** records with:
  - `chunk_id`
  - `path` (relative, normalized with `/`)
  - `start_line`, `end_line`
  - `snippet` (short text preview)
- Tokenization is performed:
  - on chunk text for indexing
  - on query text for searching (same rules)

### 3.4 Index artifacts (on disk)
Stored under `.repodex/` (paths abstracted via internal store helpers), typically:
- `meta.json` (or equivalent): index version, counts, and a config hash
- `files.bin`: file entries
- `chunks.bin`: chunk entries
- `terms.bin`: term dictionary with df + postings offsets
- `postings.bin`: postings list (chunk ids)

### 3.5 “Dirty” logic
- `status` compares:
  - existence of required artifacts
  - current config hash vs stored meta hash
  - file stats (mtime/size) vs stored file entries
- If mismatch: `Dirty=true` and agent should `sync`.

---

## 4) Part 2 — Search/Fetch/Serve over the TS/TSX index

## 4.1 Search API (candidates-only)

### Data loaded for search
- `chunks` (for metadata + snippet)
- `terms` + `postings`
- config + language plugin (project type selection)

### Query tokenization
- Tokenize the query using the TS plugin tokenizer:
  - Use the same token rules as indexing.
  - Implementation detail: reuse the plugin tokenizer on the query string.

### Candidate collection
- For each unique query token:
  - lookup `TermInfo` in `terms`
  - iterate postings range → chunk ids
- Merge candidates across terms.

### Scoring
- Score per chunk: `score = sum(idf(term))` over matched query terms.
- `idf(term) = log(1 + N/df)` where:
  - `N` = number of indexed chunks
  - `df` = document frequency for the term (from `terms`)

### Ranking + caps
- Sort descending by score.
- Enforce `max_per_file = 2` (hard internal cap).
- Return `top_k` results, default `20`, max `20`.

### Output shape (per result)
- `chunk_id`
- `path`
- `start_line`, `end_line`
- `score`
- `snippet` (from chunk entry)
- `why`: matched terms (unique terms that contributed)

---

## 4.2 Fetch API (bounded extraction)

### Input limits
- `ids`: array of chunk ids, process at most `5`
- `max_lines`: default `120`, cap at `120`

### Resolution logic
- chunk id → chunk entry → `path + line range`
- read file from disk and return line-numbered strings:
  - `"N| <line contents>"`

### Output bounding rule
- If `(end_line - start_line + 1) > max_lines`:
  - return the **first max_lines lines** of the chunk range (fixed rule, deterministic)
- Normalize line endings:
  - treat CRLF and CR as LF for consistent numbering

### Filesystem safety (required)
- Reject chunk paths if any of these are true:
  - absolute path
  - traversal segments (`..`)
  - resolves via symlinks to a location outside `root`
- Recommended approach:
  - evaluate real root path (resolve symlinks on root itself)
  - join + clean relative chunk path
  - evaluate symlinks for the final joined path
  - ensure final resolved path is within resolved root prefix

---

## 4.3 Serve — `serve --stdio` JSONL protocol

### Transport
- **One request line → one response line**
- Each line is a single JSON object
- Requests and responses are newline-delimited

### Operations
- `status`
- `sync`
- `search`
- `fetch`

### Robustness requirements (P1/P3 hardening)
- The server **must not exit** on:
  - invalid JSON
  - unknown op
  - oversized request line
- Instead: emit `{ ok:false, op:"", error:"..." }` and continue reading.

### Request size limit (P1)
- Use a size-limited line reader (not a plain Scanner default token cap).
- If a line exceeds the limit:
  - emit `"request too large"`
  - discard until newline
  - continue serving

### In-process index cache (P2)
- For `serve --stdio`, load index artifacts once and reuse:
  - cfg + cfg bytes
  - plugin
  - chunks + chunkMap
  - terms + postings
- Invalidate cache after successful `sync`.
- Thread safety:
  - protect cache with a mutex (serve is usually single-threaded, but keep it safe).

### JSON request schema (recommended)
Common fields:
- `op` (string)

For search:
- `q` (string, required)
- `top_k` (int, optional; default 20; max 20)

For fetch:
- `ids` (array of uint32, required; only first 5 processed)
- `max_lines` (int, optional; default 120; cap 120)

Responses:
- Success: `{ "ok": true, "op": "<op>", "data": <payload> }`
- Error: `{ "ok": false, "op": "", "error": "<message>" }`

---

## 5) Agent usage rules (documentation-level)

### Recommended agent flow
1) `status`
2) if `dirty=true` → `sync`
3) `search` (candidates-only)
4) `fetch` (bounded excerpts for top candidates)

### Russian queries support
- The tool itself expects English query text in `search.q`.
- If user query is Russian:
  - agent extracts English keywords (translate + compress to code-ish tokens)
  - sends only the English keyword query to repodex
- This keeps the index/tokenizer language-consistent and avoids multi-language complexity inside repodex.

---

## 6) Quality bar / acceptance checks

### Functional
- `sync` produces all index artifacts.
- `status` correctly reports indexed/dirty state.
- `search` returns ranked candidates with `max_per_file` enforcement.
- `fetch` respects ids/max_lines limits and line numbering.
- `serve --stdio` supports all ops and continues on bad inputs.

### Safety
- Scanner skips symlinks.
- Fetch cannot read outside root via traversal or symlink escape.

### Performance (baseline)
- In serve mode, repeated `search`/`fetch` does not reload index artifacts each time (cache works).

---

## 7) Next steps beyond current scope
- Better ranking: BM25, field boosts (filename/imports/exports), proximity.
- Incremental sync: only re-index changed files.
- Multi-language pipeline: RU query normalization with richer heuristics, optional bilingual stopword handling.
- Optional: expose per-term diagnostics (df/idf contributions) for explainability.
