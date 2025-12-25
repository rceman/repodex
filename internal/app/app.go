package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/memkit/repodex/internal/cli"
	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/fetch"
	"github.com/memkit/repodex/internal/gitx"
	"github.com/memkit/repodex/internal/hash"
	"github.com/memkit/repodex/internal/ignore"
	"github.com/memkit/repodex/internal/index"
	"github.com/memkit/repodex/internal/lang/factory"
	"github.com/memkit/repodex/internal/scan"
	"github.com/memkit/repodex/internal/search"
	"github.com/memkit/repodex/internal/serve"
	"github.com/memkit/repodex/internal/store"
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

	// Git-related diagnostics (helpful for clients such as Codex).
	GitRepo       bool   `json:"git_repo,omitempty"`
	RepoHead      string `json:"repo_head,omitempty"`
	CurrentHead   string `json:"current_head,omitempty"`
	WorktreeClean bool   `json:"worktree_clean,omitempty"`
	HeadMatches   bool   `json:"head_matches,omitempty"`

	// Git status diagnostics (useful for reminding to commit the index).
	GitDirtyPathCount   int  `json:"git_dirty_path_count,omitempty"`
	GitDirtyRepodexOnly bool `json:"git_dirty_repodex_only,omitempty"`

	SchemaVersion  int    `json:"schema_version,omitempty"`
	RepodexVersion string `json:"repodex_version,omitempty"`
}

// Run executes the CLI app and returns an exit code.
func Run(args []string) int {
	cmd, err := cli.Parse(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	switch cmd.Action {
	case "init":
		if err := runInit(".", cmd.Force); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "status":
		if err := runStatus(".", cmd.JSON); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "sync":
		if err := runIndexSync("."); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "search":
		if err := runSearch(".", cmd.Q, cmd.TopK); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "fetch":
		if err := runFetch(".", cmd.IDs, cmd.MaxLines); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case "index":
		switch cmd.Subcommand {
		case "sync":
			if err := runIndexSync("."); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		case "status":
			if err := runStatus(".", cmd.JSON); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return 0
		}
	case "serve":
		if cmd.Stdio {
			if err := runServeStdio("."); err != nil {
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

func runIndexSync(root string) error {
	cfgPath := store.ConfigPath(root)
	cfg, cfgBytes, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

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

	files, err := scan.Walk(root, cfg, ignoreDirs)
	if err != nil {
		return err
	}

	fileEntries, chunkEntries, postings, err := index.Build(files, plugin, cfg)
	if err != nil {
		return err
	}

	if err := index.Serialize(root, fileEntries, chunkEntries, postings); err != nil {
		return err
	}

	repoHead := currentRepoHead(root)
	cfgHash := hash.Sum64(cfgBytes)
	meta := store.NewMeta(cfg.IndexVersion, len(fileEntries), len(chunkEntries), len(postings), cfgHash, repoHead)
	if err := store.SaveMeta(store.MetaPath(root), meta); err != nil {
		return err
	}
	return nil
}

func runStatus(root string, jsonOut bool) error {
	resp, err := computeStatus(root)
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

func computeStatus(root string) (StatusResponse, error) {
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
		return StatusResponse{
			Indexed:       false,
			IndexedAtUnix: 0,
			FileCount:     0,
			ChunkCount:    0,
			TermCount:     0,
			Dirty:         true,
			ChangedFiles:  0,
		}, nil
	}

	meta, err := store.LoadMeta(metaPath)
	if err != nil {
		return StatusResponse{}, err
	}
	cfg, cfgBytes, err := config.Load(cfgPath)
	if err != nil {
		return StatusResponse{}, err
	}
	cfgHash := hash.Sum64(cfgBytes)

	// Collect git diagnostics once; do not fail status on git errors.
	gitRepo := false
	currentHead := ""
	worktreeClean := false
	gitDirtyPathCount := 0
	gitDirtyRepodexOnly := false
	if isRepo, err := gitx.IsRepo(root); err == nil && isRepo {
		gitRepo = true
		if head, err := gitx.Head(root); err == nil {
			currentHead = head
		}
		if clean, err := gitx.IsWorkTreeClean(root); err == nil {
			worktreeClean = clean
		}
		if !worktreeClean {
			if paths, err := gitx.StatusChangedPaths(root); err == nil {
				gitDirtyPathCount = len(paths)
				if len(paths) > 0 {
					repodexOnly := true
					for _, p := range paths {
						// p uses forward slashes from git.
						if p == ".repodex" || strings.HasPrefix(p, ".repodex/") {
							continue
						}
						repodexOnly = false
						break
					}
					gitDirtyRepodexOnly = repodexOnly
				}
			}
		}
	}
	headMatches := gitRepo && currentHead != "" && meta.RepoHead != "" && currentHead == meta.RepoHead

	if meta.SchemaVersion == store.SchemaVersion && meta.ConfigHash == cfgHash {
		if headMatches && worktreeClean {
			return StatusResponse{
				Indexed:       true,
				IndexedAtUnix: meta.IndexedAtUnix,
				FileCount:     meta.FileCount,
				ChunkCount:    meta.ChunkCount,
				TermCount:     meta.TermCount,
				Dirty:         false,
				ChangedFiles:  0,

				GitRepo:             gitRepo,
				RepoHead:            meta.RepoHead,
				CurrentHead:         currentHead,
				WorktreeClean:       worktreeClean,
				HeadMatches:         headMatches,
				GitDirtyPathCount:   gitDirtyPathCount,
				GitDirtyRepodexOnly: gitDirtyRepodexOnly,
				SchemaVersion:       meta.SchemaVersion,
				RepodexVersion:      meta.RepodexVersion,
			}, nil
		}
	}

	var ignoreDirs []string
	if dirs, err := ignore.LoadDirs(store.IgnorePath(root)); err == nil {
		ignoreDirs = dirs
	} else if !os.IsNotExist(err) {
		return StatusResponse{}, err
	}

	indexedFiles, err := index.LoadFileEntries(filesPath)
	if err != nil {
		return StatusResponse{}, err
	}

	currentFiles, err := scan.WalkMeta(root, cfg, ignoreDirs)
	if err != nil {
		return StatusResponse{}, err
	}

	indexMap := make(map[string]index.FileEntry, len(indexedFiles))
	for _, f := range indexedFiles {
		indexMap[filepath.ToSlash(f.Path)] = f
	}

	seen := make(map[string]struct{})
	changed := 0
	if meta.ConfigHash != cfgHash {
		changed++
	}
	for _, f := range currentFiles {
		path := filepath.ToSlash(f.Path)
		seen[path] = struct{}{}
		if existing, ok := indexMap[path]; ok {
			if existing.MTime != f.MTime || existing.Size != f.Size {
				changed++
			}
		} else {
			changed++
		}
	}
	for path := range indexMap {
		if _, ok := seen[path]; !ok {
			changed++
		}
	}

	return StatusResponse{
		Indexed:       true,
		IndexedAtUnix: meta.IndexedAtUnix,
		FileCount:     meta.FileCount,
		ChunkCount:    meta.ChunkCount,
		TermCount:     meta.TermCount,
		Dirty:         changed > 0,
		ChangedFiles:  changed,

		GitRepo:             gitRepo,
		RepoHead:            meta.RepoHead,
		CurrentHead:         currentHead,
		WorktreeClean:       worktreeClean,
		HeadMatches:         headMatches,
		GitDirtyPathCount:   gitDirtyPathCount,
		GitDirtyRepodexOnly: gitDirtyRepodexOnly,
		SchemaVersion:       meta.SchemaVersion,
		RepodexVersion:      meta.RepodexVersion,
	}, nil
}

func runServeStdio(root string) error {
	statusFn := func() (interface{}, error) {
		return computeStatus(root)
	}
	syncFn := func() (interface{}, error) {
		if err := runIndexSync(root); err != nil {
			return nil, err
		}
		return computeStatus(root)
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
