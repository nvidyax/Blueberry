package backends

// Supported OpenAI Models (as of April 2026):
//   - gpt-5.4-mini    (default — fast, cost-efficient for semantic validation)
//   - gpt-5.4-thinking (deep reasoning, higher cost)
//   - gpt-5.4-pro     (most capable, research-grade)
//   - gpt-5.3-instant (legacy fast model)
//
// Embedding model: text-embedding-3-small (still current)

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

type OpenAIBackend struct {
	Model   string
	BaseURL string
	APIKey  string
}

func NewOpenAIBackend(model string) *OpenAIBackend {
	if model == "" {
		model = "gpt-5.4-mini"
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

	evidence := buildEvidenceText(spans)
	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		var conf float64
		var err error
		var reason, corrected string

		if st.Enrich {
			conf, reason, corrected, err = o.callOpenAIEnrich(ctx, st.Claim, evidence)
		} else {
			conf, err = o.callOpenAI(ctx, st.Claim, evidence)
		}

		if err != nil {
			return nil, fmt.Errorf("openai error on step %d: %w", i, err)
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

func (o *OpenAIBackend) callOpenAI(ctx context.Context, claim string, evidence string) (float64, error) {
	prompt := buildVerifyPrompt(claim, evidence, false)

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
		lp := res.Choices[0].Logprobs.Content[0].Logprob
		prob = math.Exp(lp)
	}

	return prob, nil
}

func (o *OpenAIBackend) callOpenAIEnrich(ctx context.Context, claim string, evidence string) (float64, string, string, error) {
	prompt := buildEnrichPrompt(claim, evidence)

	payload := map[string]interface{}{
		"model": o.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature":  0.0,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, "", "", fmt.Errorf("bad status %d: %s", resp.StatusCode, string(body))
	}

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, "", "", err
	}

	if len(res.Choices) == 0 {
		return 0, "", "", fmt.Errorf("no choices returned")
	}

	return parseEnrichedJSON(res.Choices[0].Message.Content)
}

func (o *OpenAIBackend) GetEmbeddings(ctx context.Context, text []string) ([][]float64, error) {
	if len(text) == 0 {
		return nil, nil
	}
	
	payload := map[string]interface{}{
		"model": "text-embedding-3-small",
		"input": text,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status in embeddings %d", resp.StatusCode)
	}

	var res struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	embeddings := make([][]float64, len(res.Data))
	for i, d := range res.Data {
		embeddings[i] = d.Embedding
	}
	return embeddings, nil
}

func (o *OpenAIBackend) ParseAtomicClaims(ctx context.Context, text string) ([]string, error) {
	prompt := fmt.Sprintf("Split the following text into an array of completely atomic, standalone factual claims. Ensure all pronouns are resolved. Output ONLY a valid JSON array of strings, nothing else. Text: %s", text)
	
	payload := map[string]interface{}{
		"model": o.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.0,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if len(res.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned")
	}

	content := stripCodeFences(res.Choices[0].Message.Content)

	var claims []string
	if err := json.Unmarshal([]byte(content), &claims); err != nil {
		// fallback to simple string split
		return strings.Split(text, ". "), nil
	}

	return claims, nil
}

func (o *OpenAIBackend) EvaluateNLI(ctx context.Context, contextText string, claim string) (string, float64, error) {
	prompt := fmt.Sprintf("Context: %s\n\nClaim: %s\n\nIs the claim Entailed, Contradicted, or Neutral with respect to the Context? Output strictly ONE word: 'Entailment', 'Contradiction', or 'Neutral'.", contextText, claim)
	
	payload := map[string]interface{}{
		"model": o.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.0,
		"max_tokens": 5,
		"logprobs": true,
		"top_logprobs": 5,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", 0.0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0.0, err
	}
	defer resp.Body.Close()

	var res struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Logprobs struct {
				Content []struct {
					Logprob float64 `json:"logprob"`
				} `json:"content"`
			} `json:"logprobs"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", 0.0, err
	}

	if len(res.Choices) == 0 {
		return "Neutral", 0.0, fmt.Errorf("no choices returned")
	}

	ans := strings.TrimSpace(res.Choices[0].Message.Content)
	
	prob := 0.99
	if len(res.Choices[0].Logprobs.Content) > 0 {
		lp := res.Choices[0].Logprobs.Content[0].Logprob
		prob = math.Exp(lp)
	}

	if strings.Contains(strings.ToLower(ans), "entail") {
		return "Entailment", prob, nil
	} else if strings.Contains(strings.ToLower(ans), "contradiction") {
		return "Contradiction", prob, nil
	}
	return "Neutral", prob, nil
}
