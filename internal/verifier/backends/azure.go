package backends

// Supported Azure OpenAI Models (as of April 2026):
//   - gpt-5.4-mini     (default — cost-efficient, fast)
//   - gpt-5.4-thinking (deep reasoning)
//   - gpt-5.4-pro      (most capable)
//   - gpt-5.3-instant  (legacy fast)
//
// Note: Model name here refers to the Azure deployment name.
// Azure API Version: 2025-12-01-preview

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
		model = "gpt-5.4-mini"
	}

	endpoint := os.Getenv("AZURE_OPENAI_ENDPOINT")
	apiKey := os.Getenv("AZURE_OPENAI_API_KEY")
	apiVersion := os.Getenv("AZURE_OPENAI_API_VERSION")
	if apiVersion == "" {
		apiVersion = "2025-12-01-preview"
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

// callAzureChat is the shared HTTP call for all Azure OpenAI chat completions.
func (a *AzureBackend) callAzureChat(ctx context.Context, messages []map[string]string, maxTokens int, useLogprobs bool) (string, float64, error) {
	payload := map[string]interface{}{
		"messages":    messages,
		"temperature": 0.0,
		"max_tokens":  maxTokens,
	}
	if useLogprobs {
		payload["logprobs"] = true
		payload["top_logprobs"] = 10
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", 0, fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s", a.Endpoint, a.Model, a.APIVersion)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("api-key", a.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("bad status %d: %s", resp.StatusCode, string(body))
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
		return "", 0, err
	}

	if len(res.Choices) == 0 {
		return "", 0, fmt.Errorf("no choices returned")
	}

	content := strings.TrimSpace(res.Choices[0].Message.Content)

	prob := 0.99
	if useLogprobs && len(res.Choices[0].Logprobs.Content) > 0 {
		lp := res.Choices[0].Logprobs.Content[0].Logprob
		prob = math.Exp(lp)
	}

	return content, prob, nil
}

func (a *AzureBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if a.APIKey == "" || a.Endpoint == "" {
		return nil, fmt.Errorf("AZURE_OPENAI_API_KEY or AZURE_OPENAI_ENDPOINT is not set")
	}

	evidence := buildEvidenceText(spans)
	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		var conf float64
		var reason, corrected string
		var err error

		if st.Enrich {
			prompt := buildEnrichPrompt(st.Claim, evidence)
			msgs := []map[string]string{{"role": "user", "content": prompt}}
			text, _, callErr := a.callAzureChat(ctx, msgs, 200, false)
			if callErr != nil {
				return nil, fmt.Errorf("azure error on step %d: %w", i, callErr)
			}
			conf, reason, corrected, err = parseEnrichedJSON(text)
			if err != nil {
				return nil, fmt.Errorf("azure enrich parse error on step %d: %w", i, err)
			}
		} else {
			prompt := buildVerifyPrompt(st.Claim, evidence, false)
			msgs := []map[string]string{{"role": "user", "content": prompt}}
			_, prob, callErr := a.callAzureChat(ctx, msgs, 5, true)
			if callErr != nil {
				return nil, fmt.Errorf("azure error on step %d: %w", i, callErr)
			}
			conf = prob
		}

		flagged := conf < st.Confidence
		if len(st.Cites) == 0 && evidence != "" {
			flagged = true
			conf = 0.5
		}

		results[i] = verifier.TraceResult{
			Idx:             st.Idx,
			Claim:           st.Claim,
			Cites:           st.Cites,
			Target:          st.Confidence,
			ConfidenceScore: conf,
			Flagged:         flagged,
			Reason:          reason,
			CorrectedClaim:  corrected,
		}
	}
	return results, nil
}

func (a *AzureBackend) GetEmbeddings(ctx context.Context, text []string) ([][]float64, error) {
	return nil, fmt.Errorf("GetEmbeddings not implemented natively for azure via this interface")
}

func (a *AzureBackend) ParseAtomicClaims(ctx context.Context, text string) ([]string, error) {
	if a.APIKey == "" || a.Endpoint == "" {
		return strings.Split(text, ". "), nil
	}

	prompt := buildParseClaimsPrompt(text)
	msgs := []map[string]string{{"role": "user", "content": prompt}}
	content, _, err := a.callAzureChat(ctx, msgs, 1024, false)
	if err != nil {
		return strings.Split(text, ". "), nil
	}

	return parseClaimsJSON(content, text), nil
}

func (a *AzureBackend) EvaluateNLI(ctx context.Context, contextText string, claim string) (string, float64, error) {
	return "Neutral", 0.5, fmt.Errorf("EvaluateNLI not implemented for azure")
}
