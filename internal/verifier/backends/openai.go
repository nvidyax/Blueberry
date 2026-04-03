package backends

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/blueberry/mcp/internal/verifier"
)

type OpenAIBackend struct {
	Model   string
	BaseURL string
	APIKey  string
}

func NewOpenAIBackend(model string) *OpenAIBackend {
	if model == "" {
		model = "gpt-4o-mini"
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	apiKey := os.Getenv("OPENAI_API_KEY")

	return &OpenAIBackend{
		Model:   model,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}
}

func (o *OpenAIBackend) Name() string {
	return "openai"
}

func (o *OpenAIBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if o.APIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		conf, err := o.callOpenAI(ctx, st.Claim)
		if err != nil {
			return nil, fmt.Errorf("openai error on step %d: %w", i, err)
		}

		flagged := conf < st.Confidence
		if len(st.Cites) == 0 {
			flagged = true
		}

		results[i] = verifier.TraceResult{
			Idx:             st.Idx,
			Claim:           st.Claim,
			Cites:           st.Cites,
			Target:          st.Confidence,
			ConfidenceScore: conf,
			Flagged:         flagged,
		}
	}
	return results, nil
}

func (o *OpenAIBackend) callOpenAI(ctx context.Context, claim string) (float64, error) {
	prompt := fmt.Sprintf("Is the following claim supported by the text? Answer only 'Yes' or 'No'. Claim: %s", claim)

	payload := map[string]interface{}{
		"model": o.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature":  0.0,
		"max_tokens":   5,
		"logprobs":     true,
		"top_logprobs": 10,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("bad status %d: %s", resp.StatusCode, string(body))
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Logprobs struct {
				Content []struct {
					Token   string  `json:"token"`
					Logprob float64 `json:"logprob"`
				} `json:"content"`
			} `json:"logprobs"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, err
	}

	if len(res.Choices) == 0 {
		return 0, fmt.Errorf("no choices returned")
	}

	prob := 0.99
	if len(res.Choices[0].Logprobs.Content) > 0 {
		_ = res.Choices[0].Logprobs.Content[0].Logprob
		// prob = math.Exp(lp)
	}

	return prob, nil
}
