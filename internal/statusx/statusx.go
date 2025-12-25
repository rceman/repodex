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

// Git changed reasons (enum-like).
const (
	GitChangedNone            = "none"
	GitChangedWorktree        = "worktree"
	GitChangedHead            = "head"
	GitChangedHeadAndWorktree = "head+worktree"
	GitChangedUnknown         = "unknown"
)

type SyncPlan struct {
	Mode             string   `json:"mode"` // see Mode* constants
	BaseHead         string   `json:"base_head,omitempty"`
	CurrentHead      string   `json:"current_head,omitempty"`
	WorktreeClean    bool     `json:"worktree_clean,omitempty"`
	ChangedPaths     []string `json:"changed_paths,omitempty"`
	ChangedPathCount int      `json:"changed_path_count,omitempty"`
	// Canonical Stage 2 reasons:
	// - WhyUpToDate
	// - WhyMissingIndex
	// - WhyNotGitRepo
	// - WhySchemaChanged
	// - WhyConfigChanged
	// - WhyGitHeadChanged
	// - WhyGitWorktreeChanged
	// - WhyGitHeadAndWorktreeChanged
	// - WhyGitChangedNonIndexable
	// - WhyUnknown
	Why string `json:"why,omitempty"`
}

// SyncPlan modes (enum-like).
const (
	ModeFull        = "full"
	ModeNoop        = "noop"
	ModeIncremental = "incremental" // reserved for future use
)

// SyncPlan reasons (enum-like).
const (
	WhyUpToDate                  = "up_to_date"
	WhyMissingIndex              = "missing_index"
	WhyNotGitRepo                = "not_git_repo" // reserved for compatibility; git-only mode may not emit this.
	WhySchemaChanged             = "schema_changed"
	WhyConfigChanged             = "config_changed"
	WhyGitHeadChanged            = "git_head_changed"
	WhyGitWorktreeChanged        = "git_worktree_changed"
	WhyGitHeadAndWorktreeChanged = "git_head_and_worktree_changed"
	WhyGitChangedNonIndexable    = "git_changed_non_indexable"
	WhyUnknown                   = "unknown"
)

func IsValidMode(mode string) bool {
	switch mode {
	case ModeFull, ModeNoop, ModeIncremental:
		return true
	default:
		return false
	}
}

func IsValidGitChangedReason(v string) bool {
	switch v {
	case "", GitChangedNone, GitChangedWorktree, GitChangedHead, GitChangedHeadAndWorktree, GitChangedUnknown:
		return true
	default:
		return false
	}
}

func IsValidWhy(why string) bool {
	switch why {
	case "", WhyUpToDate, WhyMissingIndex, WhyNotGitRepo, WhySchemaChanged, WhyConfigChanged,
		WhyGitHeadChanged, WhyGitWorktreeChanged, WhyGitHeadAndWorktreeChanged, WhyGitChangedNonIndexable, WhyUnknown:
		return true
	default:
		return false
	}
}

const MaxChangedPaths = 200

func CollectGitInfo(root string, baseHead string) GitInfo {
	info := GitInfo{
		BaseHead: baseHead,
	}
	isRepo, err := gitx.IsRepo(root)
	if err != nil {
		info.ChangedReason = GitChangedUnknown
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
		info.ChangedReason = GitChangedUnknown
		return info
	}
	info.CurrentHead = head

	clean, err := gitx.IsWorkTreeClean(root)
	if err != nil {
		info.ChangedReason = GitChangedUnknown
		return info
	}
	info.WorktreeClean = clean
	info.WorktreeDirty = !clean

	changedSet := make(map[string]struct{})
	if !info.WorktreeClean {
		paths, err := gitx.StatusChangedPaths(root)
		if err != nil {
			info.ChangedReason = GitChangedUnknown
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
		info.ChangedReason = GitChangedUnknown
		return info
	}
	worktreeChanged := !info.WorktreeClean
	switch {
	case !headChanged && !worktreeChanged && info.ChangedPathCount == 0:
		info.ChangedReason = GitChangedNone
	case !headChanged && worktreeChanged:
		info.ChangedReason = GitChangedWorktree
	case headChanged && !worktreeChanged:
		info.ChangedReason = GitChangedHead
	case headChanged && worktreeChanged:
		info.ChangedReason = GitChangedHeadAndWorktree
	default:
		info.ChangedReason = GitChangedUnknown
	}
	return info
}

func BuildSyncPlan(meta store.Meta, cfgHash uint64, info GitInfo) *SyncPlan {
	plan := &SyncPlan{
		Mode:             ModeFull,
		BaseHead:         meta.RepoHead,
		CurrentHead:      info.CurrentHead,
		WorktreeClean:    info.WorktreeClean,
		ChangedPaths:     info.ChangedPaths,
		ChangedPathCount: info.ChangedPathCount,
	}

	if !info.Repo {
		plan.Why = WhyNotGitRepo
		return plan
	}
	if info.ChangedReason == GitChangedUnknown {
		plan.Why = WhyUnknown
		return plan
	}
	if meta.SchemaVersion != store.SchemaVersion {
		plan.Why = WhySchemaChanged
		return plan
	}
	if meta.ConfigHash != cfgHash {
		plan.Why = WhyConfigChanged
		return plan
	}

	headMatches := info.BaseHead != "" && info.CurrentHead != "" && info.BaseHead == info.CurrentHead
	if info.DirtyRepodexOnly && headMatches {
		plan.Mode = ModeNoop
		plan.Why = WhyGitChangedNonIndexable
		return plan
	}

	if info.WorktreeDirty && !headMatches {
		plan.Why = WhyGitHeadAndWorktreeChanged
		return plan
	}
	if !headMatches {
		plan.Why = WhyGitHeadChanged
		return plan
	}
	if info.WorktreeDirty {
		if info.ChangedPathCount > 0 {
			plan.Why = WhyGitWorktreeChanged
			return plan
		}
		plan.Why = WhyGitChangedNonIndexable
		return plan
	}

	plan.Mode = ModeNoop
	plan.Why = WhyUpToDate
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
