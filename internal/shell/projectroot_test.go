package shell

import (
	"os"
	"path/filepath"
	"testing"
)

// mkDirs creates every path element and returns nothing; the paths stay
// as given so the expectations can compare them directly.
func mkDirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// TestFindProjectRootVCS covers the three candidate kinds the walk
// collects and their priority: bottom-up VCS, then the recurring VCS
// run, then the outermost configured marker. Uses match the Rust
// workspace module's find_project_root.
func TestFindProjectRootVCS(t *testing.T) {
	t.Run("git repo without markers", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, "repo")
		deep := filepath.Join(root, "a", "b")
		mkDirs(t, deep, filepath.Join(root, ".git"))
		if got := findProjectRoot(deep, nil); got != root {
			t.Fatalf("findProjectRoot = %q, want %q", got, root)
		}
	})

	t.Run("monorepo git wins over inner go.mod", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, "repo")
		sub := filepath.Join(root, "svc")
		deep := filepath.Join(sub, "api")
		mkDirs(t, deep, filepath.Join(root, ".git"))
		if err := os.WriteFile(filepath.Join(sub, "go.mod"), []byte("module svc\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := findProjectRoot(deep, []string{"go.mod"}); got != root {
			t.Fatalf("findProjectRoot = %q, want %q", got, root)
		}
	})

	t.Run("svn topmost contiguous run bottom", func(t *testing.T) {
		dir := t.TempDir()
		a := filepath.Join(dir, "a")
		ab := filepath.Join(a, "b")
		mkDirs(t, ab, filepath.Join(a, ".svn"), filepath.Join(ab, ".svn"))
		// Both levels sit in .svn; the run's bottom is the deepest of
		// the two, the document's own directory.
		if got := findProjectRoot(ab, nil); got != ab {
			t.Fatalf("findProjectRoot = %q, want %q", got, ab)
		}
	})

	t.Run("svn run stops at gap", func(t *testing.T) {
		dir := t.TempDir()
		a := filepath.Join(dir, "a")
		ab := filepath.Join(a, "b")
		abc := filepath.Join(ab, "c")
		abcd := filepath.Join(abc, "d")
		mkDirs(t, abcd, filepath.Join(a, ".svn"), filepath.Join(ab, ".svn"), filepath.Join(abcd, ".svn"))
		// The gap at c ends the topmost run, so it bottoms out at b.
		if got := findProjectRoot(abcd, nil); got != ab {
			t.Fatalf("findProjectRoot = %q, want %q", got, ab)
		}
	})

	t.Run("hg repo", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, "repo")
		deep := filepath.Join(root, "src")
		mkDirs(t, deep, filepath.Join(root, ".hg"))
		if got := findProjectRoot(deep, nil); got != root {
			t.Fatalf("findProjectRoot = %q, want %q", got, root)
		}
	})

	t.Run("jj repo", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, "repo")
		deep := filepath.Join(root, "src")
		mkDirs(t, deep, filepath.Join(root, ".jj"))
		if got := findProjectRoot(deep, nil); got != root {
			t.Fatalf("findProjectRoot = %q, want %q", got, root)
		}
	})

	t.Run("nested git inside jj", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, "outer")
		inner := filepath.Join(root, "inner")
		deep := filepath.Join(inner, "src")
		mkDirs(t, deep, filepath.Join(root, ".jj"), filepath.Join(inner, ".git"))
		// The walk starts at the document, so the inner .git is the
		// closest VCS root and the outer one never replaces it.
		if got := findProjectRoot(deep, nil); got != inner {
			t.Fatalf("findProjectRoot = %q, want %q", got, inner)
		}
	})

	t.Run("markers outermost wins", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, "proj")
		deep := filepath.Join(root, "app", "web")
		mkDirs(t, deep)
		if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(filepath.Join(root, "app"), "package.json"), []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := findProjectRoot(deep, []string{"Cargo.toml", "package.json"}); got != root {
			t.Fatalf("findProjectRoot = %q, want %q", got, root)
		}
	})

	t.Run("single marker level", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, "proj")
		deep := filepath.Join(root, "src")
		mkDirs(t, deep)
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module proj\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := findProjectRoot(deep, []string{"go.mod"}); got != root {
			t.Fatalf("findProjectRoot = %q, want %q", got, root)
		}
	})

	t.Run("nothing found", func(t *testing.T) {
		dir := t.TempDir()
		deep := filepath.Join(dir, "x", "y")
		mkDirs(t, deep)
		if got := findProjectRoot(deep, []string{"go.mod", ".git"}); got != "" {
			t.Fatalf("findProjectRoot = %q, want empty", got)
		}
	})
}

// TestExplorerRootFallsBackToDocDir keeps the caller's contract: without
// any root above the document, the pane hangs from the document's
// directory even in project_root mode.
func TestExplorerRootFallsBackToDocDir(t *testing.T) {
	s, _ := newShell(t)
	saved := s.Saved
	saved.ExplorerRootMode = "project_root"
	saved.ProjectRootMarkers = nil
	s.Saved = saved

	dir := t.TempDir()
	plain := filepath.Join(dir, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := s.explorerRoot(filepath.Join(plain, "doc.md")); got != plain {
		t.Fatalf("explorerRoot = %q, want %q", got, plain)
	}
}
