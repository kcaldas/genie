package helpers

import (
	"context"
	"testing"
	"time"
)

func TestRequestContextManager_SingleRequest(t *testing.T) {
	rcm := NewRequestContextManager()
	
	// Initially no active requests
	if rcm.HasActiveRequests() {
		t.Error("Should have no active requests initially")
	}
	
	// Start a request
	ctx := rcm.StartRequest()
	if ctx == nil {
		t.Error("StartRequest should return a valid context")
	}
	
	if !rcm.HasActiveRequests() {
		t.Error("Should have active requests after starting one")
	}
	
	if rcm.GetActiveRequestCount() != 1 {
		t.Errorf("Expected 1 active request, got %d", rcm.GetActiveRequestCount())
	}
	
	// Finish the request
	isLast := rcm.FinishRequest()
	if !isLast {
		t.Error("Should return true when finishing the last request")
	}
	
	if rcm.HasActiveRequests() {
		t.Error("Should have no active requests after finishing")
	}
}

func TestRequestContextManager_MultipleRequests(t *testing.T) {
	rcm := NewRequestContextManager()
	
	// Start multiple requests
	ctx1 := rcm.StartRequest()
	ctx2 := rcm.StartRequest()
	ctx3 := rcm.StartRequest()
	
	if rcm.GetActiveRequestCount() != 3 {
		t.Errorf("Expected 3 active requests, got %d", rcm.GetActiveRequestCount())
	}
	
	// All contexts should be derived from the same master context
	if ctx1 != ctx2 || ctx2 != ctx3 {
		t.Error("All requests should share the same context")
	}
	
	// Finish requests one by one
	isLast := rcm.FinishRequest()
	if isLast {
		t.Error("Should not be last request when 2 remain")
	}
	
	isLast = rcm.FinishRequest()
	if isLast {
		t.Error("Should not be last request when 1 remains")
	}
	
	isLast = rcm.FinishRequest()
	if !isLast {
		t.Error("Should be last request when finishing the final one")
	}
	
	if rcm.HasActiveRequests() {
		t.Error("Should have no active requests after finishing all")
	}
}

func TestRequestContextManager_CancelAll(t *testing.T) {
	rcm := NewRequestContextManager()
	
	// Start multiple requests
	ctx1 := rcm.StartRequest()
	ctx2 := rcm.StartRequest()
	
	if rcm.GetActiveRequestCount() != 2 {
		t.Errorf("Expected 2 active requests, got %d", rcm.GetActiveRequestCount())
	}
	
	// Cancel all requests
	cancelled := rcm.CancelAll()
	if cancelled != 2 {
		t.Errorf("Expected 2 cancelled requests, got %d", cancelled)
	}
	
	if rcm.HasActiveRequests() {
		t.Error("Should have no active requests after cancelling all")
	}
	
	// Original contexts should be cancelled
	select {
	case <-ctx1.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled")
	}
	
	select {
	case <-ctx2.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled")
	}
	
	// New requests should get a fresh context
	ctx3 := rcm.StartRequest()
	select {
	case <-ctx3.Done():
		t.Error("New context should not be cancelled")
	default:
		// Expected
	}
	
	rcm.FinishRequest()
}

func TestRequestContextManager_CancelAllWithNoActiveRequests(t *testing.T) {
	rcm := NewRequestContextManager()
	
	// Cancel when no active requests
	cancelled := rcm.CancelAll()
	if cancelled != 0 {
		t.Errorf("Expected 0 cancelled requests, got %d", cancelled)
	}
	
	if rcm.HasActiveRequests() {
		t.Error("Should still have no active requests")
	}
}

func TestRequestContextManager_ContextCancellation(t *testing.T) {
	rcm := NewRequestContextManager()
	
	// Start a request
	ctx := rcm.StartRequest()
	
	// Verify context is not cancelled initially
	select {
	case <-ctx.Done():
		t.Error("Context should not be cancelled initially")
	default:
		// Expected
	}
	
	// Cancel all requests
	rcm.CancelAll()
	
	// Context should now be cancelled
	select {
	case <-ctx.Done():
		// Expected
		if ctx.Err() != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", ctx.Err())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled after CancelAll")
	}
}