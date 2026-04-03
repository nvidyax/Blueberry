package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/blueberry/mcp/internal/store"
	"github.com/blueberry/mcp/internal/verifier"
	"github.com/blueberry/mcp/internal/verifier/backends"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ServerWrapper struct {
	s        *server.MCPServer
	appStore *store.LocalStore
}

func NewServerWrapper(appStore *store.LocalStore) *ServerWrapper {
	s := server.NewMCPServer(
		"blueberry",
		"1.0.0",
	)

	sw := &ServerWrapper{
		s:        s,
		appStore: appStore,
	}

	sw.registerTools()
	return sw
}

func (sw *ServerWrapper) Server() *server.MCPServer {
	return sw.s
}

func (sw *ServerWrapper) registerTools() {
	// start_run
	startRunTool := mcp.NewTool("start_run",
		mcp.WithDescription("Create a new run directory with a problem statement."),
		mcp.WithString("problem_statement", mcp.Required(), mcp.Description("Problem statement")),
		mcp.WithString("deliverable", mcp.Required(), mcp.Description("Deliverable")),
	)
	sw.s.AddTool(startRunTool, sw.handleStartRun)

	// load_run
	loadRunTool := mcp.NewTool("load_run",
		mcp.WithDescription("Resume an existing run."),
		mcp.WithString("run_id", mcp.Required(), mcp.Description("Run ID to load")),
	)
	sw.s.AddTool(loadRunTool, sw.handleLoadRun)

	// add_span
	addSpanTool := mcp.NewTool("add_span",
		mcp.WithDescription("Add evidence from text."),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to add")),
		mcp.WithString("source", mcp.Description("Source of the text")),
		mcp.WithString("run_id", mcp.Description("Run ID")),
	)
	sw.s.AddTool(addSpanTool, sw.handleAddSpan)

	// detect_hallucination
	detectHallucinationTool := mcp.NewTool("detect_hallucination",
		mcp.WithDescription("Information-budget diagnostic per claim."),
		mcp.WithString("answer", mcp.Required(), mcp.Description("Answer to verify")),
	)
	sw.s.AddTool(detectHallucinationTool, sw.handleDetectHallucination)
}

func (sw *ServerWrapper) handleStartRun(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	run, err := sw.appStore.StartRun("")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	b, _ := json.Marshal(map[string]string{
		"run_id": run.RunID,
		"msg": "Run started successfully",
	})
	return mcp.NewToolResultText(string(b)), nil
}

func (sw *ServerWrapper) handleLoadRun(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, ok := req.Params.Arguments["run_id"].(string)
	if !ok {
		return mcp.NewToolResultError("run_id is required"), nil
	}

	run, err := sw.appStore.GetRun(runID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	b, _ := json.Marshal(map[string]string{
		"run_id": run.RunID,
		"status": "loaded",
	})
	return mcp.NewToolResultText(string(b)), nil
}

func (sw *ServerWrapper) handleAddSpan(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	text, _ := req.Params.Arguments["text"].(string)
	source, _ := req.Params.Arguments["source"].(string)
	runID, _ := req.Params.Arguments["run_id"].(string)

	if source == "" {
		source = "manual"
	}

	run, err := sw.appStore.GetRun(runID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rec := sw.appStore.AddSpan(run, text, source, nil)
	
	b, _ := json.Marshal(map[string]any{
		"run_id": run.RunID,
		"sid":    rec.SID,
		"chars":  len(rec.Text),
	})
	return mcp.NewToolResultText(string(b)), nil
}

func (sw *ServerWrapper) handleDetectHallucination(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	answer, _ := req.Params.Arguments["answer"].(string)
	
	backendType := os.Getenv("BERRY_VERIFIER_BACKEND")
	var backend verifier.Backend

	if backendType == "anthropic" {
		backend = backends.NewAnthropicBackend("")
	} else {
		backend = backends.NewOpenAIBackend("")
	}

	steps := []verifier.Step{
		{
			Idx:        0,
			Claim:      answer,
			Cites:      []string{"S0"},
			Confidence: 0.95,
		},
	}

	res, err := backend.Verify(ctx, answer, steps, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("verification failed: %v", err)), nil
	}

	b, _ := json.Marshal(map[string]any{
		"flagged": res[0].Flagged,
		"details": res,
	})

	return mcp.NewToolResultText(string(b)), nil
}
