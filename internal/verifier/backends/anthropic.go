package backends

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
		model = "claude-3-haiku-20240307"
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

func (a *AnthropicBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if a.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
	}

	var evidenceText strings.Builder
	for _, sp := range spans {
		evidenceText.WriteString(fmt.Sprintf("[%s] %s\n", sp["SID"], sp["Text"]))
	}
	evidence := strings.TrimSpace(evidenceText.String())

	results := make([]verifier.TraceResult, len(steps))
	for i, st := range steps {
		conf, err := a.callAnthropicHeuristic(ctx, st.Claim, evidence)
		if err != nil {
			return nil, fmt.Errorf("anthropic error on step %d: %w", i, err)
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
		}
	}
	return results, nil
}

func (a *AnthropicBackend) callAnthropicHeuristic(ctx context.Context, claim string, evidence string) (float64, error) {
	var prompt string
	if evidence != "" {
		prompt = fmt.Sprintf("Evidence:\n%s\n\nIs the following claim strictly supported by the evidence? Claim: %s\nOutput ONLY a float between 0.00 and 1.00 indicating your confidence.", evidence, claim)
	} else {
		prompt = fmt.Sprintf("Is the following claim supported by general knowledge? Claim: %s\nOutput ONLY a float between 0.00 and 1.00 indicating your confidence.", claim)
	}

	payload := map[string]interface{}{
		"model":      a.Model,
		"max_tokens": 10,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.0,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/messages", bytes.NewReader(b))
	if err != nil {
		return 0, err
	}

	req.Header.Set("x-api-key", a.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
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
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, err
	}

	if len(res.Content) == 0 {
		return 0, fmt.Errorf("no content returned")
	}

	textResp := strings.TrimSpace(res.Content[0].Text)
	score, err := strconv.ParseFloat(textResp, 64)
	if err != nil {
		// heuristic fallback
		if strings.Contains(strings.ToLower(textResp), "1.00") {
			return 1.0, nil
		}
		return 0.5, fmt.Errorf("could not parse valid float from Claude: %s", textResp)
	}

	return score, nil
}

func (a *AnthropicBackend) GetEmbeddings(ctx context.Context, text []string) ([][]float64, error) {
	return nil, fmt.Errorf("GetEmbeddings not implemented natively for anthropic via this interface")
}

func (a *AnthropicBackend) ParseAtomicClaims(ctx context.Context, text string) ([]string, error) {
	// Simplified parsing, could use heuristic prompt
	return strings.Split(text, ". "), nil
}

func (a *AnthropicBackend) EvaluateNLI(ctx context.Context, contextText string, claim string) (string, float64, error) {
	return "Neutral", 0.5, fmt.Errorf("EvaluateNLI not implemented for anthropic")
}

