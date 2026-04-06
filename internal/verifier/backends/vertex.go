package backends

// Supported Google Vertex AI Models (as of April 2026):
//   - gemini-3.1-flash      (default — fast, cost-efficient)
//   - gemini-3.1-flash-lite (cheapest, highest throughput)
//   - gemini-3.1-pro        (most capable, complex reasoning)

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/genai"
	"github.com/blueberry/mcp/internal/verifier"
)

type VertexBackend struct {
	Model     string
	ProjectID string
	Location  string
	client    *genai.Client
}

func NewVertexBackend(model string) (*VertexBackend, error) {
	if model == "" {
		model = "gemini-3.1-flash"
	}

	projectID := os.Getenv("VERTEX_PROJECT_ID")
	location := os.Getenv("VERTEX_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex client: %v", err)
	}

	return &VertexBackend{
		Model:     model,
		ProjectID: projectID,
		Location:  location,
		client:    client,
	}, nil
}

func (v *VertexBackend) Name() string {
	return "vertex"
}

// callVertex is the shared GenerateContent call for all Vertex requests.
func (v *VertexBackend) callVertex(ctx context.Context, prompt string) (string, error) {
	var temp float32 = 0.0
	config := &genai.GenerateContentConfig{
		Temperature: &temp,
	}

	resp, err := v.client.Models.GenerateContent(ctx, v.Model, genai.Text(prompt), config)
	if err != nil {
		return "", err
	}

	textResp := resp.Text()
	if textResp == "" {
		return "", fmt.Errorf("no valid text returned from Vertex")
	}

	return strings.TrimSpace(textResp), nil
}

func (v *VertexBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if v.ProjectID == "" {
		return nil, fmt.Errorf("VERTEX_PROJECT_ID is not set")
	}

	evidence := buildEvidenceText(spans)
	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		var conf float64
		var reason, corrected string
		var err error

		if st.Enrich {
			prompt := buildEnrichPrompt(st.Claim, evidence)
			text, callErr := v.callVertex(ctx, prompt)
			if callErr != nil {
				return nil, fmt.Errorf("vertex error on step %d: %w", i, callErr)
			}
			conf, reason, corrected, err = parseEnrichedJSON(text)
			if err != nil {
				return nil, fmt.Errorf("vertex enrich parse error on step %d: %w", i, err)
			}
		} else {
			prompt := buildVerifyPrompt(st.Claim, evidence, false)
			text, callErr := v.callVertex(ctx, prompt)
			if callErr != nil {
				return nil, fmt.Errorf("vertex error on step %d: %w", i, callErr)
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

func (v *VertexBackend) GetEmbeddings(ctx context.Context, text []string) ([][]float64, error) {
	return nil, fmt.Errorf("GetEmbeddings not implemented natively for vertex via this interface")
}

func (v *VertexBackend) ParseAtomicClaims(ctx context.Context, text string) ([]string, error) {
	if v.ProjectID == "" {
		return strings.Split(text, ". "), nil
	}

	prompt := buildParseClaimsPrompt(text)
	content, err := v.callVertex(ctx, prompt)
	if err != nil {
		return strings.Split(text, ". "), nil
	}

	return parseClaimsJSON(content, text), nil
}

func (v *VertexBackend) EvaluateNLI(ctx context.Context, contextText string, claim string) (string, float64, error) {
	return "Neutral", 0.5, fmt.Errorf("EvaluateNLI not implemented for vertex")
}
