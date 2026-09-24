package decide

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/tools"
)

// ToolName is the tool's name as the model and hooks call it.
const ToolName = "decide"

// NewTool exposes a Decider as a tool: state and questions in, typed
// answers out. The declaration is the tool's contract; the backend
// behind it is the host's choice and may change without the caller
// noticing.
func NewTool(decider Decider) tools.Tool {
	return &tool{decider: decider}
}

type tool struct {
	decider Decider
}

func (t *tool) Declaration() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        ToolName,
		Description: "Turn messy context into typed decisions. Give the state (the text to judge) and named questions; get one typed answer per question: a choice among your labelled options, a score on your ordered scale, or a yes/no probability (noul). Ask several questions in one call when they share the state.",
		Parameters: &ai.Schema{Type: ai.TypeObject, Properties: map[string]*ai.Schema{
			"state":          {Type: ai.TypeString, Description: "The context to judge: the message, the draft, the record. Only what the questions need."},
			"questions_json": {Type: ai.TypeString, Description: `A JSON object of named questions. Each is {"type": "choice", "instructions": "...", "criteria": {"key": "what it means", ...}} (2 to 255 options), {"type": "score", "instructions": "...", "criteria": ["lowest level", ..., "highest level"]} (2 to 10 levels), or {"type": "noul", "instructions": "a yes/no question"}.`},
		}, Required: []string{"state", "questions_json"}},
		Response: &ai.Schema{Type: ai.TypeObject, Description: "answers keyed by question name: {type, choice | score | noul, confidence?, probabilities?}; backend; usage"},
	}
}

func (t *tool) Handler() ai.HandlerFunc {
	return func(ctx context.Context, args map[string]any) (ai.ToolOutput, error) {
		fail := func(err error) (ai.ToolOutput, error) {
			return ai.ErrorToolOutput(map[string]any{"success": false, "results": err.Error()}), nil
		}
		if t.decider == nil {
			return fail(fmt.Errorf("decide: no backend configured"))
		}
		req, err := ParseRequest(args)
		if err != nil {
			return fail(err)
		}
		resp, err := t.decider.Decide(ctx, req)
		if err != nil {
			return fail(err)
		}
		encoded, err := json.Marshal(resp)
		if err != nil {
			return fail(err)
		}
		var plain map[string]any
		if err := json.Unmarshal(encoded, &plain); err != nil {
			return fail(err)
		}
		plain["success"] = true
		plain["results"] = summary(resp)
		return ai.JSONToolOutput(plain), nil
	}
}

func (t *tool) FormatOutput(result map[string]any) string {
	if text, ok := result["results"].(string); ok {
		return text
	}
	return ""
}

func summary(resp Response) string {
	parts := make([]string, 0, len(resp.Answers))
	for name, a := range resp.Answers {
		switch a.Type {
		case TypeChoice:
			parts = append(parts, name+"="+a.Choice)
		case TypeScore:
			if a.Score != nil {
				parts = append(parts, fmt.Sprintf("%s=%g", name, *a.Score))
			}
		case TypeNoul:
			if a.Noul != nil {
				parts = append(parts, fmt.Sprintf("%s=%.2f", name, *a.Noul))
			}
		}
	}
	return fmt.Sprintf("decided (%s): %s", resp.Backend, joinSorted(parts))
}

func joinSorted(parts []string) string {
	sort.Strings(parts)
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
