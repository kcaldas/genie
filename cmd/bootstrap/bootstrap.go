// Package bootstrap owns the process-wide Genie instance shared by the
// CLI and TUI clients. Both are thin clients of the same core; neither
// owns the other's bootstrap.
package bootstrap

import (
	"sync"

	"github.com/kcaldas/genie/pkg/genie"
)

var (
	once     sync.Once
	instance genie.Genie
	initErr  error
)

// Genie returns the process-wide Genie instance, constructing it on
// first use. Safe for concurrent callers.
func Genie() (genie.Genie, error) {
	once.Do(func() {
		instance, initErr = genie.ProvideGenie()
	})
	return instance, initErr
}
