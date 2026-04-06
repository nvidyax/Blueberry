# Blueberry Test Suite Documentation

This document catalogs all automated tests in the Blueberry MCP server codebase.

## Running All Tests

```bash
go test ./... -v
```

## Test Inventory

### Package: `internal/mcp` — MCP Server Handlers

| Test Name | File | Description |
|-----------|------|-------------|
| `TestServerWrapper_LoadRunMissing` | [server_test.go](internal/mcp/server_test.go) | Verifies that loading a non-existent run returns `IsError=true`. |
| `TestServerWrapper_IntegrationWorkflow` | [server_test.go](internal/mcp/server_test.go) | Integration test: creates a new run via `start_run` and validates response content. |
| `TestServerWrapper_AddAttempt_Integration` | [server_test.go](internal/mcp/server_test.go) | Integration test: starts a run, adds an attempt via `add_attempt`, and validates response. |
| `TestServerWrapper_SplitClaims` | [server_test.go](internal/mcp/server_test.go) | Validates `split_claims` handler wiring; accepts either error (no API key) or content. |

### Package: `internal/store` — Local Persistence Layer

| Test Name | File | Description |
|-----------|------|-------------|
| `TestLocalStore_StartGet` | [local_store_test.go](internal/store/local_store_test.go) | Validates run creation and retrieval from the in-memory store + disk persistence. |
| `TestLocalStore_Concurrency` | [local_store_test.go](internal/store/local_store_test.go) | Stress test: 100 concurrent goroutines adding spans to verify mutex safety. |
| `TestLocalStore_GarbageJSON` | [local_store_test.go](internal/store/local_store_test.go) | Validates graceful handling when loading a run from a corrupted JSON file. |

### Package: `internal/verifier` — Core Verifier Logic

| Test Name | File | Description |
|-----------|------|-------------|
| `TestCosineDistance` | [similarity_test.go](internal/verifier/similarity_test.go) | Table-driven test covering 5 cases: identical, orthogonal, opposite, mismatched-length, and zero vectors. |

### Package: `internal/verifier/backends` — Backend Implementations

| Test Name | File | Description |
|-----------|------|-------------|
| `TestOpenAI_VerifyLogprobs` | [openai_test.go](internal/verifier/backends/openai_test.go) | Uses httptest mock server returning logprobs. Validates confidence score extraction via `math.Exp(logprob)`. |
| `TestAnthropic_VerifyValid` | [anthropic_test.go](internal/verifier/backends/anthropic_test.go) | Uses httptest mock returning `"0.98"`. Validates score parsing and unflagged status. |
| `TestAnthropic_FallbackGarbage` | [anthropic_test.go](internal/verifier/backends/anthropic_test.go) | Mock returns `"I am a banana"`. Validates that an error is propagated on unparseable response. |
| `TestAzure_VerifyValid` | [azure_test.go](internal/verifier/backends/azure_test.go) | Uses httptest mock with Azure-style URL routing and logprobs. Validates `~0.951` confidence from `logprob=-0.05`. |
| `TestGemini_VerifyValid` | [gemini_test.go](internal/verifier/backends/gemini_test.go) | Uses httptest mock returning Gemini `candidates` format with `"0.98"`. Validates score and unflagged status. |
| `TestGemini_FallbackGarbage` | [gemini_test.go](internal/verifier/backends/gemini_test.go) | Mock returns `"I am a banana"`. Validates graceful fallback to `0.5` confidence and `flagged=true`. |
| `TestVertex_Initialize` | [vertex_test.go](internal/verifier/backends/vertex_test.go) | Validates Vertex backend constructor behavior with and without Google ADC credentials. |

## Test Summary

- **Total tests:** 16
- **Unit tests (mock HTTP):** 7 (OpenAI, Anthropic×2, Azure, Gemini×2, Vertex)
- **Integration tests (real store):** 6 (Server handlers×4, LocalStore×3)  
- **Algorithm tests:** 1 (CosineDistance, 5 sub-cases)
- **All tests pass without API keys** (backends use httptest mock servers)

## End-to-End MCP Test

An E2E test script (`run_eval.js`) is provided for manual validation against a live API:

```bash
# Writes claims to claims.txt, then runs:
node run_eval.js
```

This spawns the Go server over stdio, sends an `initialize` → `notifications/initialized` → `tools/call(evaluate_argument)` sequence, and prints the verification result. Requires a valid API key in the environment or `mcp_config.json`.
