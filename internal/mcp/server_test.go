package mcp

import (
	"context"
	"testing"

	"github.com/blueberry/mcp/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestServerWrapper_LoadRunMissing(t *testing.T) {
	appStore := store.NewLocalStore()
	sw := NewServerWrapper(appStore)

	req := mcp.CallToolRequest{}
	req.Params.Name = "load_run"
	req.Params.Arguments = map[string]interface{}{
		"run_id": "MISSING_R123",
	}

	res, err := sw.handleLoadRun(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if res.IsError == false {
		t.Errorf("expected tool to return IsError=true on missing run, got false")
	}
}

func TestServerWrapper_IntegrationWorkflow(t *testing.T) {
	appStore := store.NewLocalStore()
	sw := NewServerWrapper(appStore)

	ctx := context.Background()
	reqStart := mcp.CallToolRequest{}
	reqStart.Params.Name = "start_run"
	reqStart.Params.Arguments = map[string]interface{}{
		"problem_statement": "Hello", 
		"deliverable": "World",
	}
	res, err := sw.handleStartRun(ctx, reqStart)
	if err != nil || res.IsError {
		t.Fatalf("failed start run")
	}

	if len(res.Content) == 0 {
		t.Error("expected response content")
	}
}

func TestServerWrapper_AddAttempt_Integration(t *testing.T) {
	appStore := store.NewLocalStore()
	sw := NewServerWrapper(appStore)

	ctx := context.Background()
	sw.appStore.StartRun("R_TEST")

	req := mcp.CallToolRequest{}
	req.Params.Name = "add_attempt"
	req.Params.Arguments = map[string]interface{}{
		"run_id": "R_TEST",
		"claim_id": "C1",
		"hypothesis": "I think so",
		"budget_minutes": float64(5.0),
	}

	res, err := sw.handleAddAttempt(ctx, req)
	if err != nil || res.IsError {
		t.Fatalf("failed add attempt")
	}

	if len(res.Content) == 0 {
		t.Error("expected response content")
	}
}

func TestServerWrapper_SplitClaims(t *testing.T) {
	appStore := store.NewLocalStore()
	sw := NewServerWrapper(appStore)

	ctx := context.Background()
	req := mcp.CallToolRequest{}
	req.Params.Name = "split_claims"
	req.Params.Arguments = map[string]interface{}{
		"text": "The sky is blue. Grass is green.",
	}

	// This will likely return an error string in result if no API key is set, which is fine to test wiring
	res, _ := sw.handleSplitClaims(ctx, req)

	if res.IsError == false && len(res.Content) == 0 {
		t.Errorf("expected either an error from backend or some content")
	}
}
