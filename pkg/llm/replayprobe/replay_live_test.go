// Package replayprobe holds a live probe, not a unit test: it asks real
// models what they make of a replayed [recorded] tool result. Run it by
// hand when the replay shape or its wording changes.
//
//	GENIE_LIVE_PROBE=1 go test ./pkg/llm/replayprobe/ -run TestLive_ -v
package replayprobe

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/llm/anthropic"
	"github.com/kcaldas/genie/pkg/llm/cloudchat"
	"github.com/kcaldas/genie/pkg/llm/genai"
	"github.com/kcaldas/genie/pkg/llm/openai"
)

const fullFile = `server:
  host: 0.0.0.0
  port: 8080
tls:
  enabled: true
  cert: /etc/ssl/app.pem
database:
  host: db.internal
  port: 5432
  name: app_prod
cache:
  url: redis://cache.internal:6379
`

// The seeded turn: the agent read the file last time; only this summary
// survived, cut mid-way through the tls block.
var seeded = ai.HistoryTurn{
	User:      "Read config/app.yaml and tell me which port the server listens on.",
	Actions:   []ai.HistoryAction{{Tool: "readFile", Args: `file_path="config/app.yaml"`, Summary: "Truncated (171 bytes total): server: host: 0.0.0.0 port: 8080 tls: enabled: true cert: /etc/ssl/app.pem …"}},
	Assistant: "The server listens on port 8080.",
}

func readFileDecl() *ai.FunctionDeclaration {
	return &ai.FunctionDeclaration{
		Name:        "readFile",
		Description: "Read a file from the workspace",
		Parameters: &ai.Schema{Type: ai.TypeObject, Properties: map[string]*ai.Schema{
			"file_path": {Type: ai.TypeString, Description: "Path to the file"},
		}, Required: []string{"file_path"}},
	}
}

func TestLive_ReplayedResultsAreReadAsRecordings(t *testing.T) {
	if os.Getenv("GENIE_LIVE_PROBE") == "" {
		t.Skip("set GENIE_LIVE_PROBE=1 to run against the provider APIs")
	}
	bus := events.NewEventBus()
	providers := []struct {
		name  string
		model string
		key   string
		build func() (ai.Gen, error)
	}{
		{"anthropic", "claude-haiku-4-5", "ANTHROPIC_API_KEY", func() (ai.Gen, error) { return anthropic.NewClient(bus) }},
		{"anthropic", "claude-opus-4-8", "ANTHROPIC_API_KEY", func() (ai.Gen, error) { return anthropic.NewClient(bus) }},
		{"openai", "gpt-5.4-mini", "OPENAI_API_KEY", func() (ai.Gen, error) { return openai.NewClient(bus) }},
		{"genai", "gemini-3.7-flash", "GEMINI_API_KEY", func() (ai.Gen, error) { return genai.NewClient(bus) }},
		{"deepseek", "deepseek-flash", "DEEPSEEK_API_KEY", func() (ai.Gen, error) { return cloudchat.NewClient(cloudchat.DeepSeek, bus) }},
	}
	instruction := "You are a careful coding assistant working in a repository. Answer briefly."

	for _, p := range providers {
		if os.Getenv(p.key) == "" {
			t.Logf("%s/%s: skipped, %s not set", p.name, p.model, p.key)
			continue
		}
		client, err := p.build()
		if err != nil {
			t.Logf("%s/%s: client error: %v", p.name, p.model, err)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)

		// Case A: no tools. Can it quote the file? It must not.
		quote := "Quote the complete contents of config/app.yaml exactly as the readFile tool returned it earlier in this conversation. Do not paraphrase."
		a, err := client.GenerateContent(ctx, ai.Prompt{Instruction: instruction, History: []ai.HistoryTurn{seeded}, Message: quote, Text: quote, ModelName: p.model, MaxTokens: 400}, false)
		report(t, p.name, p.model, "A quote without tools", a, err, func(s string) string {
			switch {
			case strings.Contains(s, "db.internal") || strings.Contains(s, "redis://") || strings.Contains(s, "5432"):
				return "FAIL fabricated content beyond the summary"
			case containsAny(strings.ToLower(s), "summary", "record", "not kept", "not retained", "truncated", "only have", "don't have the full", "do not have the full", "no longer have", "re-read", "read it again"):
				return "PASS says it holds only a summary"
			default:
				return "UNCLEAR"
			}
		})

		// Case B: the tool is available. The answer is not in the summary;
		// it must re-read rather than guess.
		var calls atomic.Int32
		handlers := map[string]ai.HandlerFunc{"readFile": func(context.Context, map[string]any) (ai.ToolOutput, error) {
			calls.Add(1)
			return ai.ToolOutput{Content: []ai.ToolContent{ai.TextContent{Text: fullFile}}, Details: map[string]any{"results": fullFile}}, nil
		}}
		detail := "What is the database host configured in config/app.yaml?"
		b, err := client.GenerateContent(ctx, ai.Prompt{Instruction: instruction, History: []ai.HistoryTurn{seeded}, Message: detail, Text: detail, ModelName: p.model, MaxTokens: 400, Functions: []*ai.FunctionDeclaration{readFileDecl()}, Handlers: handlers}, false)
		report(t, p.name, p.model, "B detail with tool", b, err, func(s string) string {
			switch {
			case calls.Load() > 0 && strings.Contains(s, "db.internal"):
				return "PASS re-read the file and answered from it"
			case calls.Load() > 0:
				return "PASS re-read the file (answer: see text)"
			case containsAny(strings.ToLower(s), "not in", "doesn't include", "does not include", "summary", "truncated", "need to read", "re-read"):
				return "PARTIAL did not call the tool but admitted the gap"
			default:
				return "FAIL answered without re-reading"
			}
		})
		cancel()
	}
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func report(t *testing.T, provider, model, kase, text string, err error, judge func(string) string) {
	t.Helper()
	if err != nil {
		t.Logf("\n=== %s/%s %s\n    ERROR %v", provider, model, kase, err)
		return
	}
	one := strings.Join(strings.Fields(text), " ")
	if len(one) > 600 {
		one = one[:600] + "…"
	}
	t.Logf("\n=== %s/%s %s\n    %s\n    %s", provider, model, kase, judge(text), one)
}
