Task ID: task 181442_241225
BASE_COMMIT: 0cdb95730ab6c19a898e70ddb57ee427def182b9
## Summary
- Added language plugin factory for project type selection and wired index sync to use it.
- Simplified status config loading and tightened ignore loading error handling.
- Removed unused chunker helper and cleaned up index build signature.

## Files changed and deleted (task-scoped)
- M	internal/app/app.go
- M	internal/index/build.go
- A	internal/lang/factory/factory.go
- M	internal/lang/ts/chunker.go
- M	task_result.md

## Commands executed
- git rev-parse HEAD (exit 0)
- cat <<'EOF' > task_result.md (exit 0)
- gofmt -w ./cmd ./internal (exit 0)
- go test ./... (exit 1)
- gofmt -w ./cmd ./internal (exit 0)
- go test ./... (exit 0)
- go build ./cmd/repodex (exit 0)
- rm -f repodex (exit 0)
- git status --short (exit 0)
- git add internal/app/app.go internal/index/build.go internal/lang/ts/chunker.go internal/lang/factory/factory.go task_result.md (exit 0)
- git commit -m "Select plugin by project type" (exit 0)
- git diff --name-status 0cdb95730ab6c19a898e70ddb57ee427def182b9..HEAD (exit 0)
- gofmt -w ./... (exit 2)
- gofmt -w $(go list -f '{{.Dir}}' ./...) (exit 0)
- go test ./... (exit 0)
- go build ./cmd/repodex (exit 0)
- git add task_result.md (exit 0)
- git commit --amend --no-edit (exit 0)
- go build -v ./cmd/repodex (exit 0)
