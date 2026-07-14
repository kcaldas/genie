package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ReplayGen serves previously captured interactions as a Gen implementation,
// so sessions recorded against a real provider (GENIE_CAPTURE_LLM=true) can
// drive tests without network access.
//
// Each incoming call is matched against the interactions loaded from a
// capture file written by CaptureMiddleware:
//
//  1. The earliest unreplayed interaction whose prompt name, args, and attrs
//     all match is served, so identical repeated calls replay in the order
//     they were recorded.
//  2. Otherwise the earliest unreplayed interaction with the same prompt name
//     is served (args often embed timestamps or session ids that differ
//     between the recording and the test run).
//  3. Otherwise the call fails with an error describing the request and the
//     interactions still available.
//
// Each recorded interaction is served at most once. Interactions recorded
// with an error replay that error (as a new error carrying the captured
// message). Streaming calls replay the full recorded response as a single
// chunk. Token counts were not captured, so CountTokens* return zero counts.
type ReplayGen struct {
	mu           sync.Mutex
	interactions []Interaction
	replayed     []bool
	source       string
}

var _ Gen = (*ReplayGen)(nil)

// NewReplayGen loads a capture file (as written by CaptureMiddleware or
// SaveInteractionsToFile) and returns a Gen that replays it.
func NewReplayGen(path string) (Gen, error) {
	interactions, err := LoadInteractionsFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("replay: %w", err)
	}
	gen := NewReplayGenFromInteractions(interactions)
	gen.source = path
	return gen, nil
}

// NewReplayGenFromInteractions builds a ReplayGen from in-memory interactions,
// e.g. ones just recorded by a CaptureMiddleware in the same test.
func NewReplayGenFromInteractions(interactions []Interaction) *ReplayGen {
	copied := make([]Interaction, len(interactions))
	copy(copied, interactions)
	return &ReplayGen{
		interactions: copied,
		replayed:     make([]bool, len(copied)),
		source:       "in-memory",
	}
}

// GenerateContent replays the recorded response for a matching interaction.
func (r *ReplayGen) GenerateContent(ctx context.Context, p Prompt, debug bool, args ...string) (string, error) {
	return r.replay(p, args, nil)
}

// GenerateContentAttr replays the recorded response for a matching interaction.
func (r *ReplayGen) GenerateContentAttr(ctx context.Context, p Prompt, debug bool, attrs []Attr) (string, error) {
	return r.replay(p, nil, attrs)
}

// GenerateContentStream replays a matching interaction as a single-chunk stream.
func (r *ReplayGen) GenerateContentStream(ctx context.Context, p Prompt, debug bool, args ...string) (Stream, error) {
	response, err := r.replay(p, args, nil)
	if err != nil {
		return nil, err
	}
	return newSliceStream(&StreamChunk{Text: response}), nil
}

// GenerateContentAttrStream replays a matching interaction as a single-chunk stream.
func (r *ReplayGen) GenerateContentAttrStream(ctx context.Context, p Prompt, debug bool, attrs []Attr) (Stream, error) {
	response, err := r.replay(p, nil, attrs)
	if err != nil {
		return nil, err
	}
	return newSliceStream(&StreamChunk{Text: response}), nil
}

// CountTokens returns a zero count; token usage is not part of the capture format.
func (r *ReplayGen) CountTokens(ctx context.Context, p Prompt, debug bool, args ...string) (*TokenCount, error) {
	return &TokenCount{}, nil
}

// CountTokensAttr returns a zero count; token usage is not part of the capture format.
func (r *ReplayGen) CountTokensAttr(ctx context.Context, p Prompt, debug bool, attrs []Attr) (*TokenCount, error) {
	return &TokenCount{}, nil
}

// GetStatus reports the replay source.
func (r *ReplayGen) GetStatus() *Status {
	return &Status{
		Connected: true,
		Backend:   "replay",
		Model:     "replay",
		Message:   fmt.Sprintf("replaying captured interactions from %s", r.source),
	}
}

func (r *ReplayGen) replay(p Prompt, args []string, attrs []Attr) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	idx := r.matchLocked(p.Name, args, attrs)
	if idx < 0 {
		return "", r.unmatchedErrorLocked(p, args, attrs)
	}
	r.replayed[idx] = true

	rec := r.interactions[idx]
	if rec.Error != nil {
		return "", errors.New(rec.Error.Message)
	}
	return rec.Response, nil
}

func (r *ReplayGen) matchLocked(name string, args []string, attrs []Attr) int {
	// Prefer an exact match on prompt name + args + attrs.
	for i := range r.interactions {
		if r.replayed[i] {
			continue
		}
		rec := &r.interactions[i]
		if rec.Prompt.Name == name && stringSlicesEqual(rec.Args, args) && attrsMatch(rec.Attrs, attrs) {
			return i
		}
	}
	// Fall back to matching by prompt name only.
	for i := range r.interactions {
		if !r.replayed[i] && r.interactions[i].Prompt.Name == name {
			return i
		}
	}
	return -1
}

func (r *ReplayGen) unmatchedErrorLocked(p Prompt, args []string, attrs []Attr) error {
	var remaining []string
	for i := range r.interactions {
		if !r.replayed[i] {
			remaining = append(remaining, fmt.Sprintf("%q", r.interactions[i].Prompt.Name))
		}
	}
	if len(remaining) == 0 {
		return fmt.Errorf("replay(%s): no recorded interaction left for prompt %q (args=%v attrs=%v); all %d interactions were already replayed",
			r.source, p.Name, args, attrs, len(r.interactions))
	}
	return fmt.Errorf("replay(%s): no recorded interaction matches prompt %q (args=%v attrs=%v); unreplayed prompts: %s",
		r.source, p.Name, args, attrs, strings.Join(remaining, ", "))
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func attrsMatch(recorded []CapturedAttr, attrs []Attr) bool {
	if len(recorded) != len(attrs) {
		return false
	}
	for i := range attrs {
		if recorded[i] != CapturedAttr(attrs[i]) {
			return false
		}
	}
	return true
}
