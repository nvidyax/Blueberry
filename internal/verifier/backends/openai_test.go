package backends

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blueberry/mcp/internal/verifier"
)

func TestOpenAI_VerifyLogprobs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"choices": [
				{
					"logprobs": {
						"content": [
							{"token": "Yes", "logprob": -0.01}
						]
					}
				}
			]
		}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	backend := NewOpenAIBackend("mock")
	backend.APIKey = "test_key"
	backend.BaseURL = srv.URL

	steps := []verifier.Step{
		{Idx: 0, Claim: "test claim", Cites: []string{"S1"}, Confidence: 0.9},
	}

	res, err := backend.Verify(context.Background(), "answer", steps, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	
	if res[0].ConfidenceScore == 0 {
		t.Errorf("expected nonzero confidence score, got %f", res[0].ConfidenceScore)
	}
}
