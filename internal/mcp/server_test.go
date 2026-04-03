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

	req := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
		}{
			Name: "load_run",
			Arguments: map[string]interface{}{
				"run_id": "MISSING_R123",
			},
		},
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
	reqStart := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
		}{
			Name: "start_run",
			Arguments: map[string]interface{}{
				"problem_statement": "Hello", 
				"deliverable": "World",
			},
		},
	}
	res, err := sw.handleStartRun(ctx, reqStart)
	if err != nil || res.IsError {
		t.Fatalf("failed start run")
	}

	if len(res.Content) == 0 {
		t.Error("expected response content")
	}
}
