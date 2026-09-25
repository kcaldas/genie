package decide

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/require"
)

const questionsJSON = `{
  "route": {"type": "choice", "instructions": "Where should this go?", "criteria": {"billing": "payments or refunds", "bug": "the product is broken", "account": "login or access"}},
  "urgency": {"type": "score", "instructions": "How urgent?", "criteria": ["routine", "today", "urgent", "critical"]},
  "escalate": {"type": "noul", "instructions": "Escalate to a human now?"}
}`

func TestQuestionJSONRoundTripsTheJevShape(t *testing.T) {
	var questions map[string]Question
	require.NoError(t, json.Unmarshal([]byte(questionsJSON), &questions))
	require.Equal(t, TypeChoice, questions["route"].Type)
	require.Equal(t, "the product is broken", questions["route"].Criteria["bug"])
	require.Equal(t, []string{"routine", "today", "urgent", "critical"}, questions["urgency"].Levels)
	require.Equal(t, TypeNoul, questions["escalate"].Type)

	encoded, err := json.Marshal(questions)
	require.NoError(t, err)
	var back map[string]Question
	require.NoError(t, json.Unmarshal(encoded, &back))
	require.Equal(t, questions, back)
	require.Contains(t, string(encoded), `"criteria":["routine","today","urgent","critical"]`)
	require.NotContains(t, string(encoded), `"levels"`)
}

func TestValidateRejectsWhatTheContractForbids(t *testing.T) {
	good := func() Request {
		var questions map[string]Question
		require.NoError(t, json.Unmarshal([]byte(questionsJSON), &questions))
		return Request{State: "charged twice, no reply for 3 days", Questions: questions}
	}
	require.NoError(t, Validate(good()))

	cases := map[string]func(r *Request){
		"no state":        func(r *Request) { r.State = " " },
		"no questions":    func(r *Request) { r.Questions = nil },
		"no instructions": func(r *Request) { q := r.Questions["route"]; q.Instructions = ""; r.Questions["route"] = q },
		"one option": func(r *Request) {
			q := r.Questions["route"]
			q.Criteria = map[string]string{"only": "x"}
			r.Questions["route"] = q
		},
		"empty meaning": func(r *Request) { q := r.Questions["route"]; q.Criteria["bug"] = ""; r.Questions["route"] = q },
		"eleven levels": func(r *Request) {
			q := r.Questions["urgency"]
			q.Levels = strings.Split("a b c d e f g h i j k", " ")
			r.Questions["urgency"] = q
		},
		"noul with criteria": func(r *Request) {
			q := r.Questions["escalate"]
			q.Criteria = map[string]string{"a": "b"}
			r.Questions["escalate"] = q
		},
		"unknown type": func(r *Request) { q := r.Questions["escalate"]; q.Type = "rank"; r.Questions["escalate"] = q },
		"too many options": func(r *Request) {
			q := r.Questions["route"]
			q.Criteria = map[string]string{}
			for i := 0; i <= MaxChoices; i++ {
				q.Criteria[strings.Repeat("k", i+1)] = "m"
			}
			r.Questions["route"] = q
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := good()
			mutate(&r)
			require.Error(t, Validate(r))
		})
	}
}

func TestParseRequestAcceptsObjectOrJSONString(t *testing.T) {
	var asObject map[string]any
	require.NoError(t, json.Unmarshal([]byte(questionsJSON), &asObject))
	req, err := ParseRequest(map[string]any{"state": "hello", "questions": asObject})
	require.NoError(t, err)
	require.Len(t, req.Questions, 3)

	req, err = ParseRequest(map[string]any{"state": "hello", "questions_json": questionsJSON})
	require.NoError(t, err)
	require.Len(t, req.Questions, 3)

	_, err = ParseRequest(map[string]any{"state": "hello"})
	require.ErrorContains(t, err, "questions is required")
	_, err = ParseRequest(map[string]any{"questions_json": questionsJSON})
	require.ErrorContains(t, err, "state is required")
}

// fakeGen streams a fixed reply, in two chunks with the usage on the
// last, the way a real client does, and records the prompt it got with
// its attrs rendered the way a client renders them.
type fakeGen struct {
	reply  string
	usage  *ai.TokenCount
	prompt ai.Prompt
	err    error
}

func (f *fakeGen) GenerateContent(context.Context, ai.Prompt, bool, ...string) (string, error) {
	return "", errors.New("decide streams")
}
func (f *fakeGen) GenerateContentAttr(context.Context, ai.Prompt, bool, []ai.Attr) (string, error) {
	return "", errors.New("decide streams")
}
func (f *fakeGen) GenerateContentStream(context.Context, ai.Prompt, bool, ...string) (ai.Stream, error) {
	return nil, errors.New("decide passes attrs")
}
func (f *fakeGen) GenerateContentAttrStream(_ context.Context, p ai.Prompt, _ bool, attrs []ai.Attr) (ai.Stream, error) {
	rendered, err := ai.RenderPrompt(p, attrMap(attrs))
	if err != nil {
		return nil, err
	}
	f.prompt = rendered
	if f.err != nil {
		return nil, f.err
	}
	half := len(f.reply) / 2
	return &sliceStream{chunks: []*ai.StreamChunk{{Text: f.reply[:half]}, {Text: f.reply[half:], TokenCount: f.usage}}}, nil
}
func (f *fakeGen) CountTokens(context.Context, ai.Prompt, bool, ...string) (*ai.TokenCount, error) {
	return nil, errors.New("decide never counts")
}
func (f *fakeGen) CountTokensAttr(context.Context, ai.Prompt, bool, []ai.Attr) (*ai.TokenCount, error) {
	return nil, nil
}
func (f *fakeGen) GetStatus() *ai.Status { return &ai.Status{Model: "fake-model"} }

func attrMap(attrs []ai.Attr) map[string]string {
	m := map[string]string{}
	for _, a := range attrs {
		m[a.Key] = a.Value
	}
	return m
}

type sliceStream struct {
	chunks []*ai.StreamChunk
	idx    int
}

func (s *sliceStream) Recv() (*ai.StreamChunk, error) {
	if s.idx >= len(s.chunks) {
		return nil, io.EOF
	}
	s.idx++
	return s.chunks[s.idx-1], nil
}
func (s *sliceStream) Close() error { return nil }

func TestModelAsksForASchemaAndParsesTheAnswers(t *testing.T) {
	req, err := ParseRequest(map[string]any{"state": "charged twice, no reply", "questions_json": questionsJSON})
	require.NoError(t, err)
	gen := &fakeGen{reply: "```json\n{\"route\":\"billing\",\"urgency\":3,\"escalate\":true}\n```"}
	resp, err := Model{Gen: gen}.Decide(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, "model:fake-model", resp.Backend)
	require.Equal(t, "billing", resp.Answers["route"].Choice)
	require.Nil(t, resp.Answers["route"].Confidence, "a model cannot calibrate")
	require.Equal(t, 3.0, *resp.Answers["urgency"].Score)
	require.Equal(t, 1.0, *resp.Answers["escalate"].Noul)

	schema := gen.prompt.ResponseSchema
	require.NotNil(t, schema)
	require.ElementsMatch(t, []string{"account", "billing", "bug"}, schema.Properties["route"].Enum)
	require.Equal(t, ai.TypeInteger, schema.Properties["urgency"].Type)
	require.Equal(t, 3.0, schema.Properties["urgency"].Maximum)
	require.Equal(t, ai.TypeBoolean, schema.Properties["escalate"].Type)
	require.Contains(t, gen.prompt.Text, "bug: the product is broken")
	require.True(t, gen.prompt.DisableCache)
}

func TestModelRoutesToTheConfiguredProviderAndModel(t *testing.T) {
	req, err := ParseRequest(map[string]any{"state": "x", "questions_json": questionsJSON})
	require.NoError(t, err)
	gen := &fakeGen{reply: `{"route":"bug","urgency":1,"escalate":false}`}
	resp, err := Model{Gen: gen, Provider: "genai", ModelName: "gemini-3.5-flash-lite"}.Decide(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "genai", gen.prompt.LLMProvider)
	require.Equal(t, "gemini-3.5-flash-lite", gen.prompt.ModelName)
	require.Equal(t, "model:gemini-3.5-flash-lite", resp.Backend)
}

func TestModelRejectsAnAnswerOutsideTheContract(t *testing.T) {
	req, err := ParseRequest(map[string]any{"state": "x", "questions_json": questionsJSON})
	require.NoError(t, err)
	for name, reply := range map[string]string{
		"unknown option": `{"route":"legal","urgency":1,"escalate":false}`,
		"score too high": `{"route":"bug","urgency":9,"escalate":false}`,
		"missing":        `{"route":"bug","urgency":1}`,
		"not json":       `sure, billing`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Model{Gen: &fakeGen{reply: reply}}.Decide(context.Background(), req)
			require.Error(t, err)
		})
	}
}

func TestJevSendsTheRequestAsIsAndMapsTheAnswers(t *testing.T) {
	var got map[string]any
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{
			"route":{"type":"choice","choice":"billing","confidence":0.99,"probabilities":{"billing":0.87,"bug":0.08,"account":0.05}},
			"urgency":{"type":"score","score":3.0,"probabilities":{"0":0,"3":1}},
			"escalate":{"type":"noul","noul":0.94}},
			"usage":{"input_tokens":62,"cost_usd":0.000026}}`))
	}))
	defer server.Close()

	req, err := ParseRequest(map[string]any{"state": "charged twice", "questions_json": questionsJSON})
	require.NoError(t, err)
	resp, err := Jev{URL: server.URL, APIKey: "jv_live_test", ModelName: "jev-1.13.0"}.Decide(context.Background(), req)
	require.NoError(t, err)

	require.Equal(t, "Bearer jv_live_test", auth)
	require.Equal(t, "charged twice", got["state"])
	require.Equal(t, "jev-1.13.0", got["model"])
	require.Nil(t, got["questions"].(map[string]any)["escalate"].(map[string]any)["criteria"], "a noul without criteria sends none")
	questions := got["questions"].(map[string]any)
	require.Equal(t, []any{"routine", "today", "urgent", "critical"}, questions["urgency"].(map[string]any)["criteria"])
	require.Equal(t, "payments or refunds", questions["route"].(map[string]any)["criteria"].(map[string]any)["billing"])

	require.Equal(t, "jev:jev-1.13.0", resp.Backend)
	require.Equal(t, "billing", resp.Answers["route"].Choice)
	require.Equal(t, 0.99, *resp.Answers["route"].Confidence)
	require.Equal(t, 0.87, resp.Answers["route"].Probabilities["billing"])
	require.Equal(t, 3.0, *resp.Answers["urgency"].Score)
	require.Equal(t, 0.94, *resp.Answers["escalate"].Noul)
	require.Equal(t, 62, resp.Usage.InputTokens)
	require.Equal(t, 0.000026, resp.Usage.CostUSD)
}

func TestJevErrorsAreNamed(t *testing.T) {
	req, err := ParseRequest(map[string]any{"state": "x", "questions_json": questionsJSON})
	require.NoError(t, err)
	_, err = Jev{APIKey: ""}.Decide(context.Background(), req)
	require.ErrorContains(t, err, "no API key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(402) }))
	defer server.Close()
	_, err = Jev{URL: server.URL, APIKey: "k"}.Decide(context.Background(), req)
	require.ErrorContains(t, err, "balance is empty")

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"model":"jev","answers":{"route":{"type":"choice","choice":"legal"}}}`))
	}))
	defer server2.Close()
	_, err = Jev{URL: server2.URL, APIKey: "k"}.Decide(context.Background(), req)
	require.ErrorContains(t, err, "not one of the options")
}

func TestToolValidatesThenAnswers(t *testing.T) {
	tool := NewTool(Model{Gen: &fakeGen{reply: `{"route":"bug","urgency":0,"escalate":false}`}})
	require.Equal(t, ToolName, tool.Declaration().Name)

	out, err := tool.Handler()(context.Background(), map[string]any{"state": "the app crashes", "questions_json": questionsJSON})
	require.NoError(t, err)
	require.False(t, out.IsError)
	answers := out.Details["answers"].(map[string]any)
	require.Equal(t, "bug", answers["route"].(map[string]any)["choice"])
	require.Equal(t, 0.0, answers["escalate"].(map[string]any)["noul"])
	require.Contains(t, out.Details["results"], "route=bug")
	require.Equal(t, "model:fake-model", out.Details["backend"])

	out, err = tool.Handler()(context.Background(), map[string]any{"state": "x", "questions_json": `{"q":{"type":"choice","instructions":"?","criteria":{"one":"1"}}}`})
	require.NoError(t, err)
	require.True(t, out.IsError)
	require.Contains(t, out.Details["results"], "2 to 255 criteria")

	out, _ = NewTool(nil).Handler()(context.Background(), map[string]any{"state": "x", "questions_json": questionsJSON})
	require.True(t, out.IsError)
}

func TestModelReportsTheGenerationsOwnUsage(t *testing.T) {
	req, err := ParseRequest(map[string]any{"state": "one two three", "questions_json": `{"ok":{"type":"noul","instructions":"Is it?"}}`})
	require.NoError(t, err)
	gen := &fakeGen{reply: `{"ok": true}`, usage: &ai.TokenCount{InputTokens: 100, OutputTokens: 20, TotalTokens: 120}}
	resp, err := Model{Gen: gen}.Decide(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 100, resp.Usage.InputTokens, "what the client reported for this request, not an estimate")
	require.Equal(t, 20, resp.Usage.OutputTokens)

	resp, err = Model{Gen: &fakeGen{reply: `{"ok": true}`}}.Decide(context.Background(), req)
	require.NoError(t, err)
	require.Zero(t, resp.Usage.InputTokens, "a client that reports no usage leaves the count at zero")
}

func TestModelPassesCallerTextAsDataNotTemplate(t *testing.T) {
	// Clients render the prompt text as a Go template; a caller's state
	// must reach the model untouched, braces and all.
	state := "Dear {{customer_name}}, your order {{.id}} is {{ late }}"
	req, err := ParseRequest(map[string]any{"state": state, "questions_json": `{"ok":{"type":"noul","instructions":"Is {{it}}?"}}`})
	require.NoError(t, err)
	gen := &fakeGen{reply: `{"ok": false}`}
	_, err = Model{Gen: gen}.Decide(context.Background(), req)
	require.NoError(t, err)
	require.Contains(t, gen.prompt.Text, state)
	require.Contains(t, gen.prompt.Text, "Is {{it}}?")
}

func TestJevOfficialShape(t *testing.T) {
	// The official API requires a model on every request and reports
	// tokens without a price; a noul may say what true and false mean.
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"model":"jev-1.13.0","answers":{"escalate":{"type":"noul","noul":0.2}},"usage":{"input_tokens":40,"output_tokens":3}}`))
	}))
	defer server.Close()
	require.Equal(t, "https://api.typesafe.ai/v1/systemone", DefaultJevURL)

	req, err := ParseRequest(map[string]any{"state": "x", "questions_json": `{"escalate":{"type":"noul","instructions":"Escalate?","criteria":{"true":"a human must step in","false":"the agent can continue"}}}`})
	require.NoError(t, err)
	resp, err := Jev{URL: server.URL, APIKey: "k"}.Decide(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, DefaultJevModel, got["model"], "a model is always sent")
	require.Equal(t, "a human must step in", got["questions"].(map[string]any)["escalate"].(map[string]any)["criteria"].(map[string]any)["true"])
	require.Equal(t, 40, resp.Usage.InputTokens)
	require.Equal(t, 3, resp.Usage.OutputTokens)
	require.Equal(t, 0.0, resp.Usage.CostUSD)

	for code, want := range map[int]string{401: "refused", 422: "rejected", 429: "rate limited", 529: "overloaded"} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		_, err := Jev{URL: s.URL, APIKey: "k"}.Decide(context.Background(), req)
		s.Close()
		require.ErrorContains(t, err, want)
	}
}
