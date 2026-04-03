package backends

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blueberry/mcp/internal/verifier"
)

func TestAzure_VerifyValid(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/openai/deployments/mock-model/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"content": "Yes"
					},
					"logprobs": {
						"content": [
							{"token": "Yes", "logprob": -0.05}
						]
					}
				}
			]
		}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	backend := NewAzureBackend("mock-model")
	backend.APIKey = "test_key"
	backend.Endpoint = srv.URL
	backend.APIVersion = "2024-02-15-preview"

	steps := []verifier.Step{
		{Idx: 0, Claim: "test claim", Cites: []string{"S1"}, Confidence: 0.90},
	}

	res, err := backend.Verify(context.Background(), "answer", steps, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}

	// math.Exp(-0.05) is ~0.951
	if res[0].ConfidenceScore < 0.95 {
		t.Errorf("expected score ~0.951, got %f", res[0].ConfidenceScore)
	}

	if res[0].Flagged {
		t.Error("expected unflagged")
	}
}
