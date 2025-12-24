# StdIO protocol

Repodex `serve --stdio` exchanges one JSON object per line. Each request must fit on a single line and each response is emitted on its own line.

## Transport
- Input: one JSON object terminated by `\n`.
- Output: one JSON object terminated by `\n`.
- A request line that exceeds 1,048,576 bytes is rejected with an error response and processing continues.

## Operations

### Common fields
- `op` (string, required): one of `status`, `sync`, `search`, `fetch`.

### status
- Request: `{ "op": "status" }`
- Response: `{ "ok": true, "op": "status", "data": { ... } }`

### sync
- Request: `{ "op": "sync" }`
- Response: `{ "ok": true, "op": "sync", "data": { ... } }`
- Effect: rebuilds the index; the in-process cache is invalidated after a successful sync.

### search
- Request fields:
  - `q` (string, required): English query text.
  - `top_k` (int, optional): defaults to 20, maximum 20.
- Response: `{ "ok": true, "op": "search", "data": [ { "chunk_id": 1, ... } ] }`

### fetch
- Request fields:
  - `ids` (array of uint32, required): chunk ids to fetch; only the first 5 are processed.
  - `max_lines` (int, optional): defaults to 120 and capped at 120.
- Response: `{ "ok": true, "op": "fetch", "data": [ { "chunk_id": 1, "lines": ["10| const x = 1"] } ] }`

## Error responses
- Unknown op: `{ "ok": false, "op": "", "error": "unknown op" }`
- Invalid JSON: `{ "ok": false, "op": "", "error": "invalid request: <details>" }`
- Oversize request line: `{ "ok": false, "op": "", "error": "request too large" }`
- Other validation failures include a descriptive `error` field and keep the server alive.

## Examples

### Status then sync
```
{ "op": "status" }
{ "op": "sync" }
```
Sample responses:
```
{"ok":true,"op":"status","data":{...}}
{"ok":true,"op":"sync","data":{...}}
```

### Search then fetch
```
{ "op": "search", "q": "create websocket server", "top_k": 5 }
{ "op": "fetch", "ids": [1, 4], "max_lines": 80 }
```
Sample responses:
```
{"ok":true,"op":"search","data":[{"chunk_id":1,"path":"src/api.ts",...}]}
{"ok":true,"op":"fetch","data":[{"chunk_id":1,"lines":["100| export function start() {", ...]}]}
```

### Error samples
```
{ invalid json
{ "op": "bogus" }
{ "op": "search", "q": "x" <very long padding to exceed limit> }
```
Sample responses:
```
{"ok":false,"op":"","error":"invalid request: unexpected character..."}
{"ok":false,"op":"","error":"unknown op"}
{"ok":false,"op":"","error":"request too large"}
```

## Query language
- The stdio layer expects English tokens. If a client accepts Russian queries, the client should translate them to English keywords before sending.
