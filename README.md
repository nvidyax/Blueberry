# Blueberry MCP Server

Blueberry is an evidence-first [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server built in Go. It acts as an inline skill/tool that evaluates context and prompts to determine if there's a way to reduce hallucinations at runtime, pre-generation.

By establishing a robust local storage layer and a structured reasoning workflow, Blueberry evaluates claims against concrete evidence ("Spans") to measure confidence and calculate trackable "information budgets."

## Features

- **MCP Tool Registration**: Out-of-the-box MCP tools including `start_run`, `load_run`, `add_span`, and `detect_hallucination`.
- **Evidence-First Workflow**: Enforces a pattern where AI must construct "runs" and pull context before drawing conclusions.
- **Verification Backends**: Integrates seamlessly with OpenAI and Anthropic natively to score confidence and detect drift or hallucinations across claims.
- **Local State Persistency**: Keeps a persistent trace by logging runs, spanning evidence, and hypotheses to local JSON storage for inspection and continuity.

## Architecture

The following diagram maps out how Blueberry leverages the MCP structure to prevent hallucinations:

```mermaid
graph TD
    Client[MCP Client] <-->|stdio| Server(Blueberry MCP Server)
    
    Server -->|Read/Write| Store[Local Store Layer]
    Store -->|Persists| Disk[(Local Disk run.json)]
    
    Server -->|Validates Evidence| Verifier[Verifier Engine]
    Verifier -->|Anthropic Backend| Anthropic[Claude API]
    Verifier -->|OpenAI Backend| OpenAI[OpenAI API]
    
    subgraph Domain State
        Run(Run State)
        Run --> Spans(Spans / Evidence)
        Run --> Attempts(Attempts / Claims)
    end
    
    Store -.->|Manages| Domain State
```

## Available Tools

- `start_run`: Create a new diagnostic run using a problem statement.
- `load_run`: Load and resume an existing run.
- `add_span`: Inject evidence items ("spans") into the current trace context.
- `detect_hallucination`: Run an information-budget diagnostic against a specific claim/answer, returning flagged status and budget gap metrics.

## Getting Started

To run the server instance locally over `stdio`:

```bash
go run ./cmd/blueberry/main.go -transport stdio
```

_Beep Boop_
