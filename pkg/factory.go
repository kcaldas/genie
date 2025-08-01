package pkg

import (
	internalDI "github.com/kcaldas/genie/internal/di"
	"github.com/kcaldas/genie/pkg/genie"
)

// Shared genie instance (singleton)
var (
	genieInstance    genie.Genie
	genieError       error
	genieInitialized bool
)

// ProvideGenie provides a shared Genie singleton instance
func ProvideGenie() (genie.Genie, error) {
	if !genieInitialized {
		genieInstance, genieError = internalDI.ProvideGenie()
		genieInitialized = true
	}
	return genieInstance, genieError
}
