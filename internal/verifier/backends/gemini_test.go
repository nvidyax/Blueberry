package backends

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/blueberry/mcp/internal/verifier"
)

func TestGemini_VerifyValid(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [
				{
					"content": {
						"parts": [
							{"text": "0.98"}
						]
					}
				}
			]
		}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	backend := NewGeminiBackend("mock-model")
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

	if res[0].ConfidenceScore != 0.98 {
		t.Errorf("expected score 0.98, got %f", res[0].ConfidenceScore)
	}

	if res[0].Flagged {
		t.Error("expected unflagged")
	}
}

func TestGemini_FallbackGarbage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"candidates": [
				{
					"content": {
						"parts": [
							{"text": "I am a banana"}
						]
					}
				}
			]
		}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	backend := NewGeminiBackend("mock")
	backend.APIKey = "test"
	backend.BaseURL = srv.URL

	steps := []verifier.Step{
		{Idx: 0, Claim: "test", Cites: []string{"S1"}, Confidence: 0.9},
	}

	_, err := backend.Verify(context.Background(), "answer", steps, nil)
	if err == nil {
		t.Fatal("expected error parsing fallback garbage string")
	}
}
