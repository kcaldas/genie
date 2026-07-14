package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/toolctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitFixture spins up a real on-disk repo with a few commits so tests
// exercise go-git for real. We don't fake repos — the tools are thin
// wrappers and we want to know they work against the real library.
type gitFixture struct {
	dir       string
	repo      *git.Repository
	commitSha string
}

func newGitFixture(t *testing.T) *gitFixture {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	return &gitFixture{dir: dir, repo: repo}
}

func (f *gitFixture) write(t *testing.T, rel, content string) {
	t.Helper()
	full := filepath.Join(f.dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

func (f *gitFixture) commit(t *testing.T, msg, author, email string) string {
	t.Helper()
	wt, err := f.repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, wt.AddWithOptions(&git.AddOptions{All: true}))
	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: author, Email: email, When: time.Now()},
	})
	require.NoError(t, err)
	f.commitSha = hash.String()
	return hash.String()
}

func contextForGit(workspace, authorName, authorEmail string) context.Context {
	ctx := toolctx.WithWorkingDir(context.Background(), workspace)
	ctx = toolctx.WithCommitAuthorName(ctx, authorName)
	ctx = toolctx.WithCommitAuthorEmail(ctx, authorEmail)
	return ctx
}

// ---------- gitStatus ----------

func TestGitStatus_CleanRepo(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "x")
	f.commit(t, "init", "tester", "tester@example.com")

	handler := NewGitStatusTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "tester@example.com"), map[string]any{
		"_display_message": "status",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.True(t, r["clean"].(bool))
}

func TestGitStatus_DirtyShowsModifications(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "v1")
	f.commit(t, "init", "tester", "tester@example.com")
	f.write(t, "a.txt", "v2")

	handler := NewGitStatusTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "tester@example.com"), map[string]any{
		"_display_message": "status",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.False(t, r["clean"].(bool))
	assert.Contains(t, r["results"].(string), "a.txt")
}

func TestGitStatus_NoRepo(t *testing.T) {
	dir := t.TempDir() // not initialised
	handler := NewGitStatusTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(dir, "x", "x@x"), map[string]any{
		"_display_message": "status",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "no git repository")
}

// ---------- gitLog ----------

func TestGitLog_ReturnsCommitsNewestFirst(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "1")
	first := f.commit(t, "first", "tester", "t@x")
	f.write(t, "a.txt", "2")
	second := f.commit(t, "second", "tester", "t@x")

	handler := NewGitLogTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"_display_message": "log",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	out := r["results"].(string)
	assert.True(t, strings.Index(out, second[:12]) < strings.Index(out, first[:12]),
		"newest commit must come first in log output")
}

func TestGitLog_PathFilter(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "1")
	f.commit(t, "touch a", "tester", "t@x")
	f.write(t, "b.txt", "1")
	f.commit(t, "touch b", "tester", "t@x")

	handler := NewGitLogTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"path":             "a.txt",
		"_display_message": "log a",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	out := r["results"].(string)
	assert.Contains(t, out, "touch a")
	assert.NotContains(t, out, "touch b")
}

// ---------- gitDiff ----------

func TestGitDiff_WorkingTreeShowsDirty(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "v1")
	f.commit(t, "init", "tester", "t@x")
	f.write(t, "a.txt", "v2")

	handler := NewGitDiffTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"_display_message": "diff",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.Contains(t, r["results"].(string), "a.txt")
}

func TestGitDiff_CommitShowsPatch(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "v1\n")
	f.commit(t, "init", "tester", "t@x")
	f.write(t, "a.txt", "v2\n")
	sha := f.commit(t, "second", "tester", "t@x")

	handler := NewGitDiffTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"commit":           sha,
		"_display_message": "diff commit",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	out := r["results"].(string)
	assert.Contains(t, out, "v1")
	assert.Contains(t, out, "v2")
}

// ---------- gitShow ----------

func TestGitShow_ReadsHistoricVersion(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "first version\n")
	first := f.commit(t, "init", "tester", "t@x")
	f.write(t, "a.txt", "second version\n")
	f.commit(t, "second", "tester", "t@x")

	handler := NewGitShowTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"path":             "a.txt",
		"commit":           first,
		"_display_message": "show first",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.Equal(t, "first version\n", r["results"].(string))
}

func TestGitShow_RespectsDeniedPaths(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, ".mutiro-agent.yaml", "secrets")
	f.commit(t, "init", "tester", "t@x")

	ctx := contextForGit(f.dir, "tester", "t@x")
	ctx = toolctx.WithDeniedPaths(ctx, []string{".mutiro-agent.yaml"})

	handler := NewGitShowTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"path":             ".mutiro-agent.yaml",
		"commit":           "HEAD",
		"_display_message": "should be denied",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "denied")
}

// ---------- gitCommit ----------

func TestGitCommit_AttributesAuthorFromContext(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "seed.txt", "x")
	f.commit(t, "init", "platform", "platform@local")
	// Now the agent makes an edit during a turn:
	f.write(t, "seed.txt", "edited by alice")

	ctx := contextForGit(f.dir, "alice", "conv_alice_dm@conversations.mutiro.com")
	handler := NewGitCommitTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"message":          "Alice updated seed",
		"_display_message": "committing",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	head, err := f.repo.Head()
	require.NoError(t, err)
	commit, err := f.repo.CommitObject(head.Hash())
	require.NoError(t, err)
	assert.Equal(t, "alice", commit.Author.Name)
	assert.Equal(t, "conv_alice_dm@conversations.mutiro.com", commit.Author.Email)
	assert.Equal(t, "Alice updated seed", commit.Message)
}

func TestGitCommit_RefusesCleanTree(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "x")
	f.commit(t, "init", "tester", "t@x")

	ctx := contextForGit(f.dir, "alice", "alice@x")
	handler := NewGitCommitTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"message":          "should fail",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "clean")
}

func TestGitCommit_RespectsReadOnly(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "README.md", "v1")
	f.commit(t, "init", "tester", "t@x")
	f.write(t, "README.md", "v2")

	ctx := contextForGit(f.dir, "alice", "alice@x")
	ctx = toolctx.WithReadOnlyPaths(ctx, []string{"README.md"})

	handler := NewGitCommitTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"message":          "should fail",
		"paths":            []any{"README.md"},
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "read-only")
}

// ---------- gitRestore ----------

func TestGitRestore_RecoversPreviousVersion(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "good")
	f.commit(t, "init", "tester", "t@x")
	// Bad edit landed and got committed:
	f.write(t, "a.txt", "BAD")
	f.commit(t, "regression", "tester", "t@x")

	ctx := contextForGit(f.dir, "alice", "alice@x")
	handler := NewGitRestoreTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"path":             "a.txt",
		"commit":           "HEAD~1",
		"_display_message": "rolling back",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	got, err := os.ReadFile(filepath.Join(f.dir, "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "good", string(got))
}

func TestGitRestore_RespectsReadOnly(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "README.md", "v1")
	f.commit(t, "init", "tester", "t@x")
	f.write(t, "README.md", "v2")

	ctx := contextForGit(f.dir, "alice", "alice@x")
	ctx = toolctx.WithReadOnlyPaths(ctx, []string{"README.md"})

	handler := NewGitRestoreTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"path":             "README.md",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "read-only")
}
