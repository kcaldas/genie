// Package theme provides a global theme provider for easy access to themes across components
package theme

import (
	"sync"
)

// Global theme provider instance
var (
	globalProvider *Provider
	providerOnce   sync.Once
)

// Provider provides thread-safe access to the current theme and styles
type Provider struct {
	mu      sync.RWMutex
	manager *Manager
}

// InitGlobalProvider initializes the global theme provider
func InitGlobalProvider(configDir string) error {
	var err error
	providerOnce.Do(func() {
		manager, managerErr := NewManager(configDir)
		if managerErr != nil {
			err = managerErr
			return
		}
		
		globalProvider = &Provider{
			manager: manager,
		}
		
		// Create builtin themes if they don't exist
		_ = manager.CreateBuiltinThemes()
	})
	
	return err
}

// GetGlobalProvider returns the global theme provider
// Panics if InitGlobalProvider hasn't been called
func GetGlobalProvider() *Provider {
	if globalProvider == nil {
		panic("theme provider not initialized - call InitGlobalProvider first")
	}
	return globalProvider
}

// GetStyles returns the current computed styles (thread-safe)
func (p *Provider) GetStyles() Styles {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.manager.GetStyles()
}

// GetTheme returns the current theme (thread-safe)
func (p *Provider) GetTheme() Theme {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.manager.GetTheme()
}

// SetTheme sets the current theme (thread-safe)
func (p *Provider) SetTheme(theme Theme) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.manager.SetTheme(theme)
}

// LoadTheme loads a theme from file (thread-safe)
func (p *Provider) LoadTheme(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.manager.LoadTheme(name)
}

// SaveCurrentTheme saves the current theme (thread-safe)
func (p *Provider) SaveCurrentTheme() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.manager.SaveCurrentTheme()
}

// ListThemes returns available theme names (thread-safe)
func (p *Provider) ListThemes() ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.manager.ListThemes()
}

// GetManager returns the underlying theme manager for advanced operations
func (p *Provider) GetManager() *Manager {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.manager
}

// Convenience functions that use the global provider

// GetStyles returns the current styles from the global provider
func GetStyles() Styles {
	return GetGlobalProvider().GetStyles()
}

// CurrentTheme returns the current theme from the global provider
func CurrentTheme() Theme {
	return GetGlobalProvider().GetTheme()
}

// SetGlobalTheme sets the theme on the global provider
func SetGlobalTheme(theme Theme) {
	GetGlobalProvider().SetTheme(theme)
}

// LoadGlobalTheme loads a theme on the global provider
func LoadGlobalTheme(name string) error {
	return GetGlobalProvider().LoadTheme(name)
}