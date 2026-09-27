package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/require"
)

func TestDirectoryOperationsPreflightProtectedDescendants(t *testing.T) {
	for _, tc := range []struct {
		name string
		tool func(events.Publisher) Tool
		args map[string]any
	}{
		{"remove", NewRmTool, map[string]any{"path": ".genie", "recursive": "true"}},
		{"move source", NewMvTool, map[string]any{"source": ".genie", "destination": "moved"}},
		{"move overwrite", NewMvTool, map[string]any{"source": "replacement", "destination": ".genie", "overwrite": "true"}},
		{"copy overwrite", NewCpTool, map[string]any{"source": "replacement", "destination": ".genie", "overwrite": "true"}},
	} {
		for _, denied := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{true: " denied", false: " readonly"}[denied], func(t *testing.T) {
				root := t.TempDir()
				require.NoError(t, os.MkdirAll(filepath.Join(root, ".genie/hooks"), 0755))
				require.NoError(t, os.MkdirAll(filepath.Join(root, "replacement"), 0755))
				require.NoError(t, os.WriteFile(filepath.Join(root, ".genie/a-unprotected.txt"), []byte("keep"), 0644))
				require.NoError(t, os.WriteFile(filepath.Join(root, ".genie/hooks/hooks.js"), []byte("guard"), 0644))
				require.NoError(t, os.WriteFile(filepath.Join(root, "replacement/new.txt"), []byte("new"), 0644))
				ro, deny := []string{".genie/hooks/**"}, []string(nil)
				if denied {
					deny, ro = ro, nil
				}
				ctx := contextWithPolicy(root, deny, ro)
				tc.args["_display_message"] = "testing directory policy"
				out, err := tc.tool(&events.NoOpPublisher{}).Handler()(ctx, tc.args)
				require.NoError(t, err)
				require.True(t, out.IsError, "%v", out.Details)
				require.Contains(t, out.Details["error"], "hooks")
				for _, file := range []string{".genie/hooks/hooks.js", ".genie/a-unprotected.txt", "replacement/new.txt"} {
					require.FileExists(t, filepath.Join(root, file))
				}
				require.NoDirExists(t, filepath.Join(root, "moved"))
			})
		}
	}
}

func TestTransferChecksIncomingProtectedPathsBeforeWriting(t *testing.T) {
	for _, tool := range []Tool{NewCpTool(&events.NoOpPublisher{}), NewMvTool(&events.NoOpPublisher{})} {
		t.Run(tool.Declaration().Name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(root, "source/hooks"), 0755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "source/hooks/new.js"), []byte("injected"), 0644))
			ctx := contextWithPolicy(root, nil, []string{".genie/hooks/**"})
			out, err := tool.Handler()(ctx, map[string]any{"source": "source", "destination": ".genie", "_display_message": "testing incoming tree"})
			require.NoError(t, err)
			require.True(t, out.IsError, "%v", out.Details)
			require.NoDirExists(t, filepath.Join(root, ".genie"))
			require.FileExists(t, filepath.Join(root, "source/hooks/new.js"))
		})
	}
}

func TestCopyCannotReadDeniedDescendants(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "source"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "source/secret.txt"), []byte("secret"), 0644))
	ctx := contextWithPolicy(root, []string{"secret.txt"}, nil)
	out, err := NewCpTool(&events.NoOpPublisher{}).Handler()(ctx, map[string]any{"source": "source", "destination": "copy", "_display_message": "testing read policy"})
	require.NoError(t, err)
	require.True(t, out.IsError)
	require.NoDirExists(t, filepath.Join(root, "copy"))
}

func TestDirectoryPolicyChecksCancellation(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "child"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "child/keep.txt"), []byte("keep"), 0644))
	ctx, cancel := context.WithCancel(contextWithPolicy(root, nil, nil))
	cancel()
	out, err := NewRmTool(&events.NoOpPublisher{}).Handler()(ctx, map[string]any{"path": "child", "recursive": "true", "_display_message": "cancelled removal"})
	require.FileExists(t, filepath.Join(root, "child/keep.txt"))
	require.NoError(t, err)
	require.True(t, out.IsError)
}
