package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/memkit/repodex/internal/cachex"
	"github.com/memkit/repodex/internal/cli"
	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/fetch"
	"github.com/memkit/repodex/internal/gitx"
	"github.com/memkit/repodex/internal/hash"
	"github.com/memkit/repodex/internal/ignore"
	"github.com/memkit/repodex/internal/index"
	"github.com/memkit/repodex/internal/lang"
	"github.com/memkit/repodex/internal/lang/factory"
	"github.com/memkit/repodex/internal/scan"
	"github.com/memkit/repodex/internal/search"
	"github.com/memkit/repodex/internal/serve"
	"github.com/memkit/repodex/internal/statusx"
	"github.com/memkit/repodex/internal/store"
	"github.com/memkit/repodex/internal/textutil"
)

// StatusResponse describes output of status command.
type StatusResponse struct {
	Indexed       bool  `json:"indexed"`
	IndexedAtUnix int64 `json:"indexed_at_unix"`
	FileCount     int   `json:"file_count"`
	ChunkCount    int   `json:"chunk_count"`
	TermCount     int   `json:"term_count"`
	Dirty         bool  `json:"dirty"`
	ChangedFiles  int   `json:"changed_files"`

	// Legacy git fields (prefer GitBaseHead/GitCurrentHead/GitChanged*).
	GitRepo       bool   `json:"git_repo,omitempty"`       // Deprecated: use GitBaseHead/GitCurrentHead.
	RepoHead      string `json:"repo_head,omitempty"`      // Deprecated: use GitBaseHead.
	CurrentHead   string `json:"current_head,omitempty"`   // Deprecated: use GitCurrentHead.
	WorktreeClean bool   `json:"worktree_clean,omitempty"` // Deprecated: use GitWorktreeClean.
	HeadMatches   bool   `json:"head_matches,omitempty"`   // Deprecated: compare GitBaseHead vs GitCurrentHead.

	// Git status diagnostics (useful for reminding to commit the index). Kept for backward compatibility.
	GitDirtyPathCount   int  `json:"git_dirty_path_count,omitempty"`
	GitDirtyRepodexOnly bool `json:"git_dirty_repodex_only,omitempty"`

	SchemaVersion  int    `json:"schema_version,omitempty"`
	RepodexVersion string `json:"repodex_version,omitempty"`

	GitBaseHead         string   `json:"git_base_head,omitempty"`
	GitCurrentHead      string   `json:"git_current_head,omitempty"`
	GitWorktreeClean    bool     `json:"git_worktree_clean,omitempty"`
	GitChangedPathCount int      `json:"git_changed_path_count,omitempty"`
	GitChangedPaths     []string `json:"git_changed_paths,omitempty"`
	// GitChangedReason is a git-domain signal (not a SyncPlan why).
	GitChangedReason    string            `json:"git_changed_reason,omitempty"`
	GitChangedIndexable bool              `json:"git_changed_indexable,omitempty"`
	SyncPlan            *statusx.SyncPlan `json:"sync_plan,omitempty"`
}

// Run executes the CLI app and returns an exit code.
func Run(args []string) int {
	cmd, err := cli.Parse(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	repoRoot, err := resolveRepoRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	switch cmd.Action {
	case "init":
		if err := runInit(repoRoot, cmd.Force); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "status":
		if err := runStatus(repoRoot, cmd.JSON); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "sync":
		if err := runIndexSync(repoRoot); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "search":
		if err := runSearch(repoRoot, cmd.Q, cmd.TopK); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "fetch":
		if err := runFetch(repoRoot, cmd.IDs, cmd.MaxLines); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "index":
		switch cmd.Subcommand {
		case "sync":
			if err := runIndexSync(repoRoot); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		case "status":
			if err := runStatus(repoRoot, cmd.JSON); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		}
	case "serve":
		if cmd.Stdio {
			if err := runServeStdio(repoRoot); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		}
		fmt.Fprintln(os.Stderr, "serve supports --stdio only")
		return 1
	}
	return 1
}

func runInit(root string, force bool) error {
	dir := store.Dir(root)
	if !force {
		if _, err := os.Stat(dir); err == nil {
			return fmt.Errorf("%s already exists; rerun with --force to overwrite", dir)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if force {
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	cfg := config.DefaultConfig()
	if err := config.Save(store.ConfigPath(root), cfg); err != nil {
		return err
	}
	if err := ignore.WriteDefaultIgnore(store.IgnorePath(root)); err != nil {
		return err
	}

	repoHead := currentRepoHead(root)
	cfgBytes, err := os.ReadFile(store.ConfigPath(root))
	if err != nil {
		return err
	}
	cfgHash := hash.Sum64(cfgBytes)
	meta := store.NewMeta(cfg.IndexVersion, 0, 0, 0, cfgHash, repoHead)
	if err := store.SaveMeta(store.MetaPath(root), meta); err != nil {
		return err
	}
	return nil
}

func precomputedFromCache(entry cachex.CacheEntry) (index.PrecomputedFile, error) {
	if len(entry.Chunks) != len(entry.Tokens) {
		return index.PrecomputedFile{}, fmt.Errorf("cache invalid for %s: chunk/token length mismatch", entry.RelPath)
	}
	const maxU32 = uint64(^uint32(0))
	chunks := make([]index.PrecomputedChunk, 0, len(entry.Chunks))
	for idx, ch := range entry.Chunks {
		if ch.Start < 1 || ch.End < 1 || ch.End < ch.Start {
			return index.PrecomputedFile{}, fmt.Errorf("cache invalid for %s: invalid chunk line range", entry.RelPath)
		}
		if uint64(ch.Start) > maxU32 || uint64(ch.End) > maxU32 {
			return index.PrecomputedFile{}, fmt.Errorf("cache invalid for %s: invalid chunk line range", entry.RelPath)
		}
		chunks = append(chunks, index.PrecomputedChunk{
			StartLine: uint32(ch.Start),
			EndLine:   uint32(ch.End),
			Snippet:   ch.Snippet,
			Tokens:    entry.Tokens[idx],
		})
	}
	return index.PrecomputedFile{
		Path:   filepath.ToSlash(entry.RelPath),
		MTime:  entry.MTime,
		Size:   entry.Size,
		Hash64: entry.Hash64,
		Chunks: chunks,
	}, nil
}

func buildCacheEntry(ref scan.FileRef, plugin lang.LanguagePlugin, cfg config.Config) (index.PrecomputedFile, cachex.CacheEntry, error) {
	content, err := os.ReadFile(ref.AbsPath)
	if err != nil {
		return index.PrecomputedFile{}, cachex.CacheEntry{}, err
	}
	normalized := textutil.NormalizeNewlinesBytes(content)
	hash64 := hash.Sum64(normalized)

	chunkDrafts, err := plugin.ChunkFile(ref.RelPath, normalized, cfg.Chunk, cfg.Limits)
	if err != nil {
		return index.PrecomputedFile{}, cachex.CacheEntry{}, err
	}
	lines := strings.Split(string(normalized), "\n")

	precomputedChunks := make([]index.PrecomputedChunk, 0, len(chunkDrafts))
	cacheChunks := make([]cachex.LocalChunk, 0, len(chunkDrafts))
	tokenSets := make([][]string, 0, len(chunkDrafts))

	for _, ch := range chunkDrafts {
		chunkText := chunkTextFromLines(lines, int(ch.StartLine), int(ch.EndLine))
		tokens := plugin.TokenizeChunk(ref.RelPath, chunkText, cfg.Token)
		precomputedChunks = append(precomputedChunks, index.PrecomputedChunk{
			StartLine: ch.StartLine,
			EndLine:   ch.EndLine,
			Snippet:   ch.Snippet,
			Tokens:    tokens,
		})
		cacheChunks = append(cacheChunks, cachex.LocalChunk{
			Start:   int(ch.StartLine),
			End:     int(ch.EndLine),
			Snippet: ch.Snippet,
		})
		tokenSets = append(tokenSets, tokens)
	}

	file := index.PrecomputedFile{
		Path:   filepath.ToSlash(ref.RelPath),
		MTime:  ref.MTime,
		Size:   ref.Size,
		Hash64: hash64,
		Chunks: precomputedChunks,
	}
	cacheEntry := cachex.CacheEntry{
		RelPath: filepath.ToSlash(ref.RelPath),
		Size:    ref.Size,
		MTime:   ref.MTime,
		Hash64:  hash64,
		Chunks:  cacheChunks,
		Tokens:  tokenSets,
	}
	return file, cacheEntry, nil
}

func chunkTextFromLines(lines []string, start, end int) string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return ""
	}
	return strings.Join(lines[start-1:end], "\n")
}

func runIndexSync(root string) error {
	st, err := computeStatusResolved(root)
	if err != nil {
		return err
	}
	if st.SyncPlan != nil && st.SyncPlan.Mode == statusx.ModeNoop {
		return nil
	}

	cfgPath := store.ConfigPath(root)
	cfg, cfgBytes, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	cfgHash := hash.Sum64(cfgBytes)

	var ignoreDirs []string
	if dirs, err := ignore.LoadDirs(store.IgnorePath(root)); err == nil {
		ignoreDirs = dirs
	} else if !os.IsNotExist(err) {
		return err
	}

	plugin, err := factory.FromProjectType(cfg.ProjectType)
	if err != nil {
		return err
	}

	changedSet := make(map[string]struct{})
	fullRebuild := true
	if st.SyncPlan != nil {
		for _, p := range st.SyncPlan.ChangedPaths {
			changedSet[ignore.NormalizePath(p)] = struct{}{}
		}
		fullRebuild = st.SyncPlan.Why == statusx.WhyMissingIndex ||
			st.SyncPlan.Why == statusx.WhySchemaChanged ||
			st.SyncPlan.Why == statusx.WhyConfigChanged
	}

	if fullRebuild {
		if err := cachex.PurgeV1(root); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	purged, err := cachex.EnsureMeta(root, cachex.Meta{
		ConfigHash:  cfgHash,
		ProjectType: cfg.ProjectType,
	})
	if err != nil {
		return err
	}
	if purged {
		fullRebuild = true
	}

	refs, err := scan.WalkRefs(root, cfg, ignoreDirs)
	if err != nil {
		return err
	}

	precomputed := make([]index.PrecomputedFile, 0, len(refs))
	for _, ref := range refs {
		rebuild := fullRebuild
		if !rebuild {
			_, rebuild = changedSet[ref.RelPath]
		}
		if !rebuild {
			entry, ok, err := cachex.LoadByPath(root, ref.RelPath)
			if err != nil {
				return err
			}
			if ok {
				file, err := precomputedFromCache(entry)
				if err != nil {
					return err
				}
				precomputed = append(precomputed, file)
				continue
			}
		}

		file, cacheEntry, err := buildCacheEntry(ref, plugin, cfg)
		if err != nil {
			return err
		}
		if err := cachex.Save(root, cacheEntry); err != nil {
			return err
		}
		precomputed = append(precomputed, file)
	}

	fileEntries, chunkEntries, postings, err := index.BuildFromPrecomputed(precomputed)
	if err != nil {
		return err
	}

	if err := index.Serialize(root, fileEntries, chunkEntries, postings); err != nil {
		return err
	}

	repoHead := currentRepoHead(root)
	meta := store.NewMeta(cfg.IndexVersion, len(fileEntries), len(chunkEntries), len(postings), cfgHash, repoHead)
	if err := store.SaveMeta(store.MetaPath(root), meta); err != nil {
		return err
	}
	return nil
}

func runStatus(root string, jsonOut bool) error {
	resp, err := computeStatusResolved(root)
	if err != nil {
		return err
	}
	return outputStatus(resp, jsonOut)
}

func outputStatus(resp StatusResponse, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		return enc.Encode(resp)
	}
	fmt.Printf("Indexed: %v\nDirty: %v\nChanged files: %d\n", resp.Indexed, resp.Dirty, resp.ChangedFiles)
	if resp.GitRepo && !resp.WorktreeClean && resp.GitDirtyRepodexOnly {
		fmt.Printf("Git: working tree dirty due to .repodex only (commit index artifacts if you rely on portable index)\n")
	} else if resp.GitRepo && !resp.WorktreeClean {
		fmt.Printf("Git: working tree dirty (%d paths)\n", resp.GitDirtyPathCount)
	}
	if resp.SyncPlan != nil {
		fmt.Printf("Sync plan: mode=%s, why=%s\n", resp.SyncPlan.Mode, resp.SyncPlan.Why)
		if resp.SyncPlan.Why == statusx.WhyGitChangedNonIndexable && resp.GitDirtyRepodexOnly {
			fmt.Printf("Note: repo dirty only due to .repodex; commit index artifacts for portability.\n")
		}
	}
	return nil
}

func fileExistsOk(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func computeStatus(start string) (StatusResponse, error) {
	root, err := resolveRepoRoot(start)
	if err != nil {
		return StatusResponse{}, err
	}
	return computeStatusResolved(root)
}

func computeStatusResolved(root string) (StatusResponse, error) {
	metaPath := store.MetaPath(root)
	filesPath := store.FilesPath(root)
	chunksPath := store.ChunksPath(root)
	termsPath := store.TermsPath(root)
	postingsPath := store.PostingsPath(root)
	cfgPath := store.ConfigPath(root)

	metaExists, err := fileExistsOk(metaPath)
	if err != nil {
		return StatusResponse{}, err
	}
	filesExists, err := fileExistsOk(filesPath)
	if err != nil {
		return StatusResponse{}, err
	}
	chunksExists, err := fileExistsOk(chunksPath)
	if err != nil {
		return StatusResponse{}, err
	}
	termsExists, err := fileExistsOk(termsPath)
	if err != nil {
		return StatusResponse{}, err
	}
	postingsExists, err := fileExistsOk(postingsPath)
	if err != nil {
		return StatusResponse{}, err
	}

	if !metaExists || !filesExists || !chunksExists || !termsExists || !postingsExists {
		var meta store.Meta
		if metaExists {
			if loaded, err := store.LoadMeta(metaPath); err == nil {
				meta = loaded
			}
		}
		gitInfo := statusx.CollectGitInfo(root, meta.RepoHead)
		if !gitInfo.Repo {
			return StatusResponse{}, fmt.Errorf("repodex requires a git repository")
		}
		resp := StatusResponse{
			Indexed:       false,
			IndexedAtUnix: 0,
			FileCount:     0,
			ChunkCount:    0,
			TermCount:     0,
			Dirty:         true,
			ChangedFiles:  0,
		}
		applyGitInfo(&resp, gitInfo)
		resp.SyncPlan = &statusx.SyncPlan{
			Mode:             statusx.ModeFull,
			Why:              statusx.WhyMissingIndex,
			BaseHead:         gitInfo.BaseHead,
			CurrentHead:      gitInfo.CurrentHead,
			WorktreeClean:    gitInfo.WorktreeClean,
			ChangedPaths:     gitInfo.ChangedPaths,
			ChangedPathCount: gitInfo.ChangedPathCount,
		}
		resp.Dirty = resp.SyncPlan.Mode != statusx.ModeNoop
		if gitInfo.Repo {
			resp.ChangedFiles = resp.SyncPlan.ChangedPathCount
		}
		return resp, nil
	}

	meta, err := store.LoadMeta(metaPath)
	if err != nil {
		return StatusResponse{}, err
	}

	gitInfo := statusx.CollectGitInfo(root, meta.RepoHead)
	if !gitInfo.Repo {
		return StatusResponse{}, fmt.Errorf("repodex requires a git repository")
	}

	_, cfgBytes, err := config.Load(cfgPath)
	if err != nil {
		return StatusResponse{}, err
	}
	cfgHash := hash.Sum64(cfgBytes)

	plan := statusx.BuildSyncPlan(meta, cfgHash, gitInfo)

	resp := StatusResponse{
		Indexed:        true,
		IndexedAtUnix:  meta.IndexedAtUnix,
		FileCount:      meta.FileCount,
		ChunkCount:     meta.ChunkCount,
		TermCount:      meta.TermCount,
		Dirty:          plan.Mode != statusx.ModeNoop,
		ChangedFiles:   plan.ChangedPathCount,
		SchemaVersion:  meta.SchemaVersion,
		RepodexVersion: meta.RepodexVersion,
	}
	applyGitInfo(&resp, gitInfo)
	resp.SyncPlan = plan
	return resp, nil
}

func applyGitInfo(resp *StatusResponse, info statusx.GitInfo) {
	resp.GitRepo = info.Repo
	resp.RepoHead = info.BaseHead
	resp.CurrentHead = info.CurrentHead
	resp.WorktreeClean = info.WorktreeClean
	resp.HeadMatches = info.Repo && info.BaseHead != "" && info.CurrentHead != "" && info.BaseHead == info.CurrentHead
	resp.GitDirtyPathCount = info.DirtyPathCount
	resp.GitDirtyRepodexOnly = info.DirtyRepodexOnly
	resp.GitBaseHead = info.BaseHead
	resp.GitCurrentHead = info.CurrentHead
	resp.GitWorktreeClean = info.WorktreeClean
	resp.GitChangedPathCount = info.ChangedPathCount
	resp.GitChangedPaths = info.ChangedPaths
	resp.GitChangedReason = info.ChangedReason
	resp.GitChangedIndexable = info.Repo && info.ChangedPathCount > 0
	// For non-git repos, keep git_* fields empty; SyncPlan explains not_git_repo.
	if !info.Repo {
		resp.GitChangedReason = ""
		resp.GitChangedPaths = nil
		resp.GitChangedPathCount = 0
		resp.GitChangedIndexable = false
	}
}

func runServeStdio(root string) error {
	statusFn := func() (interface{}, error) {
		return computeStatusResolved(root)
	}
	syncFn := func() (interface{}, error) {
		if err := runIndexSync(root); err != nil {
			return nil, err
		}
		return computeStatusResolved(root)
	}
	return serve.ServeStdio(root, statusFn, syncFn)
}

func runSearch(root string, q string, topK int) error {
	if q == "" {
		return fmt.Errorf("query cannot be empty")
	}
	results, err := search.Search(root, q, search.Options{TopK: topK})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(results)
}

func runFetch(root string, ids []uint32, maxLines int) error {
	if len(ids) == 0 {
		return fmt.Errorf("at least one id is required")
	}
	results, err := fetch.Fetch(root, ids, maxLines)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(results)
}

func currentRepoHead(root string) string {
	isRepo, err := gitx.IsRepo(root)
	if err != nil || !isRepo {
		return ""
	}
	head, err := gitx.Head(root)
	if err != nil {
		return ""
	}
	return head
}

func resolveRepoRoot(start string) (string, error) {
	root, err := gitx.TopLevel(start)
	if err != nil {
		return "", fmt.Errorf("repodex requires a git repository")
	}
	return root, nil
}
