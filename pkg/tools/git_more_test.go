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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================================================================
// Cross-cutting: every git tool must require _display_message
// ===========================================================================

func TestGitTools_RequireDisplayMessage(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "x")
	f.commit(t, "init", "tester", "t@x")

	ctx := contextForGit(f.dir, "tester", "t@x")

	tools := []struct {
		name    string
		handler func() func(context.Context, map[string]any) (map[string]any, error)
		minArgs map[string]any
	}{
		{"gitStatus", func() func(context.Context, map[string]any) (map[string]any, error) {
			return NewGitStatusTool(&events.NoOpPublisher{}).Handler()
		}, map[string]any{}},
		{"gitLog", func() func(context.Context, map[string]any) (map[string]any, error) {
			return NewGitLogTool(&events.NoOpPublisher{}).Handler()
		}, map[string]any{}},
		{"gitDiff", func() func(context.Context, map[string]any) (map[string]any, error) {
			return NewGitDiffTool(&events.NoOpPublisher{}).Handler()
		}, map[string]any{}},
		{"gitShow", func() func(context.Context, map[string]any) (map[string]any, error) {
			return NewGitShowTool(&events.NoOpPublisher{}).Handler()
		}, map[string]any{"path": "a.txt", "commit": "HEAD"}},
		{"gitCommit", func() func(context.Context, map[string]any) (map[string]any, error) {
			return NewGitCommitTool(&events.NoOpPublisher{}).Handler()
		}, map[string]any{"message": "x"}},
		{"gitRestore", func() func(context.Context, map[string]any) (map[string]any, error) {
			return NewGitRestoreTool(&events.NoOpPublisher{}).Handler()
		}, map[string]any{"path": "a.txt"}},
	}

	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.handler()(ctx, tc.minArgs)
			require.Error(t, err, "%s should reject missing _display_message", tc.name)
			assert.Contains(t, err.Error(), "_display_message")
		})
	}
}

// ===========================================================================
// gitLog: empty repo, limit cap, no-repo
// ===========================================================================

func TestGitLog_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	handler := NewGitLogTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(dir, "x", "x@x"), map[string]any{
		"_display_message": "empty",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.Equal(t, 0, r["count"].(int))
}

func TestGitLog_LimitCappedAtMax(t *testing.T) {
	f := newGitFixture(t)
	// 5 commits is plenty; we just want to confirm a wild limit doesn't crash.
	for i := 0; i < 5; i++ {
		f.write(t, "a.txt", string(rune('A'+i)))
		f.commit(t, "c", "tester", "t@x")
	}

	handler := NewGitLogTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"limit":            float64(9999),
		"_display_message": "huge limit",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.LessOrEqual(t, r["count"].(int), gitLogMaxLimit)
}

func TestGitLog_NoRepo(t *testing.T) {
	handler := NewGitLogTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(t.TempDir(), "x", "x@x"), map[string]any{
		"_display_message": "no repo",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "no git repository")
}

// ===========================================================================
// gitDiff: empty repo, initial commit, invalid ref, truncation, no-repo
// ===========================================================================

func TestGitDiff_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	handler := NewGitDiffTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(dir, "x", "x@x"), map[string]any{
		"_display_message": "empty repo",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.Contains(t, r["results"].(string), "no commits yet")
}

func TestGitDiff_InitialCommitHasNoParent(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "v1")
	sha := f.commit(t, "init", "tester", "t@x")

	handler := NewGitDiffTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"commit":           sha,
		"_display_message": "diff initial",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	assert.Contains(t, r["results"].(string), "no parent")
}

func TestGitDiff_InvalidCommitRef(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "v1")
	f.commit(t, "init", "tester", "t@x")

	handler := NewGitDiffTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"commit":           "deadbeefdeadbeef",
		"_display_message": "bad ref",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "resolve")
}

func TestGitDiff_NoRepo(t *testing.T) {
	handler := NewGitDiffTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(t.TempDir(), "x", "x@x"), map[string]any{
		"_display_message": "no repo",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "no git repository")
}

// ===========================================================================
// gitShow: missing file at commit, size cap, read-only readable, invalid ref
// ===========================================================================

func TestGitShow_PathNotAtCommit(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "first.txt", "v1")
	first := f.commit(t, "init", "tester", "t@x")
	f.write(t, "later.txt", "v1")
	f.commit(t, "later", "tester", "t@x")

	handler := NewGitShowTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"path":             "later.txt",
		"commit":           first,
		"_display_message": "not yet",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "not found")
}

func TestGitShow_FileTooLarge(t *testing.T) {
	f := newGitFixture(t)
	big := strings.Repeat("x", int(gitShowMaxFileSize)+10)
	f.write(t, "big.bin", big)
	f.commit(t, "big", "tester", "t@x")

	handler := NewGitShowTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"path":             "big.bin",
		"commit":           "HEAD",
		"_display_message": "too big",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "exceeds")
}

func TestGitShow_ReadOnlyPathStillReadable(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "README.md", "v1")
	f.commit(t, "init", "tester", "t@x")

	ctx := contextForGit(f.dir, "tester", "t@x")
	ctx = context.WithValue(ctx, "read_only_paths", []string{"README.md"})

	handler := NewGitShowTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"path":             "README.md",
		"commit":           "HEAD",
		"_display_message": "should be readable",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool), "read_only_paths must allow reads")
	assert.Equal(t, "v1", r["results"].(string))
}

func TestGitShow_InvalidCommitRef(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "v1")
	f.commit(t, "init", "tester", "t@x")

	handler := NewGitShowTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "tester", "t@x"), map[string]any{
		"path":             "a.txt",
		"commit":           "0123456789abcdef0123456789abcdef01234567",
		"_display_message": "bad ref",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
}

func TestGitShow_NoRepo(t *testing.T) {
	handler := NewGitShowTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(t.TempDir(), "x", "x@x"), map[string]any{
		"path":             "missing.txt",
		"commit":           "HEAD",
		"_display_message": "no repo",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "no git repository")
}

// ===========================================================================
// gitCommit: cross-repo refusal (THE big one), explicit paths, author
// fallback, empty message, no-repo
// ===========================================================================

// TestGitCommit_RefusesToSpanRepos is the load-bearing test. With a
// nested repo layout (owner at root, conversations beneath, gitignored
// from the outer), gitCommit must refuse a single call that mixes
// paths from two repos. Without this, owner-edits-Alice would silently
// land in the wrong history.
func TestGitCommit_RefusesToSpanRepos(t *testing.T) {
	workspace := t.TempDir()

	outerRepo, err := git.PlainInit(workspace, false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "outer.txt"), []byte("o"), 0o644))
	owt, err := outerRepo.Worktree()
	require.NoError(t, err)
	require.NoError(t, owt.AddWithOptions(&git.AddOptions{All: true}))
	_, err = owt.Commit("init outer", &git.CommitOptions{
		Author: &object.Signature{Name: "owner", Email: "o@x", When: time.Now()},
	})
	require.NoError(t, err)

	nested := filepath.Join(workspace, "nested")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	innerRepo, err := git.PlainInit(nested, false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(nested, "inner.txt"), []byte("i"), 0o644))
	iwt, err := innerRepo.Worktree()
	require.NoError(t, err)
	require.NoError(t, iwt.AddWithOptions(&git.AddOptions{All: true}))
	_, err = iwt.Commit("init inner", &git.CommitOptions{
		Author: &object.Signature{Name: "inner", Email: "i@x", When: time.Now()},
	})
	require.NoError(t, err)

	// Now make both dirty.
	require.NoError(t, os.WriteFile(filepath.Join(workspace, "outer.txt"), []byte("o2"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(nested, "inner.txt"), []byte("i2"), 0o644))

	ctx := contextForGit(workspace, "alice", "alice@x")
	handler := NewGitCommitTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"message":          "spans repos — should be refused",
		"paths":            []any{"outer.txt", "nested/inner.txt"},
		"_display_message": "spans repos",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool), "must refuse cross-repo commit")
	assert.Contains(t, r["error"].(string), "refuses to span repos")

	// Both repos must be untouched (no new commit landed in either).
	outerHead, err := outerRepo.Head()
	require.NoError(t, err)
	innerHead, err := innerRepo.Head()
	require.NoError(t, err)
	outerCommit, err := outerRepo.CommitObject(outerHead.Hash())
	require.NoError(t, err)
	innerCommit, err := innerRepo.CommitObject(innerHead.Hash())
	require.NoError(t, err)
	assert.Equal(t, "init outer", outerCommit.Message)
	assert.Equal(t, "init inner", innerCommit.Message)
}

func TestGitCommit_ExplicitPaths(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "1")
	f.write(t, "b.txt", "1")
	f.commit(t, "init", "tester", "t@x")

	f.write(t, "a.txt", "2") // modify a
	f.write(t, "b.txt", "2") // modify b

	handler := NewGitCommitTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "alice", "alice@x"), map[string]any{
		"message":          "only a",
		"paths":            []any{"a.txt"},
		"_display_message": "explicit",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	// b.txt should still be dirty — not picked up by the targeted commit.
	wt, _ := f.repo.Worktree()
	st, _ := wt.Status()
	assert.False(t, st.IsClean(), "b.txt should still be unstaged after explicit-path commit on a.txt only")
}

func TestGitCommit_AuthorFallbackWhenContextUnset(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "1")
	f.commit(t, "init", "tester", "t@x")
	f.write(t, "a.txt", "2")

	// No commit_author_* set on context.
	ctx := context.WithValue(context.Background(), "cwd", f.dir)
	handler := NewGitCommitTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"message":          "fallback author",
		"_display_message": "fallback",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	head, _ := f.repo.Head()
	commit, _ := f.repo.CommitObject(head.Hash())
	assert.Equal(t, "mutiro-agent", commit.Author.Name)
	assert.Equal(t, "noreply@mutiro.local", commit.Author.Email)
}

func TestGitCommit_EmptyMessageRejected(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "1")
	f.commit(t, "init", "tester", "t@x")
	f.write(t, "a.txt", "2")

	handler := NewGitCommitTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "alice", "alice@x"), map[string]any{
		"message":          "   ",
		"_display_message": "blank message",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "non-empty")
}

func TestGitCommit_RespectsDeniedPaths(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, ".mutiro-agent.yaml", "v1")
	f.commit(t, "init", "tester", "t@x")
	f.write(t, ".mutiro-agent.yaml", "tampered")

	ctx := contextForGit(f.dir, "alice", "alice@x")
	ctx = context.WithValue(ctx, "denied_paths", []string{".mutiro-agent.yaml"})

	handler := NewGitCommitTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"message":          "tampering",
		"paths":            []any{".mutiro-agent.yaml"},
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "denied")
}

func TestGitCommit_NoRepo(t *testing.T) {
	handler := NewGitCommitTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(t.TempDir(), "x", "x@x"), map[string]any{
		"message":          "x",
		"_display_message": "no repo",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "no git repository")
}

// ===========================================================================
// gitRestore: default to HEAD, denied path, missing-at-commit, no-repo
// ===========================================================================

func TestGitRestore_DefaultsToHEAD(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "a.txt", "good")
	f.commit(t, "init", "tester", "t@x")

	// Unsaved corruption that should be undoable via gitRestore (no commit needed).
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, "a.txt"), []byte("BAD"), 0o644))

	handler := NewGitRestoreTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "alice", "alice@x"), map[string]any{
		"path": "a.txt",
		// no commit param — should default to HEAD
		"_display_message": "undo",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))

	got, _ := os.ReadFile(filepath.Join(f.dir, "a.txt"))
	assert.Equal(t, "good", string(got))
}

func TestGitRestore_DeniedPath(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, ".mutiro-agent.yaml", "v1")
	f.commit(t, "init", "tester", "t@x")
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, ".mutiro-agent.yaml"), []byte("tampered"), 0o644))

	ctx := contextForGit(f.dir, "alice", "alice@x")
	ctx = context.WithValue(ctx, "denied_paths", []string{".mutiro-agent.yaml"})

	handler := NewGitRestoreTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"path":             ".mutiro-agent.yaml",
		"_display_message": "should fail",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "denied")
	got, _ := os.ReadFile(filepath.Join(f.dir, ".mutiro-agent.yaml"))
	assert.Equal(t, "tampered", string(got), "denied file must not be touched even on restore")
}

func TestGitRestore_PathNotAtCommit(t *testing.T) {
	f := newGitFixture(t)
	f.write(t, "exists.txt", "v1")
	first := f.commit(t, "init", "tester", "t@x")
	f.write(t, "later.txt", "v1")
	f.commit(t, "added", "tester", "t@x")
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, "later.txt"), []byte("dirty"), 0o644))

	handler := NewGitRestoreTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(f.dir, "alice", "alice@x"), map[string]any{
		"path":             "later.txt",
		"commit":           first,
		"_display_message": "not yet at first",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "not found")
}

func TestGitRestore_NoRepo(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o644))

	handler := NewGitRestoreTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(contextForGit(dir, "x", "x@x"), map[string]any{
		"path":             "x.txt",
		"_display_message": "no repo",
	})
	require.NoError(t, err)
	assert.False(t, r["success"].(bool))
	assert.Contains(t, r["error"].(string), "no git repository")
}

// ===========================================================================
// `repo` parameter: can target a different repo than cwd
// ===========================================================================

// TestGit_RepoParameterTargetsExplicitRepo proves the owner-cross-cutting
// use case: parent cwd, query a child conversation repo by passing
// `repo="conversations/conv_xyz"`.
func TestGit_RepoParameterTargetsExplicitRepo(t *testing.T) {
	parent := t.TempDir()

	outerRepo, err := git.PlainInit(parent, false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(parent, "owner.txt"), []byte("o"), 0o644))
	owt, _ := outerRepo.Worktree()
	require.NoError(t, owt.AddWithOptions(&git.AddOptions{All: true}))
	_, err = owt.Commit("owner: init", &git.CommitOptions{
		Author: &object.Signature{Name: "owner", Email: "o@x", When: time.Now()},
	})
	require.NoError(t, err)

	conv := filepath.Join(parent, "conversations", "conv_alice_dm")
	require.NoError(t, os.MkdirAll(conv, 0o755))
	innerRepo, err := git.PlainInit(conv, false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(conv, "notes.md"), []byte("hi"), 0o644))
	iwt, _ := innerRepo.Worktree()
	require.NoError(t, iwt.AddWithOptions(&git.AddOptions{All: true}))
	_, err = iwt.Commit("alice: hello", &git.CommitOptions{
		Author: &object.Signature{Name: "conv_alice_dm", Email: "alice@conv", When: time.Now()},
	})
	require.NoError(t, err)

	// cwd is parent (owner's view), but query alice's repo via `repo` param.
	ctx := contextForGit(parent, "owner", "owner@x")
	handler := NewGitLogTool(&events.NoOpPublisher{}).Handler()
	r, err := handler(ctx, map[string]any{
		"repo":             "conversations/conv_alice_dm",
		"_display_message": "alice's history from owner cwd",
	})
	require.NoError(t, err)
	assert.True(t, r["success"].(bool))
	out := r["results"].(string)
	assert.Contains(t, out, "alice: hello", "should see alice's commit")
	assert.NotContains(t, out, "owner: init", "should not see owner's commit when querying alice's repo")
}
