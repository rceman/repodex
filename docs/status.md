# StatusResponse and SyncPlan compatibility

StatusResponse preserves existing fields for backward compatibility:

- `indexed`, `indexed_at_unix`, `file_count`, `chunk_count`, `term_count`, `dirty`, `changed_files` remain stable.
- New fields are optional and may be omitted: `git_*`, `sync_plan`, `schema_version`, `repodex_version`.
- Clients should ignore unknown fields and must not require the presence of new fields.

Git vs non-git behavior:

- For git repositories, `dirty` and `changed_files` are derived from `sync_plan` and reflect git-detected, indexable path changes (TS/TSX only). Filesystem mtime/size comparisons are not used.
- For non-git repositories, `dirty`/`changed_files` use filesystem comparison (mtime/size). `sync_plan.why` is `not_git_repo`.
- For missing index artifacts, `sync_plan.why` is `missing_index` and a full sync is required.

Canonical `sync_plan.why` values:

- `up_to_date`
- `missing_index`
- `not_git_repo`
- `schema_changed`
- `config_changed`
- `git_head_changed`
- `git_worktree_changed`
- `git_head_and_worktree_changed`
- `git_changed_non_indexable`
- `unknown`

When the plan is absent, clients should treat the reason as `unknown`.

Deprecated / backward-compatible fields:

- Legacy git fields (`git_repo`, `repo_head`, `current_head`, `worktree_clean`, `head_matches`, `git_dirty_path_count`, `git_dirty_repodex_only`) remain for compatibility but may be omitted for non-git roots.
- New fields are optional (omitempty); clients must ignore unknown fields and should not rely on their presence.
