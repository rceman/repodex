package app

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/memkit/repodex/internal/statusx"
	"github.com/memkit/repodex/internal/store"
)

func TestGitChangeDetectionAndSyncPlan(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")

	// Ignore .repodex to keep the worktree clean after syncs.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".repodex\n"), 0o644); err != nil {
		t.Fatalf("failed to write gitignore: %v", err)
	}

	aPath := filepath.Join(root, "a.ts")
	if err := os.WriteFile(aPath, []byte("const a = 1;\n"), 0o644); err != nil {
		t.Fatalf("failed to write a.ts: %v", err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	if err := runInit(root, false); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	if err := runIndexSync(root); err != nil {
		t.Fatalf("runIndexSync failed: %v", err)
	}

	// 1) baseline noop.
	resp := statusMust(t, root)
	if resp.GitChangedPathCount != 0 {
		t.Fatalf("expected GitChangedPathCount=0, got %d", resp.GitChangedPathCount)
	}
	if resp.GitChangedReason != "none" {
		t.Fatalf("expected GitChangedReason=none, got %s", resp.GitChangedReason)
	}
	if resp.SyncPlan == nil || resp.SyncPlan.Mode != statusx.ModeNoop {
		t.Fatalf("expected SyncPlan noop at baseline, got %#v", resp.SyncPlan)
	}
	assertDirtyMatchesPlan(t, resp)

	// 2) dirty non-indexable change.
	notesPath := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(notesPath, []byte("note\n"), 0o644); err != nil {
		t.Fatalf("failed to write notes.txt: %v", err)
	}
	resp = statusMust(t, root)
	if resp.GitChangedReason != "worktree" {
		t.Fatalf("expected GitChangedReason=worktree for non-indexable change, got %s", resp.GitChangedReason)
	}
	if resp.GitChangedPathCount != 0 {
		t.Fatalf("expected GitChangedPathCount=0 for non-indexable change, got %d", resp.GitChangedPathCount)
	}
	if resp.GitChangedIndexable {
		t.Fatalf("expected GitChangedIndexable=false for non-indexable change")
	}
	if resp.SyncPlan == nil || resp.SyncPlan.Mode != statusx.ModeFull {
		t.Fatalf("expected SyncPlan full when worktree dirty but non-indexable, got %#v", resp.SyncPlan)
	}
	if resp.SyncPlan != nil && resp.SyncPlan.Why != statusx.WhyGitChangedNonIndexable {
		t.Fatalf("expected SyncPlan why=git_changed_non_indexable, got %s", resp.SyncPlan.Why)
	}
	if resp.ChangedFiles != 0 {
		t.Fatalf("expected ChangedFiles=0 for non-indexable change in git repo, got %d", resp.ChangedFiles)
	}
	assertDirtyMatchesPlan(t, resp)
	if err := os.Remove(notesPath); err != nil {
		t.Fatalf("failed to remove notes.txt: %v", err)
	}

	// 3) unstaged change.
	if err := os.WriteFile(aPath, []byte("const a = 2;\n"), 0o644); err != nil {
		t.Fatalf("failed to modify a.ts: %v", err)
	}
	resp = statusMust(t, root)
	if resp.GitChangedReason != "worktree" {
		t.Fatalf("expected GitChangedReason=worktree for unstaged change, got %s", resp.GitChangedReason)
	}
	if resp.SyncPlan == nil || resp.SyncPlan.Mode != statusx.ModeFull {
		t.Fatalf("expected SyncPlan full when worktree dirty, got %#v", resp.SyncPlan)
	}
	assertContainsPath(t, resp.GitChangedPaths, "a.ts")
	assertDirtyMatchesPlan(t, resp)

	// 4) staged change.
	runGit(t, root, "add", "a.ts")
	resp = statusMust(t, root)
	if resp.GitChangedReason != "worktree" {
		t.Fatalf("expected GitChangedReason=worktree for staged change, got %s", resp.GitChangedReason)
	}
	assertContainsPath(t, resp.GitChangedPaths, "a.ts")
	assertDirtyMatchesPlan(t, resp)

	// 5) untracked file.
	bPath := filepath.Join(root, "b.ts")
	if err := os.WriteFile(bPath, []byte("const b = 1;\n"), 0o644); err != nil {
		t.Fatalf("failed to write b.ts: %v", err)
	}
	resp = statusMust(t, root)
	assertContainsPath(t, resp.GitChangedPaths, "b.ts")
	if resp.GitChangedReason != "worktree" {
		t.Fatalf("expected GitChangedReason=worktree with untracked files, got %s", resp.GitChangedReason)
	}
	assertDirtyMatchesPlan(t, resp)

	// 6) head changed.
	if err := os.Remove(bPath); err != nil {
		t.Fatalf("failed to remove b.ts: %v", err)
	}
	runGit(t, root, "commit", "-m", "update a")
	resp = statusMust(t, root)
	if resp.GitChangedReason != "head" {
		t.Fatalf("expected GitChangedReason=head after commit, got %s", resp.GitChangedReason)
	}
	assertContainsPath(t, resp.GitChangedPaths, "a.ts")
	assertDirtyMatchesPlan(t, resp)

	// 7) sync no-op when up-to-date.
	if err := runIndexSync(root); err != nil {
		t.Fatalf("runIndexSync (refresh) failed: %v", err)
	}
	resp = statusMust(t, root)
	if resp.SyncPlan == nil || resp.SyncPlan.Mode != statusx.ModeNoop {
		t.Fatalf("expected SyncPlan noop after refresh, got %#v", resp.SyncPlan)
	}
	assertDirtyMatchesPlan(t, resp)
	metaPath := store.MetaPath(root)
	beforeBytes, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read meta: %v", err)
	}
	beforeInfo, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("failed to stat meta: %v", err)
	}
	if err := runIndexSync(root); err != nil {
		t.Fatalf("runIndexSync (noop) failed: %v", err)
	}
	afterBytes, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read meta after noop: %v", err)
	}
	afterInfo, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("failed to stat meta after noop: %v", err)
	}
	if !bytes.Equal(beforeBytes, afterBytes) {
		t.Fatalf("expected meta bytes unchanged after noop sync")
	}
	if !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
		t.Fatalf("expected meta mtime unchanged after noop sync")
	}
}

func statusMust(t *testing.T, root string) StatusResponse {
	t.Helper()
	resp, err := computeStatus(root)
	if err != nil {
		t.Fatalf("computeStatus returned error: %v", err)
	}
	if !statusx.IsValidGitChangedReason(resp.GitChangedReason) {
		t.Fatalf("invalid git changed reason: %s", resp.GitChangedReason)
	}
	if resp.SyncPlan != nil {
		if !statusx.IsValidMode(resp.SyncPlan.Mode) {
			t.Fatalf("invalid sync plan mode: %s", resp.SyncPlan.Mode)
		}
		if !statusx.IsValidWhy(resp.SyncPlan.Why) {
			t.Fatalf("invalid sync plan why: %s", resp.SyncPlan.Why)
		}
	}
	return resp
}

func assertContainsPath(t *testing.T, paths []string, target string) {
	t.Helper()
	for _, p := range paths {
		if p == target {
			return
		}
	}
	t.Fatalf("expected %s to be present in %+v", target, paths)
}

func TestRepodexOnlyDirtySyncPlanNoop(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")

	aPath := filepath.Join(root, "a.ts")
	if err := os.WriteFile(aPath, []byte("const a = 1;\n"), 0o644); err != nil {
		t.Fatalf("failed to write a.ts: %v", err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "initial")

	if err := runInit(root, false); err != nil {
		t.Fatalf("runInit failed: %v", err)
	}
	if err := runIndexSync(root); err != nil {
		t.Fatalf("runIndexSync failed: %v", err)
	}

	metaPath := store.MetaPath(root)
	origMeta, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read meta: %v", err)
	}
	origInfo, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("failed to stat meta: %v", err)
	}

	resp := statusMust(t, root)
	if !resp.GitDirtyRepodexOnly {
		t.Fatalf("expected GitDirtyRepodexOnly=true")
	}
	if resp.SyncPlan == nil || resp.SyncPlan.Mode != statusx.ModeNoop {
		t.Fatalf("expected SyncPlan noop for repodex-only dirt, got %#v", resp.SyncPlan)
	}
	if resp.SyncPlan != nil && resp.SyncPlan.Why != statusx.WhyGitChangedNonIndexable {
		t.Fatalf("expected SyncPlan why=git_changed_non_indexable, got %s", resp.SyncPlan.Why)
	}
	if resp.Dirty {
		t.Fatalf("expected Dirty=false for repodex-only dirt")
	}

	// ensure GitDirtyPathCount still reflects porcelain.
	if resp.GitDirtyPathCount == 0 {
		t.Fatalf("expected GitDirtyPathCount>0 for repodex-only dirt")
	}

	if err := runIndexSync(root); err != nil {
		t.Fatalf("runIndexSync (repodex dirty) failed: %v", err)
	}

	afterMeta, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("failed to read meta after noop: %v", err)
	}
	afterInfo, err := os.Stat(metaPath)
	if err != nil {
		t.Fatalf("failed to stat meta after noop: %v", err)
	}
	if !bytes.Equal(afterMeta, origMeta) {
		t.Fatalf("expected meta bytes unchanged after noop sync when repodex dirty (untracked)")
	}
	if !afterInfo.ModTime().Equal(origInfo.ModTime()) {
		t.Fatalf("expected meta mtime unchanged after noop sync when repodex dirty (untracked)")
	}
}

func assertDirtyMatchesPlan(t *testing.T, resp StatusResponse) {
	t.Helper()
	if resp.SyncPlan == nil {
		return
	}
	wantDirty := resp.SyncPlan.Mode != statusx.ModeNoop
	if resp.Dirty != wantDirty {
		t.Fatalf("expected Dirty=%v to match SyncPlan mode %s", wantDirty, resp.SyncPlan.Mode)
	}
}
