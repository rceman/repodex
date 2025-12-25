# StatusResponse and SyncPlan compatibility

Repodex requires a git repository and resolves the working root via `git rev-parse --show-toplevel`. For git repositories, `dirty` and `changed_files` are derived from `sync_plan` (indexable TS/TSX paths). Outside git, commands return an error instead of a StatusResponse. If index artifacts are missing, `sync_plan.why` is `missing_index` and a full sync is required.

StatusResponse compatibility contract

- Stable fields (always present, stable semantics and types): `indexed`, `indexed_at_unix`, `file_count`, `chunk_count`, `term_count`, `dirty`, `changed_files`.
- Optional extension fields (omitempty): `sync_plan`, `schema_version`, `repodex_version`, all `git_*` fields (including legacy ones).
- Deprecated but preserved legacy fields: `git_repo`, `repo_head`, `current_head`, `worktree_clean`, `head_matches`, `git_dirty_path_count`, `git_dirty_repodex_only` (clients should treat them as optional).
- Client rules: ignore unknown fields; do not require `sync_plan`; if `sync_plan` is absent, treat the reason as `unknown`.

Git alignment rules

- For git repositories, `dirty` must align with `sync_plan.mode != noop` (except the `.repodex`-only case where `why = git_changed_non_indexable` allows `mode = noop`, `dirty = false`).
- For git repositories, `changed_files` equals `sync_plan.changed_path_count` (indexable changes).

Canonical `sync_plan.mode` values:

- `full`
- `noop`
- `incremental` (reserved)

Canonical `sync_plan.why` values:

- `up_to_date`
- `missing_index`
- `not_git_repo` (reserved for compatibility; git-only mode may not emit this)
- `schema_changed`
- `config_changed`
- `git_head_changed`
- `git_worktree_changed`
- `git_head_and_worktree_changed`
- `git_changed_non_indexable`
- `unknown`
