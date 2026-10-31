// Package gitsvc wraps the system git executable. DevVault deliberately uses
// the installed git binary (no embedded git) so behaviour matches the
// developer's environment; every operation is explicit and driven from the UI.
package gitsvc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrGitMissing is returned when no git executable is on PATH.
var ErrGitMissing = errors.New("git executable not found on PATH. Install git (https://git-scm.com) and restart DevVault")

// ErrNotARepo is returned when the directory is not inside a git work tree.
var ErrNotARepo = errors.New("not a git repository")

func findGit() (string, error) {
	p, err := exec.LookPath("git")
	if err != nil {
		return "", ErrGitMissing
	}
	return p, nil
}

// Available reports whether git is installed.
func Available() bool {
	_, err := findGit()
	return err == nil
}

func run(ctx context.Context, dir string, args ...string) (string, error) {
	git, err := findGit()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, git, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args[:min(2, len(args))], " "), msg)
	}
	return out.String(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// EnsureRepo verifies dir is a git work tree.
func EnsureRepo(dir string) error {
	st, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("directory not accessible: %w", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := run(ctx, dir, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return ErrNotARepo
	}
	if strings.TrimSpace(out) != "true" {
		return ErrNotARepo
	}
	return nil
}

// StatusEntry is one changed path.
type StatusEntry struct {
	X      string `json:"x"`
	Y      string `json:"y"`
	Path   string `json:"path"`
	Orig   string `json:"orig,omitempty"`
	Staged bool   `json:"staged"`
}

// StatusResult is the working tree state.
type StatusResult struct {
	Branch   string        `json:"branch"`
	Upstream string        `json:"upstream,omitempty"`
	Ahead    int           `json:"ahead"`
	Behind   int           `json:"behind"`
	Entries  []StatusEntry `json:"entries"`
	Clean    bool          `json:"clean"`
}

var statusRe = regexp.MustCompile(`^(..) (.+)$`)

// Status returns branch and changed files using porcelain v1.
func Status(dir string) (*StatusResult, error) {
	if err := EnsureRepo(dir); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := run(ctx, dir, "status", "--porcelain=v1", "-b")
	if err != nil {
		return nil, err
	}
	st := &StatusResult{Entries: []StatusEntry{}}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			h := strings.TrimPrefix(line, "## ")
			if i := strings.Index(h, "..."); i >= 0 {
				st.Branch = h[:i]
				up := h[i+3:]
				if j := strings.Index(up, " ["); j >= 0 {
					st.Upstream = up[:j]
					tracker := up[j:]
					st.Ahead = extractNum(tracker, "ahead ")
					st.Behind = extractNum(tracker, "behind ")
				} else {
					st.Upstream = up
				}
			} else {
				st.Branch = h
			}
			continue
		}
		m := statusRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		x, y := string(m[1][0]), string(m[1][1])
		rest := m[2]
		path := rest
		orig := ""
		if i := strings.Index(rest, " -> "); i >= 0 {
			orig, path = rest[:i], rest[i+4:]
		}
		path = strings.Trim(path, `"`)
		st.Entries = append(st.Entries, StatusEntry{
			X: x, Y: y, Path: path, Orig: orig,
			Staged: x != "?" && x != " " && x != "U",
		})
	}
	st.Clean = len(st.Entries) == 0
	return st, nil
}

func extractNum(s, prefix string) int {
	i := strings.Index(s, prefix)
	if i < 0 {
		return 0
	}
	rest := s[i+len(prefix):]
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	n, _ := strconv.Atoi(rest[:end])
	return n
}

// CommitEntry is one commit in the log.
type CommitEntry struct {
	Hash    string `json:"hash"`
	Short   string `json:"short"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

// Log returns recent commits.
func Log(dir string, limit int) ([]CommitEntry, error) {
	if err := EnsureRepo(dir); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := run(ctx, dir, "log", "--pretty=format:%H%x09%h%x09%an%x09%ad%x09%s", "--date=iso", "-n", strconv.Itoa(limit))
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits yet") || strings.Contains(err.Error(), "ambiguous argument") {
			return []CommitEntry{}, nil
		}
		return nil, err
	}
	commits := []CommitEntry{}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) != 5 {
			continue
		}
		commits = append(commits, CommitEntry{Hash: parts[0], Short: parts[1], Author: parts[2], Date: parts[3], Subject: parts[4]})
	}
	return commits, nil
}

// Branch describes a local or remote branch.
type Branch struct {
	Name     string `json:"name"`
	Current  bool   `json:"current"`
	IsRemote bool   `json:"isRemote"`
}

// Branches lists branches.
func Branches(dir string) ([]Branch, error) {
	if err := EnsureRepo(dir); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := run(ctx, dir, "branch", "-a")
	if err != nil {
		return nil, err
	}
	bs := []Branch{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cur := strings.HasPrefix(line, "* ")
		line = strings.TrimPrefix(line, "* ")
		line = strings.TrimPrefix(line, "+ ")
		if strings.Contains(line, "HEAD ->") {
			continue
		}
		remote := strings.HasPrefix(line, "remotes/")
		line = strings.TrimPrefix(line, "remotes/")
		bs = append(bs, Branch{Name: line, Current: cur, IsRemote: remote})
	}
	return bs, nil
}

// Stage adds paths to the index. Paths must be inside the repo.
func Stage(dir string, paths []string) error {
	if err := EnsureRepo(dir); err != nil {
		return err
	}
	cleaned := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, err := safeRel(dir, p)
		if err != nil {
			return err
		}
		cleaned = append(cleaned, rel)
	}
	if len(cleaned) == 0 {
		cleaned = []string{"-A"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := run(ctx, dir, append([]string{"add", "--"}, cleaned...)...)
	return err
}

// Unstage removes paths from the index.
func Unstage(dir string, paths []string) error {
	if err := EnsureRepo(dir); err != nil {
		return err
	}
	cleaned := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, err := safeRel(dir, p)
		if err != nil {
			return err
		}
		cleaned = append(cleaned, rel)
	}
	if len(cleaned) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := run(ctx, dir, append([]string{"reset", "--", cleaned[0]}, cleaned[1:]...)...)
	return err
}

// safeRel converts p to a repo-relative path, rejecting traversal outside dir.
func safeRel(dir, p string) (string, error) {
	if filepath.IsAbs(p) {
		rel, err := filepath.Rel(dir, p)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("path outside repository: %s", p)
		}
		return rel, nil
	}
	p = filepath.Clean(p)
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", fmt.Errorf("path outside repository: %s", p)
	}
	return p, nil
}

// Commit commits staged changes.
func Commit(dir, message string, amend bool) (string, error) {
	if err := EnsureRepo(dir); err != nil {
		return "", err
	}
	if strings.TrimSpace(message) == "" && !amend {
		return "", errors.New("commit message is required")
	}
	args := []string{"commit", "-m", message}
	if amend {
		args = []string{"commit", "--amend", "--no-edit"}
		if strings.TrimSpace(message) != "" {
			args = []string{"commit", "--amend", "-m", message}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return run(ctx, dir, args...)
}

// Checkout switches branches, optionally creating it first.
func Checkout(dir, branch string, create bool) error {
	if err := EnsureRepo(dir); err != nil {
		return err
	}
	if !validRef(branch) {
		return fmt.Errorf("invalid branch name: %q", branch)
	}
	args := []string{"checkout"}
	if create {
		args = append(args, "-b")
	}
	args = append(args, branch)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := run(ctx, dir, args...)
	return err
}

// DeleteBranch removes a local branch.
func DeleteBranch(dir, branch string, force bool) error {
	if err := EnsureRepo(dir); err != nil {
		return err
	}
	if !validRef(branch) {
		return fmt.Errorf("invalid branch name: %q", branch)
	}
	flag := "-d"
	if force {
		flag = "-D"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := run(ctx, dir, "branch", flag, branch)
	return err
}

func validRef(s string) bool {
	if s == "" || len(s) > 200 {
		return false
	}
	if strings.ContainsAny(s, " ~^:?*[\\") || strings.HasSuffix(s, ".lock") || strings.HasPrefix(s, "-") {
		return false
	}
	return true
}

// Push pushes a branch; setUpsstream adds -u on first push.
func Push(dir, remote, branch string, setUpstream bool) error {
	if err := EnsureRepo(dir); err != nil {
		return err
	}
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		branch = "HEAD"
	}
	args := []string{"push"}
	if setUpstream {
		args = append(args, "-u")
	}
	args = append(args, remote, branch)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	_, err := run(ctx, dir, args...)
	return err
}

// Pull pulls from a remote.
func Pull(dir, remote, branch string) error {
	if err := EnsureRepo(dir); err != nil {
		return err
	}
	if remote == "" {
		remote = "origin"
	}
	args := []string{"pull", remote}
	if branch != "" {
		args = append(args, branch)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	_, err := run(ctx, dir, args...)
	return err
}

// Fetch fetches all remotes.
func Fetch(dir string) error {
	if err := EnsureRepo(dir); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	_, err := run(ctx, dir, "fetch", "--all", "--prune")
	return err
}

// Clone clones a repository URL into dest.
func Clone(ctx context.Context, url, dest string) (string, error) {
	if strings.TrimSpace(url) == "" {
		return "", errors.New("repository URL is required")
	}
	// Block option injection: URL must not start with '-'.
	if strings.HasPrefix(strings.TrimSpace(url), "-") {
		return "", errors.New("invalid repository URL")
	}
	if dest == "" {
		dest = defaultDest(url)
	}
	abs, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err == nil {
		return "", fmt.Errorf("destination already exists: %s", abs)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
	}
	if _, err := run(ctx, filepath.Dir(abs), "clone", url, abs); err != nil {
		return "", err
	}
	return abs, nil
}

func defaultDest(url string) string {
	base := strings.TrimSuffix(filepath.Base(strings.TrimSuffix(url, "/")), ".git")
	if base == "" || base == "." {
		base = "cloned-repo"
	}
	return base
}

// Diff returns the diff for a path (working tree, or staged when cached).
func Diff(dir, path string, cached bool) (string, error) {
	if err := EnsureRepo(dir); err != nil {
		return "", err
	}
	args := []string{"diff"}
	if cached {
		args = append(args, "--cached")
	}
	if path != "" {
		rel, err := safeRel(dir, path)
		if err != nil {
			return "", err
		}
		args = append(args, "--", rel)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return run(ctx, dir, args...)
}

// Remotes lists configured remotes as name/url pairs.
func Remotes(dir string) (map[string]string, error) {
	if err := EnsureRepo(dir); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := run(ctx, dir, "remote", "-v")
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 && !strings.HasSuffix(line, "(push)") {
			m[parts[0]] = parts[1]
		}
	}
	return m, nil
}

// Init creates a new repository in dir.
func Init(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := run(ctx, dir, "init")
	return err
}
