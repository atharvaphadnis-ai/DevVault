package gitsvc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func requireGit(t *testing.T) {
	t.Helper()
	if !Available() {
		t.Skip("git not installed")
	}
}

func tempRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "initial commit")
	return dir
}

func TestStatusLogBranches(t *testing.T) {
	dir := tempRepo(t)

	st, err := Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Clean || st.Branch != "main" {
		t.Fatalf("unexpected status: %+v", st)
	}

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err = Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Clean || len(st.Entries) != 1 || st.Entries[0].Path != "new.txt" {
		t.Fatalf("status entries: %+v", st.Entries)
	}

	logs, err := Log(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Subject != "initial commit" {
		t.Fatalf("log: %+v", logs)
	}

	if err := Stage(dir, []string{"new.txt"}); err != nil {
		t.Fatal(err)
	}
	st, _ = Status(dir)
	if !st.Entries[0].Staged {
		t.Fatalf("expected staged entry: %+v", st.Entries)
	}
	if _, err := Commit(dir, "add new file", false); err != nil {
		t.Fatal(err)
	}
	st, _ = Status(dir)
	if !st.Clean {
		t.Fatalf("expected clean after commit: %+v", st)
	}

	if err := Checkout(dir, "feature/x", true); err != nil {
		t.Fatal(err)
	}
	bs, err := Branches(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range bs {
		if b.Name == "feature/x" && b.Current {
			found = true
		}
	}
	if !found {
		t.Fatalf("branch list: %+v", bs)
	}
	if err := Checkout(dir, "main", false); err != nil {
		t.Fatal(err)
	}
	if err := DeleteBranch(dir, "feature/x", false); err != nil {
		t.Fatal(err)
	}
}

func TestDiffAndRejectTraversal(t *testing.T) {
	dir := tempRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Stage(dir, []string{"f.txt"}); err != nil {
		t.Fatal(err)
	}
	d, err := Diff(dir, "f.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "+a") {
		t.Fatalf("diff: %q", d)
	}
	// path traversal must be rejected
	if err := Stage(dir, []string{"../../etc/passwd"}); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestNotARepo(t *testing.T) {
	if _, err := Status(t.TempDir()); err == nil {
		t.Fatal("expected not-a-repo error")
	}
}

func TestCloneValidation(t *testing.T) {
	if _, err := Clone(context.Background(), "-oProxyCommand=evil", ""); err == nil {
		t.Fatal("expected URL validation error")
	}
}

var _ = time.Second
