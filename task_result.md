Task ID: task 192803_241225
BASE_COMMIT: 500d54eb6565ce344ece8df44f47e0eeb80ec331

## Summary
- Added deserializers for chunk and term/posting files to reload the TS/TSX index.
- Implemented search scoring and fetch line extraction with tests covering ranking, per-file caps, and truncation.
- Added stdio serve mode with CLI flag, reusable status helper, and agent-facing usage notes.

## Files changed and deleted (task-scoped)
A\tdocs/agent_rules.md
M\tinternal/app/app.go
M\tinternal/cli/parse.go
A\tinternal/fetch/fetch.go
A\tinternal/fetch/fetch_test.go
A\tinternal/index/deserialize_chunks.go
A\tinternal/index/deserialize_terms_postings.go
A\tinternal/search/search.go
A\tinternal/search/search_test.go
A\tinternal/serve/stdio.go
M\ttask_result.md

## Commands executed
- `ls` (exit 0)
- `find .. -name AGENTS.md -print` (exit 0)
- `git rev-parse HEAD` (exit 0)
- `cat <<'EOF' > task_result.md` (exit 0)
- `ls internal` (exit 0)
- `ls internal/index` (exit 0)
- `sed -n '1,200p' internal/index/types.go` (exit 0)
- `sed -n '1,200p' internal/index/serialize.go` (exit 0)
- `sed -n '1,200p' internal/index/deserialize_min.go` (exit 0)
- `sed -n '1,200p' internal/app/app.go` (exit 0)
- `sed -n '200,400p' internal/app/app.go` (exit 0)
- `sed -n '1,200p' internal/cli/parse.go` (exit 0)
- `ls internal/serve` (exit 2)
- `ls internal/lang` (exit 0)
- `sed -n '1,200p' internal/lang/plugin.go` (exit 0)
- `sed -n '1,200p' internal/lang/factory/factory.go` (exit 0)
- `ls internal/lang/ts` (exit 0)
- `sed -n '1,200p' internal/lang/ts/plugin.go` (exit 0)
- `sed -n '1,240p' internal/lang/ts/tokenize.go` (exit 0)
- `sed -n '1,200p' cmd/repodex/main.go` (exit 0)
- `gofmt -w $(go list -f '{{.Dir}}' ./...)` (exit 0)
- `go test ./...` (exit 0)
- `go build -v ./cmd/repodex` (exit 0)
- `git status --short` (exit 0)
- `rm -f repodex` (exit 0)
- `git status --short` (exit 0)
- `sed -n '1,240p' internal/search/search.go` (exit 0)
- `sed -n '1,240p' internal/fetch/fetch.go` (exit 0)
- `sed -n '1,200p' internal/serve/stdio.go` (exit 0)
- `sed -n '1,200p' internal/fetch/fetch_test.go` (exit 0)
- `git diff --name-status "500d54eb6565ce344ece8df44f47e0eeb80ec331"..HEAD` (exit 0)
- `git add .` (exit 0)
- `git status --short` (exit 0)
- `git commit -m "Add search, fetch, and serve interfaces"` (exit 0)
- `git diff --name-status "500d54eb6565ce344ece8df44f47e0eeb80ec331"..HEAD` (exit 0)
- `git status --short` (exit 0)
- `nl -ba internal/index/deserialize_chunks.go` (exit 0)
- `nl -ba internal/index/deserialize_terms_postings.go` (exit 0)
- `nl -ba internal/search/search.go` (exit 0)
- `nl -ba internal/search/search_test.go` (exit 0)
- `nl -ba internal/fetch/fetch_test.go` (exit 0)
- `nl -ba internal/serve/stdio.go` (exit 0)
- `nl -ba internal/app/app.go` (exit 0)
- `nl -ba internal/cli/parse.go` (exit 0)
- `nl -ba docs/agent_rules.md` (exit 0)
- `nl -ba internal/fetch/fetch.go` (exit 0)
- `git add task_result.md` (exit 0)
- `git status --short` (exit 0)
- `git commit --amend --no-edit` (exit 0)
- `git status --short` (exit 0)
- `cat <<'EOF' > task_result.md` (exit 0)
- `git add task_result.md` (exit 0)
- `git commit --amend --no-edit` (exit 0)
- `git status --short` (exit 0)
