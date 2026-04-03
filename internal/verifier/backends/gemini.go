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

type GeminiBackend struct {
	Model   string
	APIKey  string
	BaseURL string
}

func NewGeminiBackend(model string) *GeminiBackend {
	if model == "" {
		model = "gemini-1.5-flash"
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

func (g *GeminiBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if g.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is not set")
	}

	var evidenceText strings.Builder
	for _, sp := range spans {
		evidenceText.WriteString(fmt.Sprintf("[%s] %s\n", sp["SID"], sp["Text"]))
	}
	evidence := strings.TrimSpace(evidenceText.String())

	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		conf, err := g.callGeminiHeuristic(ctx, st.Claim, evidence)
		if err != nil {
			return nil, fmt.Errorf("gemini error on step %d: %w", i, err)
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

func (g *GeminiBackend) callGeminiHeuristic(ctx context.Context, claim string, evidence string) (float64, error) {
	var prompt string
	if evidence != "" {
		prompt = fmt.Sprintf("Evidence:\n%s\n\nIs the following claim strictly supported by the evidence? Claim: %s\nOutput ONLY a float between 0.00 and 1.00 indicating your confidence.", evidence, claim)
	} else {
		prompt = fmt.Sprintf("Is the following claim supported by general knowledge? Claim: %s\nOutput ONLY a float between 0.00 and 1.00 indicating your confidence.", claim)
	}

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
		return 0, err
	}

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", g.BaseURL, g.Model, g.APIKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return 0, err
	}

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
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, err
	}

	if len(res.Candidates) == 0 || len(res.Candidates[0].Content.Parts) == 0 {
		return 0, fmt.Errorf("no content returned")
	}

	textResp := strings.TrimSpace(res.Candidates[0].Content.Parts[0].Text)
	score, err := strconv.ParseFloat(textResp, 64)
	if err != nil {
		if strings.Contains(strings.ToLower(textResp), "1.00") {
			return 1.0, nil
		}
		if strings.Contains(strings.ToLower(textResp), "0.00") {
			return 0.0, nil
		}
		return 0.5, fmt.Errorf("could not parse valid float from Gemini: %s", textResp)
	}

	return score, nil
}
