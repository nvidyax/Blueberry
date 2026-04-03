package backends

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"

	"github.com/blueberry/mcp/internal/verifier"
)

type AzureBackend struct {
	Model      string
	Endpoint   string
	APIKey     string
	APIVersion string
}

func NewAzureBackend(model string) *AzureBackend {
	if model == "" {
		model = "gpt-4" // Common deployment name default
	}

	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	apiKey := os.Getenv("AZURE_OPENAI_API_KEY")
	apiVersion := os.Getenv("AZURE_OPENAI_API_VERSION")
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}

	return &AzureBackend{
		Model:      model,
		Endpoint:   strings.TrimRight(endpoint, "/"),
		APIKey:     apiKey,
		APIVersion: apiVersion,
	}
}

func (a *AzureBackend) Name() string {
	return "azure"
}

func (a *AzureBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if a.APIKey == "" || a.Endpoint == "" {
		return nil, fmt.Errorf("AZURE_OPENAI_API_KEY or AZURE_OPENAI_ENDPOINT is not set")
	}

	var evidenceText strings.Builder
	for _, sp := range spans {
		evidenceText.WriteString(fmt.Sprintf("[%s] %s\n", sp["SID"], sp["Text"]))
	}
	evidence := strings.TrimSpace(evidenceText.String())

	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		conf, err := a.callAzureOpenAI(ctx, st.Claim, evidence)
		if err != nil {
			return nil, fmt.Errorf("azure error on step %d: %w", i, err)
		}

		flagged := conf < st.Confidence
		if len(st.Cites) == 0 && evidence != "" {
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

func (a *AzureBackend) callAzureOpenAI(ctx context.Context, claim string, evidence string) (float64, error) {
	var prompt string
	if evidence != "" {
		prompt = fmt.Sprintf("Evidence:\n%s\n\nIs the following claim strictly supported by the evidence? Answer only 'Yes' or 'No'. Claim: %s", evidence, claim)
	} else {
		prompt = fmt.Sprintf("Is the following claim supported by general knowledge? Answer only 'Yes' or 'No'. Claim: %s", claim)
	}

	payload := map[string]interface{}{
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

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", a.Endpoint, a.Model, a.APIVersion)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return 0, err
	}

	req.Header.Set("api-key", a.APIKey)
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
		lp := res.Choices[0].Logprobs.Content[0].Logprob
		prob = math.Exp(lp)
	}

	return prob, nil
}
