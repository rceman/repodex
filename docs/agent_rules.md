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
