package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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
		model = "anthropic.claude-3-haiku-20240307-v1:0"
	}
	
	// Bedrock requires valid AWS credentials to be configured in the environment or files
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

func (b *BedrockBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if b.client == nil {
		return nil, fmt.Errorf("AWS Bedrock client is not initialized")
	}

	var evidenceText strings.Builder
	for _, sp := range spans {
		evidenceText.WriteString(fmt.Sprintf("[%s] %s\n", sp["SID"], sp["Text"]))
	}
	evidence := strings.TrimSpace(evidenceText.String())

	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		conf, err := b.callBedrockHeuristic(ctx, st.Claim, evidence)
		if err != nil {
			return nil, fmt.Errorf("bedrock error on step %d: %w", i, err)
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

func (b *BedrockBackend) callBedrockHeuristic(ctx context.Context, claim string, evidence string) (float64, error) {
	var prompt string
	if evidence != "" {
		prompt = fmt.Sprintf("Evidence:\n%s\n\nIs the following claim strictly supported by the evidence? Claim: %s\nOutput ONLY a float between 0.00 and 1.00 indicating your confidence.", evidence, claim)
	} else {
		prompt = fmt.Sprintf("Is the following claim supported by general knowledge? Claim: %s\nOutput ONLY a float between 0.00 and 1.00 indicating your confidence.", claim)
	}

	// Assuming Claude v3 payload structure for Bedrock (which is typical for Bedrock text generation)
	payload := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        10,
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
		return 0, err
	}

	contentType := "application/json"
	accept := "application/json"
	
	output, err := b.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     &b.Model,
		ContentType: &contentType,
		Accept:      &accept,
		Body:        payloadBytes,
	})
	if err != nil {
		return 0, err
	}

	var res struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(output.Body, &res); err != nil {
		return 0, err
	}

	if len(res.Content) == 0 {
		return 0, fmt.Errorf("no content returned from Bedrock")
	}

	textResp := strings.TrimSpace(res.Content[0].Text)
	score, err := strconv.ParseFloat(textResp, 64)
	if err != nil {
		if strings.Contains(strings.ToLower(textResp), "1.00") {
			return 1.0, nil
		}
		if pos := strings.Contains(strings.ToLower(textResp), "0.00"); pos {
			return 0.0, nil
		}
		return 0.5, fmt.Errorf("could not parse valid float from Bedrock Response: %s", textResp)
	}

	return score, nil
}

func (b *BedrockBackend) GetEmbeddings(ctx context.Context, text []string) ([][]float64, error) {
	return nil, fmt.Errorf("GetEmbeddings not implemented natively for bedrock via this interface")
}

func (b *BedrockBackend) ParseAtomicClaims(ctx context.Context, text string) ([]string, error) {
	return strings.Split(text, ". "), nil
}

func (b *BedrockBackend) EvaluateNLI(ctx context.Context, contextText string, claim string) (string, float64, error) {
	return "Neutral", 0.5, fmt.Errorf("EvaluateNLI not implemented for bedrock")
}

