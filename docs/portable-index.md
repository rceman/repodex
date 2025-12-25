# Portable index workflow

Repodex indices can be checked in for portability. Follow this workflow:

1. Run `repodex index sync` (or `repodex sync`) to build the index locally.
2. Commit the `.repodex` directory (including `*.dat` and `meta.json`). The repository marks `.repodex/*.dat` as binary via `.gitattributes`.
3. Use `repodex status --json` to confirm the state:
   - If the worktree is dirty only because of `.repodex`, `git_dirty_repodex_only` will be true.
   - The `sync_plan` will report `mode:"noop"` and `why:"git_changed_non_indexable"`, indicating no rebuild is needed despite repodex dirt.
   - Indexable change lists stay empty because `.repodex` artifacts are not treated as source changes.
4. When you update the index, re-run sync and commit the refreshed `.repodex` artifacts so that clean checkouts remain `sync_plan.mode:"noop"`.
