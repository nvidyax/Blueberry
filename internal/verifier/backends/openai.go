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

	var evidenceText strings.Builder
	for _, sp := range spans {
		evidenceText.WriteString(fmt.Sprintf("[%s] %s\n", sp["SID"], sp["Text"]))
	}
	evidence := strings.TrimSpace(evidenceText.String())

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
	var prompt string
	if evidence != "" {
		prompt = fmt.Sprintf("Evidence:\n%s\n\nIs the following claim strictly supported by the evidence? Answer only 'Yes' or 'No'. Claim: %s", evidence, claim)
	} else {
		prompt = fmt.Sprintf("Is the following claim supported by general knowledge? Answer only 'Yes' or 'No'. Claim: %s", claim)
	}

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
	var prompt string
	if evidence != "" {
		prompt = fmt.Sprintf("Evidence:\n%s\n\nIs the following claim strictly supported by the evidence? Claim: %s\nOutput ONLY a valid JSON object with: 1. 'supported' (boolean), 2. 'confidence' (float 0.00 to 1.00), 3. 'reason' (short string tag), 4. 'corrected' (string, corrected claim).", evidence, claim)
	} else {
		prompt = fmt.Sprintf("Is the following claim supported by general knowledge? Claim: %s\nOutput ONLY a valid JSON object with: 1. 'supported' (boolean), 2. 'confidence' (float 0.00 to 1.00), 3. 'reason' (short string tag), 4. 'corrected' (string, corrected claim).", claim)
	}

	payload := map[string]interface{}{
		"model": o.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature":  0.0,
	}

	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(b))
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

	content := strings.TrimSpace(res.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var enriched struct {
		Supported  bool    `json:"supported"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
		Corrected  string  `json:"corrected"`
	}

	if err := json.Unmarshal([]byte(content), &enriched); err != nil {
		return 0, "", "", fmt.Errorf("JSON unmarshal error: %w", err)
	}

	return enriched.Confidence, enriched.Reason, enriched.Corrected, nil
}

func (o *OpenAIBackend) GetEmbeddings(ctx context.Context, text []string) ([][]float64, error) {
	if len(text) == 0 {
		return nil, nil
	}
	
	payload := map[string]interface{}{
		"model": "text-embedding-3-small", // standard small embedding model
		"input": text,
	}

	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/embeddings", bytes.NewReader(b))
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

	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(b))
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

	content := strings.TrimSpace(res.Choices[0].Message.Content)
	// simple cleanup if it wraps in markdown code blocks
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

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

	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/chat/completions", bytes.NewReader(b))
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

