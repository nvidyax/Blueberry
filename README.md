# Blueberry MCP Server

Blueberry is an evidence-first [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server built in Go. It acts as an inline skill/tool that evaluates context and prompts to determine if there's a way to reduce hallucinations at runtime, pre-generation.

By establishing a robust local storage layer and a structured reasoning workflow, Blueberry evaluates claims against concrete evidence ("Spans") to measure confidence and calculate trackable "information budgets."

## Features

- **MCP Tool Registration**: Out-of-the-box MCP tools spanning full diagnostics limits (`start_run`, `load_run`, `add_span`, `detect_hallucination`, `get_run_status`, `list_spans`, `add_attempt`).
- **Evidence-First Workflow**: Enforces a pattern where AI must construct "runs" and pull context before drawing conclusions.
- **Verification Backends**: Integrates seamlessly with OpenAI, Anthropic, Google Gemini, and AWS Bedrock to securely score confidence and strictly limit drift across generative claims.
- **Local State Persistency**: Keeps a persistent trace by logging runs, spanning evidence, and hypotheses to local JSON storage for inspection and continuity.

## Architecture

The following diagram maps out how Blueberry leverages the MCP structure to properly ground workflows:

```mermaid
graph TD
    Client["MCP Client"] <-->|"stdio"| Server("Blueberry MCP Server")
    
    Server -->|"Read/Write"| Store["Local Store Layer"]
    Store -->|"Persists"| Disk[("Local Disk run.json")]
    
    Server -->|"Validates Evidence"| Verifier["Verifier Engine"]
    Verifier -->|"Anthropic Backend"| Anthropic["Claude API"]
    Verifier -->|"OpenAI Backend"| OpenAI["OpenAI API"]
    Verifier -->|"Gemini Backend"| Gemini["Google Gemini API"]
    Verifier -->|"Bedrock Backend"| Bedrock["AWS Bedrock"]
    
    subgraph Domain State
        Run("Run State")
        Run --> Spans("Spans / Evidence")
        Run --> Attempts("Attempts / Claims")
    end
    
    Store -.->|"Manages"| Domain State
```

## Supported Verification Backends

You can define which foundational model verifies your AI's tracing logs by setting the environment variable `BERRY_VERIFIER_BACKEND`:

1. **OpenAI** (`BERRY_VERIFIER_BACKEND=openai`)
   - Evaluates token bounds via natural logarithmic probabilities logic.
   - Env Defaults: Uses `OPENAI_API_KEY`, default model `gpt-4o-mini`.

2. **Anthropic** (`BERRY_VERIFIER_BACKEND=anthropic`)
   - Employs heuristic constraints for Claude 3 logic grading.
   - Env Defaults: Uses `ANTHROPIC_API_KEY`, default model `claude-3-haiku-20240307`.

3. **Google Gemini** (`BERRY_VERIFIER_BACKEND=gemini`)
   - Uses strict bounding parameters via Google Generative REST framework.
   - Env Defaults: Uses `GEMINI_API_KEY`, default model `gemini-1.5-flash`.

4. **AWS Bedrock** (`BERRY_VERIFIER_BACKEND=bedrock`)
   - Uses AWS SDK v2 orchestration with SigV4 API compliance handling for enterprise clouds.
   - Env Defaults: Relies on localized AWS credentials profile. Default model acts as Claude Haiku wrapper.

## Available Tools

- `start_run`: Create a new diagnostic run directory initialized with an overarching primary problem statement.
- `load_run`: Resume an actively suspended execution trace by passing its ID.
- `get_run_status`: Quickly fetch active tracking diagnostics (budget usages, array counts) without triggering deep text traversals.
- `list_spans`: Print standard text representations of actively populated spans for review by host AI models.
- `add_span`: Add individual fragments of fetched evidence arrays linearly to limit data context windows natively prior to conclusions.
- `add_attempt`: Formulate and queue an LLM conclusion claim into the timeline securely without verifying yet.
- `detect_hallucination`: Pass a specified claim answer against the trace timeline. Extracts spans out of the active store, passes them strictly into the defined `Verifier Engine`, returning hallucination status and specific probability tolerances!

## Getting Started

To run the server instance locally over `stdio`:

```bash
go run ./cmd/blueberry/main.go -transport stdio
```

_Beep Boop_
