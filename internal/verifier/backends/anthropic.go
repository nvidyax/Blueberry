package backends

// Supported Anthropic Models (as of April 2026):
//   - claude-sonnet-4-6-20260217  (default — balanced, high price-to-performance)
//   - claude-opus-4-6-20260205    (most capable, complex reasoning)
//   - claude-haiku-4-5            (fastest, most cost-effective)

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/blueberry/mcp/internal/verifier"
)

type AnthropicBackend struct {
	Model   string
	APIKey  string
	BaseURL string
}

func NewAnthropicBackend(model string) *AnthropicBackend {
	if model == "" {
		model = "claude-sonnet-4-6-20260217"
	}
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}
	return &AnthropicBackend{
		Model:   model,
		APIKey:  os.Getenv("ANTHROPIC_API_KEY"),
		BaseURL: baseURL,
	}
}

func (a *AnthropicBackend) Name() string {
	return "anthropic"
}

// callAnthropicChat is the shared HTTP call for all Anthropic Messages API requests.
func (a *AnthropicBackend) callAnthropicChat(ctx context.Context, prompt string, maxTokens int) (string, error) {
	payload := map[string]interface{}{
		"model":      a.Model,
		"max_tokens": maxTokens,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.0,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/messages", bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2025-01-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("bad status %d: %s", resp.StatusCode, string(body))
	}

	var res struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if len(res.Content) == 0 {
		return "", fmt.Errorf("no content returned")
	}

	return strings.TrimSpace(res.Content[0].Text), nil
}

func (a *AnthropicBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}

	evidence := buildEvidenceText(spans)
	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		var conf float64
		var reason, corrected string
		var err error

		if st.Enrich {
			prompt := buildEnrichPrompt(st.Claim, evidence)
			text, callErr := a.callAnthropicChat(ctx, prompt, 200)
			if callErr != nil {
				return nil, fmt.Errorf("anthropic error on step %d: %w", i, callErr)
			}
			conf, reason, corrected, err = parseEnrichedJSON(text)
			if err != nil {
				return nil, fmt.Errorf("anthropic enrich parse error on step %d: %w", i, err)
			}
		} else {
			prompt := buildVerifyPrompt(st.Claim, evidence, false)
			text, callErr := a.callAnthropicChat(ctx, prompt, 10)
			if callErr != nil {
				return nil, fmt.Errorf("anthropic error on step %d: %w", i, callErr)
			}
			conf, err = parseHeuristicFloat(text)
			if err != nil {
				return nil, fmt.Errorf("anthropic parse error on step %d: %w", i, err)
			}
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

func (a *AnthropicBackend) GetEmbeddings(ctx context.Context, text []string) ([][]float64, error) {
	return nil, fmt.Errorf("GetEmbeddings not implemented natively for anthropic via this interface")
}

func (a *AnthropicBackend) ParseAtomicClaims(ctx context.Context, text string) ([]string, error) {
	if a.APIKey == "" {
		// Fallback to naive split if no API key
		return strings.Split(text, ". "), nil
	}

	prompt := buildParseClaimsPrompt(text)
	content, err := a.callAnthropicChat(ctx, prompt, 1024)
	if err != nil {
		// Fallback to naive split on error
		return strings.Split(text, ". "), nil
	}

	return parseClaimsJSON(content, text), nil
}

func (a *AnthropicBackend) EvaluateNLI(ctx context.Context, contextText string, claim string) (string, float64, error) {
	return "Neutral", 0.5, fmt.Errorf("EvaluateNLI not implemented for anthropic")
}

// parseHeuristicFloat attempts to parse a float from an LLM response.
// Falls back to scanning for common float patterns.
func parseHeuristicFloat(text string) (float64, error) {
	score, err := strconv.ParseFloat(text, 64)
	if err == nil {
		return score, nil
	}
	if strings.Contains(strings.ToLower(text), "1.00") {
		return 1.0, nil
	}
	if strings.Contains(strings.ToLower(text), "0.00") {
		return 0.0, nil
	}
	return 0.5, fmt.Errorf("could not parse valid float: %s", text)
}
