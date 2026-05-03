package genie

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestInMemorySession_PolicyAccessors covers the new getters/setters
// added so the host can pass per-turn denied / read-only policy and
// the opaque commit author identity through the session.
func TestInMemorySession_PolicyAccessors(t *testing.T) {
	sess := NewSession("/home", "/work", []string{"/extra"}, nil, nil).(*InMemorySession)

	// Initial state
	assert.Empty(t, sess.GetDeniedPaths())
	assert.Empty(t, sess.GetReadOnlyPaths())
	name, email := sess.GetCommitAuthor()
	assert.Empty(t, name)
	assert.Empty(t, email)

	// Setters
	sess.SetDeniedPaths([]string{".mutiro/**"})
	sess.SetReadOnlyPaths([]string{"shared/**"})
	sess.SetCommitAuthor("conv-2bfe5f1a", "conv-2bfe5f1a@actors.mutiro.local")

	assert.Equal(t, []string{".mutiro/**"}, sess.GetDeniedPaths())
	assert.Equal(t, []string{"shared/**"}, sess.GetReadOnlyPaths())
	name, email = sess.GetCommitAuthor()
	assert.Equal(t, "conv-2bfe5f1a", name)
	assert.Equal(t, "conv-2bfe5f1a@actors.mutiro.local", email)
}

// TestApplySessionContext is the load-bearing assertion for the genie
// SDK changes: every per-turn value the host configures on the session
// must reach the tool call's context with the correct key the genie
// tools already read.
func TestApplySessionContext(t *testing.T) {
	sess := NewSession("/home", "/work", []string{"/extra"}, nil, nil).(*InMemorySession)
	sess.SetDeniedPaths([]string{".mutiro/**", ".mutiro-agent.yaml"})
	sess.SetReadOnlyPaths([]string{"shared/**"})
	sess.SetCommitAuthor("conv-2bfe5f1a", "conv-2bfe5f1a@actors.mutiro.local")

	ctx := applySessionContext(context.Background(), sess)

	assert.Equal(t, "/work", ctx.Value("cwd"))
	assert.Equal(t, "/home", ctx.Value("genie_home"))
	assert.Equal(t, []string{"/extra"}, ctx.Value("allowed_dirs"))
	assert.Equal(t, []string{".mutiro/**", ".mutiro-agent.yaml"}, ctx.Value("denied_paths"))
	assert.Equal(t, []string{"shared/**"}, ctx.Value("read_only_paths"))
	assert.Equal(t, "conv-2bfe5f1a", ctx.Value("commit_author_name"))
	assert.Equal(t, "conv-2bfe5f1a@actors.mutiro.local", ctx.Value("commit_author_email"))
}

// TestApplySessionContext_OmitsEmptyOptionals confirms unconfigured
// fields don't pollute ctx with empty strings/slices — tool callers
// can rely on the keys being absent rather than zero-valued.
func TestApplySessionContext_OmitsEmptyOptionals(t *testing.T) {
	sess := NewSession("", "/work", nil, nil, nil)

	ctx := applySessionContext(context.Background(), sess)

	assert.Equal(t, "/work", ctx.Value("cwd"), "cwd is always set, even when other fields are absent")
	assert.Nil(t, ctx.Value("genie_home"), "empty genie_home should not be set on ctx")
	assert.Nil(t, ctx.Value("allowed_dirs"), "empty allowed_dirs should not be set on ctx")
	assert.Nil(t, ctx.Value("denied_paths"))
	assert.Nil(t, ctx.Value("read_only_paths"))
	assert.Nil(t, ctx.Value("commit_author_name"))
	assert.Nil(t, ctx.Value("commit_author_email"))
}

// TestStartOptions_PolicyOptions covers the new WithDeniedPaths /
// WithReadOnlyPaths / WithCommitAuthor builder helpers.
func TestStartOptions_PolicyOptions(t *testing.T) {
	opts := applyStartOptions(
		WithDeniedPaths(".mutiro/**", "owner/**"),
		WithReadOnlyPaths("shared/**"),
		WithCommitAuthor("conv-2bfe5f1a", "conv-2bfe5f1a@actors.mutiro.local"),
	)

	assert.Equal(t, []string{".mutiro/**", "owner/**"}, opts.deniedPaths)
	assert.Equal(t, []string{"shared/**"}, opts.readOnlyPaths)
	assert.Equal(t, "conv-2bfe5f1a", opts.commitAuthorName)
	assert.Equal(t, "conv-2bfe5f1a@actors.mutiro.local", opts.commitAuthorEmail)
}

// TestStartOptions_PolicyOptions_SkipsEmpty confirms passing empty
// strings doesn't pollute the option lists.
func TestStartOptions_PolicyOptions_SkipsEmpty(t *testing.T) {
	opts := applyStartOptions(
		WithDeniedPaths("", ".git/**", ""),
		WithReadOnlyPaths(""),
	)

	assert.Equal(t, []string{".git/**"}, opts.deniedPaths)
	assert.Empty(t, opts.readOnlyPaths)
}
