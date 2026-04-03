package backends

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"cloud.google.com/go/vertexai/genai"
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
		model = "gemini-1.5-flash-001"
	}

	projectID := os.Getenv("VERTEX_PROJECT_ID")
	location := os.Getenv("VERTEX_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	client, err := genai.NewClient(context.Background(), projectID, location)
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

func (v *VertexBackend) Verify(ctx context.Context, answer string, steps []verifier.Step, spans []map[string]string) ([]verifier.TraceResult, error) {
	if v.ProjectID == "" {
		return nil, fmt.Errorf("VERTEX_PROJECT_ID is not set")
	}

	var evidenceText strings.Builder
	for _, sp := range spans {
		evidenceText.WriteString(fmt.Sprintf("[%s] %s\n", sp["SID"], sp["Text"]))
	}
	evidence := strings.TrimSpace(evidenceText.String())

	results := make([]verifier.TraceResult, len(steps))

	for i, st := range steps {
		conf, err := v.callVertexHeuristic(ctx, st.Claim, evidence)
		if err != nil {
			return nil, fmt.Errorf("vertex error on step %d: %w", i, err)
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

func (v *VertexBackend) callVertexHeuristic(ctx context.Context, claim string, evidence string) (float64, error) {
	var prompt string
	if evidence != "" {
		prompt = fmt.Sprintf("Evidence:\n%s\n\nIs the following claim strictly supported by the evidence? Claim: %s\nOutput ONLY a float between 0.00 and 1.00 indicating your confidence.", evidence, claim)
	} else {
		prompt = fmt.Sprintf("Is the following claim supported by general knowledge? Claim: %s\nOutput ONLY a float between 0.00 and 1.00 indicating your confidence.", claim)
	}

	model := v.client.GenerativeModel(v.Model)
	
	temp := float32(0.0)
	model.SetTemperature(temp)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return 0, err
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return 0, fmt.Errorf("no content returned from Vertex")
	}

	part := resp.Candidates[0].Content.Parts[0]
	textResp := ""
	switch p := part.(type) {
	case genai.Text:
		textResp = string(p)
	default:
		textResp = fmt.Sprintf("%v", p)
	}

	textResp = strings.TrimSpace(textResp)

	score, err := strconv.ParseFloat(textResp, 64)
	if err != nil {
		if strings.Contains(strings.ToLower(textResp), "1.00") {
			return 1.0, nil
		}
		if pos := strings.Contains(strings.ToLower(textResp), "0.00"); pos {
			return 0.0, nil
		}
		return 0.5, fmt.Errorf("could not parse valid float from Vertex Response: %s", textResp)
	}

	return score, nil
}
