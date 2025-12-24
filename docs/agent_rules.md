# Agent rules

- If `status.dirty` is true, run `sync` before searching.
- Perform `search` first, then call `fetch` for specific chunk ids you want to inspect.
- Russian queries are handled by the agent: derive English keywords first and send only English text to the tool.

## Stdio operations

All requests and responses are JSON objects on a single line.

- `{"op":"status"}` returns the structured status payload.
- `{"op":"sync"}` rebuilds the index and returns an updated status payload.
- `{"op":"search","q":"tokens","top_k":20}` returns ranked candidates with reasons.
- `{"op":"fetch","ids":[1,2],"max_lines":120}` returns bounded line excerpts.

Example interaction:

```
{"op":"status"}
{"op":"search","q":"router handler","top_k":10}
{"op":"fetch","ids":[2],"max_lines":40}
```

Malformed JSON lines return `{"ok":false,"op":"","error":"..."}` because the request cannot be parsed.

## Limits

- `search.top_k` defaults to 20 and is clamped to 20.
- `search.max_per_file` defaults to 2 results per file.
- `fetch.ids` is trimmed to the first 5 IDs when more are requested.
- `fetch.max_lines` defaults to 120 and is clamped to 120 lines.
