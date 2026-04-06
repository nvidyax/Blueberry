package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

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
		"2.0.0",
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

// resolveBackend creates the appropriate verifier backend based on the
// BLUEBERRY_VERIFIER_BACKEND environment variable. This replaces the previously
// duplicated if/else chains that were copy-pasted across three handlers.
func resolveBackend() (verifier.Backend, error) {
	backendType := os.Getenv("BLUEBERRY_VERIFIER_BACKEND")
	// Backwards compatibility: also check the legacy env var
	if backendType == "" {
		backendType = os.Getenv("BERRY_VERIFIER_BACKEND")
	}

	switch backendType {
	case "anthropic":
		return backends.NewAnthropicBackend(""), nil
	case "gemini":
		return backends.NewGeminiBackend(""), nil
	case "bedrock":
		return backends.NewBedrockBackend("")
	case "azure":
		return backends.NewAzureBackend(""), nil
	case "vertex":
		return backends.NewVertexBackend("")
	default:
		return backends.NewOpenAIBackend(""), nil
	}
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
		mcp.WithDescription("Information-budget diagnostic per claim. *NOTICE: 'self_consistency' mode invokes the AI multiple times, increasing API token costs!*"),
		mcp.WithString("answer", mcp.Required(), mcp.Description("Answer to verify")),
		mcp.WithString("run_id", mcp.Description("Run ID to verify against context")),
		mcp.WithBoolean("use_nli", mcp.Description("Fast path via NLI Entailment logic")),
		mcp.WithBoolean("use_similarity", mcp.Description("Fast path checking cosine distance first")),
		mcp.WithBoolean("self_consistency", mcp.Description("Run LLM validation 3x to measure variance. WARNING: 3x Token Cost!")),
	)
	sw.s.AddTool(detectHallucinationTool, sw.handleDetectHallucination)

	// get_run_status
	getRunStatusTool := mcp.NewTool("get_run_status",
		mcp.WithDescription("Get the current state of a run."),
		mcp.WithString("run_id", mcp.Description("Run ID")),
	)
	sw.s.AddTool(getRunStatusTool, sw.handleGetRunStatus)

	// list_spans
	listSpansTool := mcp.NewTool("list_spans",
		mcp.WithDescription("List all evidence spans for a run."),
		mcp.WithString("run_id", mcp.Description("Run ID")),
	)
	sw.s.AddTool(listSpansTool, sw.handleListSpans)

	// add_attempt
	addAttemptTool := mcp.NewTool("add_attempt",
		mcp.WithDescription("Log an attempt or hypothesis to a run."),
		mcp.WithString("run_id", mcp.Description("Run ID")),
		mcp.WithString("claim_id", mcp.Required(), mcp.Description("Claim ID")),
		mcp.WithString("hypothesis", mcp.Required(), mcp.Description("Hypothesis text")),
		mcp.WithNumber("budget_minutes", mcp.Description("Budget minutes")),
	)
	sw.s.AddTool(addAttemptTool, sw.handleAddAttempt)

	// split_claims
	splitClaimsTool := mcp.NewTool("split_claims",
		mcp.WithDescription("Atomize a large response into individual factual claims for precise verification."),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text to split into claims")),
	)
	sw.s.AddTool(splitClaimsTool, sw.handleSplitClaims)

	// evaluate_argument
	evaluateArgumentTool := mcp.NewTool("evaluate_argument",
		mcp.WithDescription("Orchestrates parsing and verification across an entire argument. Formats and returns enriched validation results explicitly for the IDE Agent."),
		mcp.WithString("text", mcp.Required(), mcp.Description("The complete argument text to evaluate")),
		mcp.WithBoolean("enrich", mcp.Description("Whether to flag reasonings and generate corrected claims for failed verifications (uses more tokens)")),
		mcp.WithString("context_text", mcp.Description("Optional contextual evidence to verify against. If empty, evaluates against general knowledge.")),
	)
	sw.s.AddTool(evaluateArgumentTool, sw.handleEvaluateArgument)
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
	runID, _ := req.Params.Arguments["run_id"].(string)
	
	backend, err := resolveBackend()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to init backend: %v", err)), nil
	}

	steps := []verifier.Step{
		{
			Idx:        0,
			Claim:      answer,
			Cites:      []string{"S0"},
			Confidence: 0.95,
		},
	}

	var spans []map[string]string
	run, err := sw.appStore.GetRun(runID)
	if err == nil {
		for _, sid := range run.SpanOrder {
			if spanRec, ok := run.Spans[sid]; ok {
				spans = append(spans, map[string]string{
					"SID":  spanRec.SID,
					"Text": spanRec.Text,
				})
			}
		}
	} else if runID != "" {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get run: %v", err)), nil
	}

	useNLI, _ := req.Params.Arguments["use_nli"].(bool)
	useSimilarity, _ := req.Params.Arguments["use_similarity"].(bool)
	selfConsistency, _ := req.Params.Arguments["self_consistency"].(bool)

	var spanTexts []string
	for _, sp := range spans { spanTexts = append(spanTexts, sp["Text"]) }

	if useSimilarity && len(spanTexts) > 0 {
		ansEmb, err := backend.GetEmbeddings(ctx, []string{answer})
		if err == nil && len(ansEmb) > 0 {
			spanEmb, err := backend.GetEmbeddings(ctx, []string{strings.Join(spanTexts, " ")})
			if err == nil && len(spanEmb) > 0 {
				dist := verifier.CosineDistance(ansEmb[0], spanEmb[0])
				if dist < 0.3 {
					b, _ := json.Marshal(map[string]any{"flagged": true, "reason": "semantic distance too low", "distance": dist})
					return mcp.NewToolResultText(string(b)), nil
				}
			}
		}
	}

	if useNLI {
		nliRes, prob, err := backend.EvaluateNLI(ctx, strings.Join(spanTexts, " "), answer)
		if err == nil {
			b, _ := json.Marshal(map[string]any{"flagged": nliRes != "Entailment", "nli_result": nliRes, "nli_confidence": prob})
			return mcp.NewToolResultText(string(b)), nil
		}
	}

	var res []verifier.TraceResult
	if selfConsistency {
		// Run 3 times and average
		var totalConf float64
		flagCount := 0
		for i := 0; i < 3; i++ {
			r, e := backend.Verify(ctx, answer, steps, spans)
			if e == nil && len(r) > 0 {
				totalConf += r[0].ConfidenceScore
				if r[0].Flagged { flagCount++ }
				res = r // keep the last one as base structure
			}
		}
		if len(res) > 0 {
			res[0].ConfidenceScore = totalConf / 3.0
			res[0].Flagged = flagCount >= 2 // majority vote
		} else {
			res, err = backend.Verify(ctx, answer, steps, spans)
		}
	} else {
		res, err = backend.Verify(ctx, answer, steps, spans)
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("verification failed: %v", err)), nil
	}

	b, _ := json.Marshal(map[string]any{
		"flagged": res[0].Flagged,
		"details": res,
	})

	return mcp.NewToolResultText(string(b)), nil
}

func (sw *ServerWrapper) handleGetRunStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, _ := req.Params.Arguments["run_id"].(string)
	run, err := sw.appStore.GetRun(runID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	b, _ := json.Marshal(map[string]any{
		"run_id":           run.RunID,
		"created_at":       run.CreatedAt,
		"next_span_idx":    run.NextSpanIdx,
		"next_attempt_idx": run.NextAttemptIdx,
		"span_count":       len(run.SpanOrder),
		"attempt_count":    len(run.Attempts),
	})
	return mcp.NewToolResultText(string(b)), nil
}

func (sw *ServerWrapper) handleListSpans(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, _ := req.Params.Arguments["run_id"].(string)
	run, err := sw.appStore.GetRun(runID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	spans := make([]map[string]interface{}, 0)
	for _, sid := range run.SpanOrder {
		if sp, ok := run.Spans[sid]; ok {
			spans = append(spans, map[string]interface{}{
				"sid":    sp.SID,
				"source": sp.Source,
				"chars":  len(sp.Text),
				"text":   sp.Text,
			})
		}
	}

	b, _ := json.Marshal(spans)
	return mcp.NewToolResultText(string(b)), nil
}

func (sw *ServerWrapper) handleAddAttempt(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, _ := req.Params.Arguments["run_id"].(string)
	claimID, _ := req.Params.Arguments["claim_id"].(string)
	hypothesis, _ := req.Params.Arguments["hypothesis"].(string)
	budgetVal, ok := req.Params.Arguments["budget_minutes"].(float64)
	if !ok {
		budgetVal = 10.0 // Default 10 mins
	}

	run, err := sw.appStore.GetRun(runID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	att := sw.appStore.AddAttempt(run, claimID, hypothesis, budgetVal)
	b, _ := json.Marshal(att)
	return mcp.NewToolResultText(string(b)), nil
}

func (sw *ServerWrapper) handleSplitClaims(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	text, _ := req.Params.Arguments["text"].(string)

	backend, err := resolveBackend()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to init backend: %v", err)), nil
	}

	claims, err := backend.ParseAtomicClaims(ctx, text)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("parse failed: %v", err)), nil
	}

	b, _ := json.Marshal(map[string]any{
		"claims": claims,
	})
	return mcp.NewToolResultText(string(b)), nil
}

func (sw *ServerWrapper) handleEvaluateArgument(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	text, _ := req.Params.Arguments["text"].(string)
	contextText, _ := req.Params.Arguments["context_text"].(string)
	enrich, ok := req.Params.Arguments["enrich"].(bool)
	if !ok {
		enrich = false
	}

	backend, err := resolveBackend()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to init backend: %v", err)), nil
	}

	claims, err := backend.ParseAtomicClaims(ctx, text)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse claims: %v", err)), nil
	}

	steps := make([]verifier.Step, len(claims))
	for i, claim := range claims {
		steps[i] = verifier.Step{
			Idx:        i,
			Claim:      claim,
			Cites:      []string{},
			Confidence: 0.95,
			Enrich:     enrich,
		}
	}

	spans := make([]map[string]string, 0)
	if contextText != "" {
		spans = append(spans, map[string]string{"SID": "CTX", "Text": contextText})
	}

	results, err := backend.Verify(ctx, text, steps, spans)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("verification failed: %v", err)), nil
	}

	var sb strings.Builder
	for i, res := range results {
		if res.Flagged {
			confStr := fmt.Sprintf("%.1f%%", res.ConfidenceScore*100)
			reasonStr := ""
			if res.Reason != "" {
				reasonStr = fmt.Sprintf(" — \"%s\"", res.Reason)
			}
			sb.WriteString(fmt.Sprintf("Claim %d: ❌ Flagged (%s)%s\n", i+1, confStr, reasonStr))
			if res.CorrectedClaim != "" {
				sb.WriteString(fmt.Sprintf("  ↳ *Correction: %s*\n", res.CorrectedClaim))
			}
			sb.WriteString("\n")
		} else {
			confStr := fmt.Sprintf("%.1f%%", res.ConfidenceScore*100)
			sb.WriteString(fmt.Sprintf("Claim %d: ✅ Verified (%s confidence)\n\n", i+1, confStr))
		}
	}

	return mcp.NewToolResultText(sb.String()), nil
}
