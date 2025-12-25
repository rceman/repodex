package statusx

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/memkit/repodex/internal/gitx"
	"github.com/memkit/repodex/internal/store"
)

type GitInfo struct {
	Repo             bool
	BaseHead         string
	CurrentHead      string
	WorktreeClean    bool
	WorktreeDirty    bool
	DirtyPathCount   int
	DirtyRepodexOnly bool
	ChangedPaths     []string
	ChangedPathCount int
	ChangedReason    string
}

type SyncPlan struct {
	Mode             string   `json:"mode"` // "full" | "noop" | "incremental"
	BaseHead         string   `json:"base_head,omitempty"`
	CurrentHead      string   `json:"current_head,omitempty"`
	WorktreeClean    bool     `json:"worktree_clean,omitempty"`
	ChangedPaths     []string `json:"changed_paths,omitempty"`
	ChangedPathCount int      `json:"changed_path_count,omitempty"`
	// Canonical Stage 2 reasons:
	// - "up_to_date"
	// - "missing_index"
	// - "not_git_repo"
	// - "schema_changed"
	// - "config_changed"
	// - "git_head_changed"
	// - "git_worktree_changed"
	// - "git_head_and_worktree_changed"
	// - "git_changed_non_indexable"
	// - "unknown"
	Why string `json:"why,omitempty"`
}

const MaxChangedPaths = 200

func CollectGitInfo(root string, baseHead string) GitInfo {
	info := GitInfo{
		BaseHead: baseHead,
	}
	isRepo, err := gitx.IsRepo(root)
	if err != nil {
		info.ChangedReason = "unknown"
		return info
	}
	if !isRepo {
		// Non-git repo: do not emit git change reason/paths; SyncPlan will explain "not_git_repo".
		info.WorktreeClean = true
		info.WorktreeDirty = false
		return info
	}
	info.Repo = true

	head, err := gitx.Head(root)
	if err != nil {
		info.ChangedReason = "unknown"
		return info
	}
	info.CurrentHead = head

	clean, err := gitx.IsWorkTreeClean(root)
	if err != nil {
		info.ChangedReason = "unknown"
		return info
	}
	info.WorktreeClean = clean
	info.WorktreeDirty = !clean

	changedSet := make(map[string]struct{})
	if !info.WorktreeClean {
		paths, err := gitx.StatusChangedPaths(root)
		if err != nil {
			info.ChangedReason = "unknown"
			return info
		}
		info.DirtyPathCount = len(paths)
		if len(paths) > 0 {
			repodexOnly := true
			for _, p := range paths {
				if p == ".repodex" || strings.HasPrefix(p, ".repodex/") {
					continue
				}
				repodexOnly = false
				break
			}
			info.DirtyRepodexOnly = repodexOnly
		}
		// Use porcelain paths as the single source for worktree change paths (staged/unstaged/untracked).
		addChangedPaths(changedSet, paths)
	}

	headChanged := info.BaseHead != "" && info.CurrentHead != "" && info.BaseHead != info.CurrentHead
	gitErr := false
	if headChanged {
		paths, err := gitx.DiffNameOnly(root, info.BaseHead, info.CurrentHead)
		if err != nil {
			gitErr = true
		} else {
			addChangedPaths(changedSet, paths)
		}
	}
	info.ChangedPathCount = len(changedSet)
	info.ChangedPaths = sortedLimitedPaths(changedSet, MaxChangedPaths)
	if gitErr {
		info.ChangedReason = "unknown"
		return info
	}
	worktreeChanged := !info.WorktreeClean
	switch {
	case !headChanged && !worktreeChanged && info.ChangedPathCount == 0:
		info.ChangedReason = "none"
	case !headChanged && worktreeChanged:
		info.ChangedReason = "worktree"
	case headChanged && !worktreeChanged:
		info.ChangedReason = "head"
	case headChanged && worktreeChanged:
		info.ChangedReason = "head+worktree"
	default:
		info.ChangedReason = "unknown"
	}
	return info
}

func BuildSyncPlan(meta store.Meta, cfgHash uint64, info GitInfo) *SyncPlan {
	plan := &SyncPlan{
		Mode:             "full",
		BaseHead:         meta.RepoHead,
		CurrentHead:      info.CurrentHead,
		WorktreeClean:    info.WorktreeClean,
		ChangedPaths:     info.ChangedPaths,
		ChangedPathCount: info.ChangedPathCount,
	}

	if !info.Repo {
		plan.Why = "not_git_repo"
		return plan
	}
	if info.ChangedReason == "unknown" {
		plan.Why = "unknown"
		return plan
	}
	if meta.SchemaVersion != store.SchemaVersion {
		plan.Why = "schema_changed"
		return plan
	}
	if meta.ConfigHash != cfgHash {
		plan.Why = "config_changed"
		return plan
	}

	headMatches := info.BaseHead != "" && info.CurrentHead != "" && info.BaseHead == info.CurrentHead
	if info.DirtyRepodexOnly && headMatches {
		plan.Mode = "noop"
		plan.Why = "git_changed_non_indexable"
		return plan
	}

	if info.WorktreeDirty && !headMatches {
		plan.Why = "git_head_and_worktree_changed"
		return plan
	}
	if !headMatches {
		plan.Why = "git_head_changed"
		return plan
	}
	if info.WorktreeDirty {
		if info.ChangedPathCount > 0 {
			plan.Why = "git_worktree_changed"
			return plan
		}
		plan.Why = "git_changed_non_indexable"
		return plan
	}

	plan.Mode = "noop"
	plan.Why = "up_to_date"
	return plan
}

func addChangedPaths(set map[string]struct{}, paths []string) {
	for _, p := range paths {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if !isIndexableChangedPath(p) {
			continue
		}
		set[p] = struct{}{}
	}
}

func isIndexableChangedPath(p string) bool {
	p = filepath.ToSlash(p)
	if p == "" {
		return false
	}
	// Never treat index artifacts as content changes.
	if p == ".repodex" || strings.HasPrefix(p, ".repodex/") {
		return false
	}
	// TypeScript only.
	if strings.HasSuffix(p, ".d.ts") {
		return false
	}
	return strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".tsx")
}

func sortedLimitedPaths(set map[string]struct{}, limit int) []string {
	if len(set) == 0 {
		return nil
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	if len(paths) > limit {
		return paths[:limit]
	}
	return paths
}
