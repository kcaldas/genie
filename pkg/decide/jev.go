package decide

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultJevURL is the official System One endpoint. A third-party proxy
// (https://jevtypesafeai.com/api/v1/decide) takes the same request with
// its own keys; point URL at it to use one.
const DefaultJevURL = "https://api.typesafe.ai/v1/systemone"

// DefaultJevModel is the model alias sent when none is pinned; the
// official API requires a model on every request.
const DefaultJevModel = "jev-latest"

// Jev answers with the Jev decision API: the request goes out as is and
// the answers come back with calibrated probabilities.
type Jev struct {
	// URL is the endpoint; DefaultJevURL when empty.
	URL string
	// APIKey is sent as a bearer token. Never logged.
	APIKey string
	// ModelName pins a Jev version ("jev-1.13.0"); empty means the latest.
	ModelName string
	// HTTPClient is used when set; a 30 s client otherwise.
	HTTPClient *http.Client
}

type jevRequest struct {
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
	Model     string              `json:"model,omitempty"`
}

type jevAnswer struct {
	Type          Type               `json:"type"`
	Choice        string             `json:"choice"`
	Score         *float64           `json:"score"`
	Noul          *float64           `json:"noul"`
	Confidence    *float64           `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

type jevResponse struct {
	Model   string               `json:"model"`
	Answers map[string]jevAnswer `json:"answers"`
	Usage   struct {
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		CostUSD      float64 `json:"cost_usd"` // the proxy adds it; the official API does not
	} `json:"usage"`
	Error any `json:"error"`
}

// Decide implements Decider.
func (j Jev) Decide(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(j.APIKey) == "" {
		return Response{}, fmt.Errorf("decide: jev: no API key")
	}
	if err := Validate(req); err != nil {
		return Response{}, err
	}
	model := strings.TrimSpace(j.ModelName)
	if model == "" {
		model = DefaultJevModel
	}
	body, err := json.Marshal(jevRequest{State: req.State, Questions: req.Questions, Model: model})
	if err != nil {
		return Response{}, err
	}
	url := strings.TrimSpace(j.URL)
	if url == "" {
		url = DefaultJevURL
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+j.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	client := j.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	started := time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("decide: jev: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Response{}, fmt.Errorf("decide: jev: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return Response{}, fmt.Errorf("decide: jev: the API key was refused (401)")
	case http.StatusPaymentRequired:
		return Response{}, fmt.Errorf("decide: jev: prepaid balance is empty (402)")
	case http.StatusUnprocessableEntity:
		return Response{}, fmt.Errorf("decide: jev: request rejected (422): %s", excerpt(payload))
	case http.StatusTooManyRequests:
		return Response{}, fmt.Errorf("decide: jev: rate limited (429); retry with backoff")
	case 529:
		return Response{}, fmt.Errorf("decide: jev: overloaded (529); retry with backoff")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("decide: jev: HTTP %d: %s", resp.StatusCode, excerpt(payload))
	}
	var parsed jevResponse
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return Response{}, fmt.Errorf("decide: jev: not a decide response: %w", err)
	}
	if parsed.Error != nil {
		return Response{}, fmt.Errorf("decide: jev: %v", parsed.Error)
	}
	answers := make(map[string]Answer, len(req.Questions))
	for name, q := range req.Questions {
		a, ok := parsed.Answers[name]
		if !ok {
			return Response{}, fmt.Errorf("decide: jev did not answer %q", name)
		}
		answer := Answer{Type: q.Type, Confidence: a.Confidence, Probabilities: a.Probabilities}
		switch q.Type {
		case TypeChoice:
			if _, known := q.Criteria[a.Choice]; !known {
				return Response{}, fmt.Errorf("decide: jev chose %q for %q, not one of the options", a.Choice, name)
			}
			answer.Choice = a.Choice
		case TypeScore:
			if a.Score == nil {
				return Response{}, fmt.Errorf("decide: jev gave no score for %q", name)
			}
			answer.Score = a.Score
		case TypeNoul:
			if a.Noul == nil {
				return Response{}, fmt.Errorf("decide: jev gave no noul for %q", name)
			}
			answer.Noul = a.Noul
		}
		answers[name] = answer
	}
	return Response{
		Backend: "jev:" + parsed.Model,
		Answers: answers,
		Usage:   Usage{InputTokens: parsed.Usage.InputTokens, OutputTokens: parsed.Usage.OutputTokens, CostUSD: parsed.Usage.CostUSD, LatencyMs: time.Since(started).Milliseconds()},
	}, nil
}

func excerpt(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
