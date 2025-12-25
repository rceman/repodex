package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memkit/repodex/internal/statusx"
	"github.com/memkit/repodex/internal/store"
)

func TestRunInitForceOverwritesRepodexDir(t *testing.T) {
	root := setupGitRepo(t, false)

	if err := runInit(root, false); err != nil {
		t.Fatalf("initial init failed: %v", err)
	}

	sentinel := filepath.Join(root, ".repodex", "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("sentinel"), 0o644); err != nil {
		t.Fatalf("failed to write sentinel: %v", err)
	}

	if err := runInit(root, true); err != nil {
		t.Fatalf("force init failed: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil || !os.IsNotExist(err) {
		t.Fatalf("sentinel should be removed after force init")
	}

	required := []string{
		".repodex/config.json",
		".repodex/ignore",
		".repodex/meta.json",
	}
	for _, rel := range required {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}
}

func TestComputeStatusMissingIndexArtifact(t *testing.T) {
	root := setupGitRepo(t, true)

	if err := runInit(root, false); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	resp, err := computeStatus(root)
	if err != nil {
		t.Fatalf("computeStatus returned error: %v", err)
	}

	if resp.Indexed {
		t.Fatalf("expected Indexed to be false when an index artifact is missing")
	}
	if !resp.Dirty {
		t.Fatalf("expected Dirty to be true when an index artifact is missing")
	}
	if resp.ChangedFiles != 0 {
		t.Fatalf("expected ChangedFiles to be 0 for missing artifact, got %d", resp.ChangedFiles)
	}
}

func TestComputeStatusMissingIndexHasSyncPlan(t *testing.T) {
	root := setupGitRepo(t, true)

	if err := runInit(root, false); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	resp, err := computeStatus(root)
	if err != nil {
		t.Fatalf("computeStatus returned error: %v", err)
	}
	if resp.Indexed {
		t.Fatalf("expected Indexed=false for missing index artifacts")
	}
	if resp.SyncPlan == nil {
		t.Fatalf("expected SyncPlan to be present when index is missing")
	}
	if resp.SyncPlan.Mode != statusx.ModeFull {
		t.Fatalf("expected SyncPlan mode=full for missing index, got %s", resp.SyncPlan.Mode)
	}
	if resp.SyncPlan.Why != statusx.WhyMissingIndex {
		t.Fatalf("expected SyncPlan why=missing_index, got %s", resp.SyncPlan.Why)
	}
}

func TestComputeStatusNonGitUsesFilesystemDiff(t *testing.T) {
	root := t.TempDir()

	if err := runInit(root, false); err == nil {
		t.Fatalf("expected runInit to fail outside git repo")
	}

	resp, err := computeStatus(root)
	if err == nil {
		t.Fatalf("expected computeStatus to fail outside git repo")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Fatalf("expected git repository error, got %v", err)
	}
	if resp.Indexed || resp.SyncPlan != nil {
		t.Fatalf("expected empty response when computeStatus fails outside git repo")
	}
}

func TestComputeStatusCRLFFileNotDirtyAfterSync(t *testing.T) {
	root := setupGitRepo(t, true)

	content := "const a = 1;\r\nconst b = 2;\r\n"
	srcPath := filepath.Join(root, "sample.ts")
	if err := os.WriteFile(srcPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}
	runGit(t, root, "add", "sample.ts")
	runGit(t, root, "commit", "-m", "add sample")

	if err := runInit(root, false); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	if err := runIndexSync(root); err != nil {
		t.Fatalf("runIndexSync failed: %v", err)
	}

	resp, err := computeStatus(root)
	if err != nil {
		t.Fatalf("computeStatus returned error: %v", err)
	}

	if resp.Dirty {
		t.Fatalf("expected Dirty to be false after syncing CRLF file")
	}
}

func TestComputeStatusGitFastPathWhenClean(t *testing.T) {
	root := setupGitRepoWithIndex(t, true, true)

	resp, err := computeStatus(root)
	if err != nil {
		t.Fatalf("computeStatus returned error: %v", err)
	}

	if resp.Dirty {
		t.Fatalf("expected Dirty to be false for clean git repo")
	}
	if resp.ChangedFiles != 0 {
		t.Fatalf("expected ChangedFiles to be 0 for clean git repo, got %d", resp.ChangedFiles)
	}
	if !resp.GitRepo {
		t.Fatalf("expected GitRepo=true")
	}
	if !resp.WorktreeClean {
		t.Fatalf("expected WorktreeClean=true")
	}
	if !resp.HeadMatches {
		t.Fatalf("expected HeadMatches=true")
	}
}

func TestComputeStatusGitDirtyRepodexOnly(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := setupGitRepoWithIndex(t, false, false)

	// Modify only .repodex to make git dirty in a repodex-only way.
	metaPath := store.MetaPath(root)
	b, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	if err := os.WriteFile(metaPath, append(b, '\n'), 0o644); err != nil {
		t.Fatalf("modify meta: %v", err)
	}

	resp, err := computeStatus(root)
	if err != nil {
		t.Fatalf("computeStatus returned error: %v", err)
	}
	if !resp.GitRepo {
		t.Fatalf("expected GitRepo=true")
	}
	if resp.WorktreeClean {
		t.Fatalf("expected WorktreeClean=false")
	}
	if !resp.GitDirtyRepodexOnly {
		t.Fatalf("expected GitDirtyRepodexOnly=true")
	}
	if resp.GitDirtyPathCount == 0 {
		t.Fatalf("expected GitDirtyPathCount>0")
	}
}

func TestComputeStatusSkipsFastPathWhenGitDirty(t *testing.T) {
	root := setupGitRepoWithIndex(t, true, false)

	samplePath := filepath.Join(root, "sample.ts")
	if err := os.WriteFile(samplePath, []byte("const x = 2;\nconst y = 3;\n"), 0o644); err != nil {
		t.Fatalf("failed to modify sample file: %v", err)
	}

	resp, err := computeStatus(root)
	if err != nil {
		t.Fatalf("computeStatus returned error: %v", err)
	}

	if !resp.Dirty {
		t.Fatalf("expected Dirty to be true for git repo with local changes")
	}
	if resp.ChangedFiles == 0 {
		t.Fatalf("expected ChangedFiles to be greater than 0 for dirty git repo")
	}
}

func setupGitRepoWithIndex(t *testing.T, ignoreRepodex bool, corruptFiles bool) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")
	if ignoreRepodex {
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".repodex\n"), 0o644); err != nil {
			t.Fatalf("failed to write gitignore: %v", err)
		}
	}

	samplePath := filepath.Join(root, "sample.ts")
	if err := os.WriteFile(samplePath, []byte("const x = 1;\n"), 0o644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}

	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	if err := runInit(root, false); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	if err := runIndexSync(root); err != nil {
		t.Fatalf("runIndexSync failed: %v", err)
	}
	if !ignoreRepodex {
		runGit(t, root, "add", ".repodex")
		runGit(t, root, "commit", "-m", "index")
	}
	if corruptFiles {
		if err := os.WriteFile(store.FilesPath(root), []byte("corrupt\n"), 0o644); err != nil {
			t.Fatalf("failed to corrupt files.dat: %v", err)
		}
	}

	return root
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	requireGit(t)
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func setupGitRepo(t *testing.T, ignoreRepodex bool) string {
	t.Helper()
	requireGit(t)

	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")
	if ignoreRepodex {
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".repodex\n"), 0o644); err != nil {
			t.Fatalf("failed to write gitignore: %v", err)
		}
	}
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")
	return root
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}
