// Package toolctx defines the typed context contract between the Genie
// session layer and tools.
//
// Session state (working directory, sandbox policy, commit author,
// persona, and execution identifiers) flows from pkg/genie onto every
// tool call's context. This package owns the context keys for that
// contract so producers (the core, clients) and consumers (tools,
// skills, prompt loaders) share one explicit, collision-proof API
// instead of bare string keys.
//
// toolctx is a leaf package: it imports only the standard library and
// must never import pkg/genie, pkg/tools, or any other Genie package.
//
// Every value has a With* setter and a getter returning (value, ok);
// ok reports whether the value was set on the context at all, leaving
// fallback decisions (empty-string handling, os.Getwd, defaults) to
// each call site.
package toolctx

import "context"

type (
	workingDirKey        struct{}
	genieHomeKey         struct{}
	allowedDirsKey       struct{}
	deniedPathsKey       struct{}
	readOnlyPathsKey     struct{}
	commitAuthorNameKey  struct{}
	commitAuthorEmailKey struct{}
	personaKey           struct{}
	sessionIDKey         struct{}
	executionIDKey       struct{}
)

// WithWorkingDir returns a context carrying the session working
// directory (the workspace root tools are bound to).
func WithWorkingDir(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, workingDirKey{}, dir)
}

// WorkingDir returns the session working directory and whether it was set.
func WorkingDir(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(workingDirKey{}).(string)
	return v, ok
}

// WithGenieHome returns a context carrying the genie home directory
// (where .genie/ configuration lives).
func WithGenieHome(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, genieHomeKey{}, dir)
}

// GenieHome returns the genie home directory and whether it was set.
func GenieHome(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(genieHomeKey{}).(string)
	return v, ok
}

// WithAllowedDirs returns a context carrying additional read-allowed
// directories granted to the agent besides the workspace root.
func WithAllowedDirs(ctx context.Context, dirs []string) context.Context {
	return context.WithValue(ctx, allowedDirsKey{}, dirs)
}

// AllowedDirs returns the additional allowed directories and whether
// they were set.
func AllowedDirs(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(allowedDirsKey{}).([]string)
	return v, ok
}

// WithDeniedPaths returns a context carrying glob patterns the agent
// must not touch at all (read or mutate).
func WithDeniedPaths(ctx context.Context, patterns []string) context.Context {
	return context.WithValue(ctx, deniedPathsKey{}, patterns)
}

// DeniedPaths returns the denied path patterns and whether they were set.
func DeniedPaths(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(deniedPathsKey{}).([]string)
	return v, ok
}

// WithReadOnlyPaths returns a context carrying glob patterns the agent
// may read but not mutate.
func WithReadOnlyPaths(ctx context.Context, patterns []string) context.Context {
	return context.WithValue(ctx, readOnlyPathsKey{}, patterns)
}

// ReadOnlyPaths returns the read-only path patterns and whether they
// were set.
func ReadOnlyPaths(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(readOnlyPathsKey{}).([]string)
	return v, ok
}

// WithCommitAuthorName returns a context carrying the commit author
// name the host wants git tools to use.
func WithCommitAuthorName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, commitAuthorNameKey{}, name)
}

// CommitAuthorName returns the commit author name and whether it was set.
func CommitAuthorName(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(commitAuthorNameKey{}).(string)
	return v, ok
}

// WithCommitAuthorEmail returns a context carrying the commit author
// email the host wants git tools to use.
func WithCommitAuthorEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, commitAuthorEmailKey{}, email)
}

// CommitAuthorEmail returns the commit author email and whether it was set.
func CommitAuthorEmail(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(commitAuthorEmailKey{}).(string)
	return v, ok
}

// WithPersona returns a context carrying the active persona ID.
func WithPersona(ctx context.Context, personaID string) context.Context {
	return context.WithValue(ctx, personaKey{}, personaID)
}

// Persona returns the active persona ID and whether it was set.
func Persona(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(personaKey{}).(string)
	return v, ok
}

// WithSessionID returns a context carrying the chat session ID.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// SessionID returns the chat session ID and whether it was set.
func SessionID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(sessionIDKey{}).(string)
	return v, ok
}

// WithExecutionID returns a context carrying the per-tool-call
// execution ID used to correlate tool lifecycle events.
func WithExecutionID(ctx context.Context, executionID string) context.Context {
	return context.WithValue(ctx, executionIDKey{}, executionID)
}

// ExecutionID returns the tool execution ID and whether it was set.
func ExecutionID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(executionIDKey{}).(string)
	return v, ok
}
