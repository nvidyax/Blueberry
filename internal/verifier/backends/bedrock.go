package backends

// Supported AWS Bedrock Models (as of April 2026):
//   - anthropic.claude-haiku-4-5-20251001-v1:0  (default — fast, cost-effective)
//   - anthropic.claude-sonnet-4-6               (balanced, high quality)
//   - anthropic.claude-opus-4-6-v1              (most capable)

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/blueberry/mcp/internal/verifier"
)

type BedrockBackend struct {
	Model  string
	client *bedrockruntime.Client
}

func NewBedrockBackend(model string) (*BedrockBackend, error) {
	if model == "" {
		model = "anthropic.claude-haiku-4-5-20251001-v1:0"
	}
	
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	client := bedrockruntime.NewFromConfig(cfg)

	return &BedrockBackend{
		Model:  model,
		client: client,
	}, nil
}

func (b *BedrockBackend) Name() string {
	return "bedrock"
}

// callBedrock is the shared InvokeModel call for all Bedrock requests.
func (bk *BedrockBackend) callBedrock(ctx context.Context, prompt string, maxTokens int) (string, error) {
	payload := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        maxTokens,
		"temperature":       0.0,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": prompt,
					},
				},
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	contentType := "application/json"
	accept := "application/json"
	
	output, err := bk.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     &bk.Model,
		ContentType: &contentType,
		Accept:      &accept,
		Body:        payloadBytes,
	})
	if err != nil {
		return "", err
	}

	var res struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(output.Body, &res); err != nil {
		return "", err
	}

	if len(res.Content) == 0 {
		return "", fmt.Errorf("no content returned from Bedrock")
	}

	return strings.TrimSpace(res.Content[0].Text), nil
}

func (bk *BedrockBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if bk.client == nil {
		return nil, fmt.Errorf("AWS Bedrock client is not initialized")
	}

	evidence := buildEvidenceText(spans)
	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		var conf float64
		var reason, corrected string
		var err error

		if st.Enrich {
			prompt := buildEnrichPrompt(st.Claim, evidence)
			text, callErr := bk.callBedrock(ctx, prompt, 200)
			if callErr != nil {
				return nil, fmt.Errorf("bedrock error on step %d: %w", i, callErr)
			}
			conf, reason, corrected, err = parseEnrichedJSON(text)
			if err != nil {
				return nil, fmt.Errorf("bedrock enrich parse error on step %d: %w", i, err)
			}
		} else {
			prompt := buildVerifyPrompt(st.Claim, evidence, false)
			text, callErr := bk.callBedrock(ctx, prompt, 10)
			if callErr != nil {
				return nil, fmt.Errorf("bedrock error on step %d: %w", i, callErr)
			}
			conf, err = parseHeuristicFloat(text)
			if err != nil {
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

func (bk *BedrockBackend) GetEmbeddings(ctx context.Context, text []string) ([][]float64, error) {
	return nil, fmt.Errorf("GetEmbeddings not implemented natively for bedrock via this interface")
}

func (bk *BedrockBackend) ParseAtomicClaims(ctx context.Context, text string) ([]string, error) {
	prompt := buildParseClaimsPrompt(text)
	content, err := bk.callBedrock(ctx, prompt, 1024)
	if err != nil {
		return strings.Split(text, ". "), nil
	}
	return parseClaimsJSON(content, text), nil
}

func (bk *BedrockBackend) EvaluateNLI(ctx context.Context, contextText string, claim string) (string, float64, error) {
	return "Neutral", 0.5, fmt.Errorf("EvaluateNLI not implemented for bedrock")
}
