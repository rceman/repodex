package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/memkit/repodex/internal/cli"
	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/fetch"
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

	cfgBytes, err := os.ReadFile(store.ConfigPath(root))
	if err != nil {
		return err
	}
	cfgHash := hash.Sum64(cfgBytes)
	meta := store.NewMeta(cfg.IndexVersion, 0, 0, 0, cfgHash)
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

	cfgHash := hash.Sum64(cfgBytes)
	meta := store.NewMeta(cfg.IndexVersion, len(fileEntries), len(chunkEntries), len(postings), cfgHash)
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
	cfgPath := store.ConfigPath(root)

	metaExists, err := fileExistsOk(metaPath)
	if err != nil {
		return StatusResponse{}, err
	}
	filesExists, err := fileExistsOk(filesPath)
	if err != nil {
		return StatusResponse{}, err
	}

	if !metaExists || !filesExists {
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
