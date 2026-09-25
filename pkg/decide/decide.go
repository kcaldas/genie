// Package decide turns messy context into typed decisions: a choice among
// labelled options, a score on an ordered scale, or a yes/no probability.
// The contract mirrors the Jev decision API (state + named questions in,
// answers keyed by the same names out), so a Jev backend and a model
// backend are interchangeable behind one tool; the caller never learns
// which answered.
package decide

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Type is a question's kind.
type Type string

const (
	// TypeChoice picks one of up to 255 labelled options.
	TypeChoice Type = "choice"
	// TypeScore places the state on an ordered scale of 2 to 10 levels.
	TypeScore Type = "score"
	// TypeNoul is a yes/no as a probability from 0 to 1.
	TypeNoul Type = "noul"
)

// MaxChoices bounds a choice question's options.
const MaxChoices = 255

// Question is one typed question about the state.
type Question struct {
	Type         Type   `json:"type"`
	Instructions string `json:"instructions"`
	// Criteria is the options of a choice, key to meaning; for a noul,
	// optionally what "true" and "false" mean. Empty for a score.
	Criteria map[string]string `json:"-"`
	// Levels is the ordered scale of a score, low to high. Empty for the
	// other types.
	Levels []string `json:"-"`
}

// questionWire is the JSON form: criteria is a map for a choice and an
// array for a score, as Jev has it.
type questionWire struct {
	Type         Type            `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
}

// MarshalJSON writes the Jev shape.
func (q Question) MarshalJSON() ([]byte, error) {
	w := questionWire{Type: q.Type, Instructions: q.Instructions}
	switch q.Type {
	case TypeChoice, TypeNoul:
		if q.Type == TypeNoul && len(q.Criteria) == 0 {
			break
		}
		encoded, err := json.Marshal(q.Criteria)
		if err != nil {
			return nil, err
		}
		w.Criteria = encoded
	case TypeScore:
		encoded, err := json.Marshal(q.Levels)
		if err != nil {
			return nil, err
		}
		w.Criteria = encoded
	}
	return json.Marshal(w)
}

// UnmarshalJSON reads the Jev shape.
func (q *Question) UnmarshalJSON(data []byte) error {
	var w questionWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	q.Type, q.Instructions, q.Criteria, q.Levels = w.Type, w.Instructions, nil, nil
	if len(w.Criteria) == 0 || string(w.Criteria) == "null" {
		return nil
	}
	switch w.Type {
	case TypeChoice, TypeNoul:
		return json.Unmarshal(w.Criteria, &q.Criteria)
	case TypeScore:
		return json.Unmarshal(w.Criteria, &q.Levels)
	}
	return nil
}

// ChoiceKeys returns a choice's option keys in a stable order.
func (q Question) ChoiceKeys() []string {
	keys := make([]string, 0, len(q.Criteria))
	for key := range q.Criteria {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Request is one decision call: the state to judge and the questions to
// answer about it, evaluated together.
type Request struct {
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// Answer is what a question got. Type says which fields apply. Confidence
// and Probabilities are present when the backend can calibrate (Jev) and
// absent when it cannot (a model): a caller that thresholds on them treats
// absence as certainty.
type Answer struct {
	Type Type `json:"type"`
	// Choice is the winning option key of a choice.
	Choice string `json:"choice,omitempty"`
	// Score is a score's position on its scale, 0-based, possibly
	// fractional.
	Score *float64 `json:"score,omitempty"`
	// Noul is a yes/no as a probability from 0 to 1.
	Noul          *float64           `json:"noul,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

// Usage is what the call cost, as far as the backend says. CostUSD is
// present only when the backend prices the call itself (the proxy does;
// the official API reports tokens for the caller to price).
type Usage struct {
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
	LatencyMs    int64   `json:"latency_ms,omitempty"`
}

// Response is the answers, keyed like the questions, and who answered.
type Response struct {
	// Backend names what answered: "model:<model>" or "jev:<model>".
	Backend string            `json:"backend"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

// Decider answers a request. Implementations: Model (one structured call
// to a chat model) and Jev (the Jev decision API).
type Decider interface {
	Decide(ctx context.Context, req Request) (Response, error)
}

// Validate checks a request against the contract, so every backend gets
// the same guarantees: a state, at least one question, a type per
// question, instructions on each, 2 to MaxChoices options on a choice, 2
// to 10 levels on a score, nothing on a noul.
func Validate(req Request) error {
	if strings.TrimSpace(req.State) == "" {
		return fmt.Errorf("state is required")
	}
	if len(req.Questions) == 0 {
		return fmt.Errorf("at least one question is required")
	}
	for name, q := range req.Questions {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("a question needs a name")
		}
		if strings.TrimSpace(q.Instructions) == "" {
			return fmt.Errorf("question %q needs instructions", name)
		}
		switch q.Type {
		case TypeChoice:
			if len(q.Criteria) < 2 || len(q.Criteria) > MaxChoices {
				return fmt.Errorf("question %q: a choice needs 2 to %d criteria, got %d", name, MaxChoices, len(q.Criteria))
			}
			for key, meaning := range q.Criteria {
				if strings.TrimSpace(key) == "" || strings.TrimSpace(meaning) == "" {
					return fmt.Errorf("question %q: every criterion needs a key and a meaning", name)
				}
			}
		case TypeScore:
			if len(q.Levels) < 2 || len(q.Levels) > 10 {
				return fmt.Errorf("question %q: a score needs 2 to 10 levels, got %d", name, len(q.Levels))
			}
			for _, level := range q.Levels {
				if strings.TrimSpace(level) == "" {
					return fmt.Errorf("question %q: every level needs a description", name)
				}
			}
		case TypeNoul:
			if len(q.Levels) > 0 {
				return fmt.Errorf("question %q: a noul takes no levels", name)
			}
			for key := range q.Criteria {
				if key != "true" && key != "false" {
					return fmt.Errorf("question %q: a noul's criteria are \"true\" and \"false\" only, got %q", name, key)
				}
			}
		default:
			return fmt.Errorf("question %q: type must be choice, score, or noul, got %q", name, q.Type)
		}
	}
	return nil
}

// ParseRequest reads a request from the tool's arguments: state as text,
// questions as an object in the Jev shape. A questions_json string is
// accepted in place of the object, for callers whose argument schema
// cannot express a free-form object.
func ParseRequest(args map[string]any) (Request, error) {
	req := Request{}
	switch state := args["state"].(type) {
	case string:
		req.State = state
	case nil:
	default:
		encoded, err := json.Marshal(state)
		if err != nil {
			return req, fmt.Errorf("state: %w", err)
		}
		req.State = string(encoded)
	}
	var raw []byte
	switch questions := args["questions"].(type) {
	case string:
		raw = []byte(questions)
	case nil:
		if text, ok := args["questions_json"].(string); ok {
			raw = []byte(text)
		}
	default:
		encoded, err := json.Marshal(questions)
		if err != nil {
			return req, fmt.Errorf("questions: %w", err)
		}
		raw = encoded
	}
	if len(raw) == 0 {
		return req, fmt.Errorf("questions is required")
	}
	if err := json.Unmarshal(raw, &req.Questions); err != nil {
		return req, fmt.Errorf("questions: %w", err)
	}
	return req, Validate(req)
}

func float(v float64) *float64 { return &v }
