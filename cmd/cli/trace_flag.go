package cli

import (
	"fmt"
	"os"

	"github.com/kcaldas/genie/pkg/session"
)

// configureTraceEnv resolves the --trace flag into GENIE_SESSION_RECORDING
// before Genie construction. Unlike the env path (which warns and records
// nothing on garbage), an explicit flag fails fast: a user who typed
// --trace wants a recording, not a silent no-op.
func configureTraceEnv(level string) error {
	if level == "" {
		return nil
	}
	if _, err := session.ParseLevel(level); err != nil {
		return fmt.Errorf("--trace: %w", err)
	}
	return os.Setenv("GENIE_SESSION_RECORDING", level)
}
