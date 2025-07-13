package helpers

import (
	"context"
	"sync"
)

// RequestContextManager manages shared context for multiple concurrent requests
type RequestContextManager struct {
	// Master context shared by all requests
	masterCtx    context.Context
	masterCancel context.CancelFunc
	
	// Request tracking
	activeRequests int
	mutex          sync.RWMutex
}

// NewRequestContextManager creates a new request context manager
func NewRequestContextManager() *RequestContextManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &RequestContextManager{
		masterCtx:    ctx,
		masterCancel: cancel,
	}
}

// StartRequest increments the active request count and returns the shared context
func (rcm *RequestContextManager) StartRequest() context.Context {
	rcm.mutex.Lock()
	defer rcm.mutex.Unlock()
	
	rcm.activeRequests++
	return rcm.masterCtx
}

// FinishRequest decrements the active request count
// Returns true if this was the last active request
func (rcm *RequestContextManager) FinishRequest() bool {
	rcm.mutex.Lock()
	defer rcm.mutex.Unlock()
	
	rcm.activeRequests--
	if rcm.activeRequests < 0 {
		rcm.activeRequests = 0
	}
	
	return rcm.activeRequests == 0
}

// CancelAll cancels all active requests and creates a new master context for future requests
// Returns the number of requests that were cancelled
func (rcm *RequestContextManager) CancelAll() int {
	rcm.mutex.Lock()
	defer rcm.mutex.Unlock()
	
	cancelledCount := rcm.activeRequests
	
	if rcm.activeRequests > 0 {
		// Cancel the current master context
		rcm.masterCancel()
		
		// Create new master context for future requests
		rcm.masterCtx, rcm.masterCancel = context.WithCancel(context.Background())
		
		// Reset active requests count
		rcm.activeRequests = 0
	}
	
	return cancelledCount
}

// GetActiveRequestCount returns the current number of active requests
func (rcm *RequestContextManager) GetActiveRequestCount() int {
	rcm.mutex.RLock()
	defer rcm.mutex.RUnlock()
	
	return rcm.activeRequests
}

// HasActiveRequests returns true if there are any active requests
func (rcm *RequestContextManager) HasActiveRequests() bool {
	return rcm.GetActiveRequestCount() > 0
}