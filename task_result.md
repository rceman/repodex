Task ID: task 213201_241225
BASE_COMMIT: 1d6f9506bf80f4f5c70d45d9379867928fb80e90
## Summary
- Added bounded stdio request handling with recoverable error responses and an in-process index cache invalidated on sync.
- Hardened scanning and fetch path resolution to skip symlinks and prevent traversal outside the project root.
- Introduced reusable search and fetch helpers for cached index data.
- Documented the stdio protocol and linked it from the README.

## Files changed and deleted (task-scoped)
M	README.md
A	docs/stdio_protocol.md
M	internal/fetch/fetch.go
M	internal/scan/scan.go
M	internal/search/search.go
A	internal/serve/cache.go
M	internal/serve/stdio.go
A	task_result.md

## Commands executed
- gofmt -w $(go list -f '{{.Dir}}' ./...) && echo "gofmt done" (exit 0)
- go test ./... (exit 0)
- go build -v ./cmd/repodex (exit 0)

