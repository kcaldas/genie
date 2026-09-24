package decide

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
)

// Model answers with one structured call to a chat model: the questions
// become a JSON schema (an enum per choice, an integer per score, a
// boolean per noul), the model fills it, and the answer is parsed back.
// It cannot calibrate, so Confidence and Probabilities stay absent.
type Model struct {
	Gen ai.Gen
	// ModelName selects the model when set; empty means the client's
	// default.
	ModelName string
}

const modelInstruction = `You are a decision function. You will be given a STATE and a set of QUESTIONS about it. Answer every question from the state alone: for a choice, pick exactly one of its option keys, the one whose meaning fits best; for a score, pick the index of the level that fits best, 0 for the first; for a noul, answer true or false. Reply with only the JSON object described by the schema, nothing else.`

// Decide implements Decider.
func (m Model) Decide(ctx context.Context, req Request) (Response, error) {
	if m.Gen == nil {
		return Response{}, fmt.Errorf("decide: no model client")
	}
	if err := Validate(req); err != nil {
		return Response{}, err
	}
	names := make([]string, 0, len(req.Questions))
	for name := range req.Questions {
		names = append(names, name)
	}
	sort.Strings(names)

	schema := &ai.Schema{Type: ai.TypeObject, Properties: map[string]*ai.Schema{}, Required: names}
	var text strings.Builder
	text.WriteString("STATE:\n")
	text.WriteString(req.State)
	text.WriteString("\n\nQUESTIONS:\n")
	for _, name := range names {
		q := req.Questions[name]
		fmt.Fprintf(&text, "\n%s (%s): %s\n", name, q.Type, q.Instructions)
		switch q.Type {
		case TypeChoice:
			keys := q.ChoiceKeys()
			for _, key := range keys {
				fmt.Fprintf(&text, "  - %s: %s\n", key, q.Criteria[key])
			}
			schema.Properties[name] = &ai.Schema{Type: ai.TypeString, Enum: keys, Description: q.Instructions}
		case TypeScore:
			for i, level := range q.Levels {
				fmt.Fprintf(&text, "  %d: %s\n", i, level)
			}
			schema.Properties[name] = &ai.Schema{Type: ai.TypeInteger, Minimum: 0, Maximum: float64(len(q.Levels) - 1), Description: q.Instructions}
		case TypeNoul:
			schema.Properties[name] = &ai.Schema{Type: ai.TypeBoolean, Description: q.Instructions}
		}
	}

	prompt := ai.Prompt{
		Name:           "decide",
		Instruction:    modelInstruction,
		Text:           text.String(),
		ResponseSchema: schema,
		ModelName:      m.ModelName,
		DisableCache:   true,
	}
	started := time.Now()
	raw, err := m.Gen.GenerateContent(ctx, prompt, false)
	if err != nil {
		return Response{}, fmt.Errorf("decide: model call failed: %w", err)
	}
	var filled map[string]any
	if err := json.Unmarshal([]byte(stripFences(raw)), &filled); err != nil {
		return Response{}, fmt.Errorf("decide: model answered with something that is not the JSON object asked for: %w", err)
	}
	answers := make(map[string]Answer, len(req.Questions))
	for _, name := range names {
		q := req.Questions[name]
		value, has := filled[name]
		if !has {
			return Response{}, fmt.Errorf("decide: model did not answer %q", name)
		}
		answer, err := modelAnswer(q, value)
		if err != nil {
			return Response{}, fmt.Errorf("decide: %s: %w", name, err)
		}
		answers[name] = answer
	}
	model := m.ModelName
	if model == "" {
		if status := m.Gen.GetStatus(); status != nil {
			model = status.Model
		}
	}
	return Response{Backend: "model:" + model, Answers: answers, Usage: Usage{LatencyMs: time.Since(started).Milliseconds()}}, nil
}

func modelAnswer(q Question, value any) (Answer, error) {
	switch q.Type {
	case TypeChoice:
		key, _ := value.(string)
		if _, ok := q.Criteria[key]; !ok {
			return Answer{}, fmt.Errorf("model chose %q, not one of the options", key)
		}
		return Answer{Type: TypeChoice, Choice: key}, nil
	case TypeScore:
		n, ok := value.(float64)
		if !ok {
			return Answer{}, fmt.Errorf("model scored %v, not a number", value)
		}
		n = math.Round(n)
		if n < 0 || int(n) >= len(q.Levels) {
			return Answer{}, fmt.Errorf("model scored %v, outside 0 to %d", n, len(q.Levels)-1)
		}
		return Answer{Type: TypeScore, Score: float(n)}, nil
	case TypeNoul:
		yes, ok := value.(bool)
		if !ok {
			return Answer{}, fmt.Errorf("model answered %v, not true or false", value)
		}
		p := 0.0
		if yes {
			p = 1
		}
		return Answer{Type: TypeNoul, Noul: float(p)}, nil
	}
	return Answer{}, fmt.Errorf("unknown type %q", q.Type)
}

// stripFences removes a ```json fence a model may wrap its object in.
func stripFences(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return strings.TrimSpace(s)
}
