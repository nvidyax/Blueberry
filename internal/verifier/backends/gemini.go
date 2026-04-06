package backends

// Supported Google Gemini Models (as of April 2026):
//   - gemini-3.1-flash      (default — fast, cost-efficient for semantic validation)
//   - gemini-3.1-flash-lite (cheapest, highest throughput)
//   - gemini-3.1-pro        (most capable, complex reasoning)

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

type GeminiBackend struct {
	Model   string
	APIKey  string
	BaseURL string
}

func NewGeminiBackend(model string) *GeminiBackend {
	if model == "" {
		model = "gemini-3.1-flash"
	}
	baseURL := os.Getenv("GEMINI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta/models"
	}
	apiKey := os.Getenv("GEMINI_API_KEY")

	return &GeminiBackend{
		Model:   model,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}
}

func (g *GeminiBackend) Name() string {
	return "gemini"
}

// callGemini is the shared HTTP call for all Gemini generateContent requests.
func (g *GeminiBackend) callGemini(ctx context.Context, prompt string) (string, error) {
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.0,
		},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", g.BaseURL, g.Model, g.APIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

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
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content returned")
	}

	return strings.TrimSpace(res.Candidates[0].Content.Parts[0].Text), nil
}

func (g *GeminiBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if g.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}

	evidence := buildEvidenceText(spans)
	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		var conf float64
		var reason, corrected string
		var err error

		if st.Enrich {
			prompt := buildEnrichPrompt(st.Claim, evidence)
			text, callErr := g.callGemini(ctx, prompt)
			if callErr != nil {
				return nil, fmt.Errorf("gemini error on step %d: %w", i, callErr)
			}
			conf, reason, corrected, err = parseEnrichedJSON(text)
			if err != nil {
				return nil, fmt.Errorf("gemini enrich parse error on step %d: %w", i, err)
			}
		} else {
			prompt := buildVerifyPrompt(st.Claim, evidence, false)
			text, callErr := g.callGemini(ctx, prompt)
			if callErr != nil {
				return nil, fmt.Errorf("gemini error on step %d: %w", i, callErr)
			}
			conf, err = parseHeuristicFloat(text)
			if err != nil {
				// Non-fatal: use 0.5 as fallback
				conf = 0.5
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

func (g *GeminiBackend) callGeminiHeuristic(ctx context.Context, claim string, evidence string) (float64, error) {
	prompt := buildVerifyPrompt(claim, evidence, false)
	text, err := g.callGemini(ctx, prompt)
	if err != nil {
		return 0, err
	}

	score, err := strconv.ParseFloat(text, 64)
	if err != nil {
		if strings.Contains(strings.ToLower(text), "1.00") {
			return 1.0, nil
		}
		if strings.Contains(strings.ToLower(text), "0.00") {
			return 0.0, nil
		}
		return 0.5, fmt.Errorf("could not parse valid float from Gemini: %s", text)
	}

	return score, nil
}

func (g *GeminiBackend) GetEmbeddings(ctx context.Context, text []string) ([][]float64, error) {
	return nil, fmt.Errorf("GetEmbeddings not implemented natively for gemini via this interface")
}

func (g *GeminiBackend) ParseAtomicClaims(ctx context.Context, text string) ([]string, error) {
	if g.APIKey == "" {
		return strings.Split(text, ". "), nil
	}

	prompt := buildParseClaimsPrompt(text)
	content, err := g.callGemini(ctx, prompt)
	if err != nil {
		return strings.Split(text, ". "), nil
	}

	return parseClaimsJSON(content, text), nil
}

func (g *GeminiBackend) EvaluateNLI(ctx context.Context, contextText string, claim string) (string, float64, error) {
	return "Neutral", 0.5, fmt.Errorf("EvaluateNLI not implemented for gemini")
}
