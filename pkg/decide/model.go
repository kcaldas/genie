package decide

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	// Provider routes the call when the client multiplexes providers
	// (genai, anthropic, openai, ...); empty means the client's default.
	Provider string
	// ModelName selects the model when set; empty means the client's
	// default.
	ModelName string
}

// promptAttr carries the state and the questions into the prompt as
// data, not template source: clients render Prompt.Text as a Go template,
// and a caller's state may hold anything, "{{customer_name}}" included.
const promptAttr = "decide"

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
		Text:           "{{." + promptAttr + "}}",
		ResponseSchema: schema,
		LLMProvider:    m.Provider,
		ModelName:      m.ModelName,
		DisableCache:   true,
	}
	started := time.Now()
	raw, usage, err := m.generate(ctx, prompt, []ai.Attr{{Key: promptAttr, Value: text.String()}})
	if err != nil {
		return Response{}, fmt.Errorf("decide: model call failed: %w", err)
	}
	usage.LatencyMs = time.Since(started).Milliseconds()
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
	return Response{Backend: "model:" + model, Answers: answers, Usage: usage}, nil
}

// generate streams the call, because the stream is where a client reports
// the generation's own token usage; the text is joined and the usage, when
// the client reports one, is the request's actual count. A client that
// reports none leaves the tokens at zero.
func (m Model) generate(ctx context.Context, prompt ai.Prompt, attrs []ai.Attr) (string, Usage, error) {
	stream, err := m.Gen.GenerateContentAttrStream(ctx, prompt, false, attrs)
	if err != nil {
		return "", Usage{}, err
	}
	defer stream.Close()
	var text strings.Builder
	var usage Usage
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", Usage{}, err
		}
		if chunk == nil {
			continue
		}
		text.WriteString(chunk.Text)
		if chunk.TokenCount != nil {
			usage.InputTokens = int(chunk.TokenCount.InputTokens)
			usage.OutputTokens = int(chunk.TokenCount.OutputTokens)
		}
	}
	return text.String(), usage, nil
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
