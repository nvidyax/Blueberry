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
    Client[MCP Client] <-->|stdio| Server(Blueberry MCP Server)
    
    Server -->|Read/Write| Store[Local Store Layer]
    Store -->|Persists| Disk[(Local Disk run.json)]
    
    Server -->|Validates Evidence| Verifier[Verifier Engine]
    Verifier -->|Anthropic Backend| Anthropic[Claude API]
    Verifier -->|OpenAI Backend| OpenAI[OpenAI API]
    Verifier -->|Gemini Backend| Gemini[Google Gemini API]
    Verifier -->|Bedrock Backend| Bedrock[AWS Bedrock]
    Verifier -->|Azure Backend| Azure[Azure OpenAI API]
    Verifier -->|Vertex Backend| Vertex[Google Vertex AI]
    
    subgraph Domain State
        Run(Run State)
        Run --> Spans(Spans / Evidence)
        Run --> Attempts(Attempts / Claims)
    end
    
    Store -.->|Manages| Domain State
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

5. **Azure OpenAI** (`BERRY_VERIFIER_BACKEND=azure`)
   - Fully mimics OpenAI logic grading utilizing dedicated enterprise endpoints.
   - Env Defaults: `AZURE_OPENAI_ENDPOINT`, `AZURE_OPENAI_API_KEY`, default model `gpt-4`.

6. **Google Vertex AI** (`BERRY_VERIFIER_BACKEND=vertex`)
   - Enterprise equivalent to Google Gemini utilizing Google Cloud's official `genai` SDK and service-account OAuth handshakes.
   - Env Defaults: `VERTEX_PROJECT_ID`, default location `us-central1`, default model `gemini-1.5-flash-001`.

## Available Tools

- `start_run`: Create a new diagnostic run directory initialized with an overarching primary problem statement.
- `load_run`: Resume an actively suspended execution trace by passing its ID.
- `get_run_status`: Quickly fetch active tracking diagnostics (budget usages, array counts) without triggering deep text traversals.
- `list_spans`: Print standard text representations of actively populated spans for review by host AI models.
- `add_span`: Add individual fragments of fetched evidence arrays linearly to limit data context windows natively prior to conclusions.
- `add_attempt`: Formulate and queue an LLM conclusion claim into the timeline securely without verifying yet.
- `detect_hallucination`: Pass a specified claim answer against the trace timeline. Extracts spans out of the active store, passes them strictly into the defined `Verifier Engine`, returning hallucination status and specific probability tolerances!

## Installation & Usage (Universal IDE Support)

Blueberry is an open-standard MCP server. You can install it on **Cursor, VS Code, Zed, Antigravity**, or any other MCP-compatible IDE without needing language-specific plugins.

### 1. Download the Executable
Download the pre-compiled binary for your operating system (Windows, macOS, Linux) from the [GitHub Releases](https://github.com/vidyabodepudi/Blueberry/releases) page.

### 2. Configure Your IDE
Open your IDE's MCP settings (e.g., `mcp.json` in VS Code/Cursor conventions) and add Blueberry. You can specify your preferred LLM provider here using `BERRY_VERIFIER_BACKEND` (`openai`, `anthropic`, `gemini`, `azure`, `bedrock`, or `vertex`) and passing the respective API key:

```json
{
  "mcpServers": {
    "blueberry": {
      "command": "/absolute/path/to/downloaded/blueberry",
      "args": ["-transport", "stdio"],
      "env": {
        "BERRY_VERIFIER_BACKEND": "openai",
        "OPENAI_API_KEY": "your-api-key"
      }
    }
  }
}
```

### 3. Install the Agent Skill (Optional)
To enable the elegant `/blueberry evaluate` slash command formatting directly in your AI chat view, copy the contents of [`.agents/skills/blueberry.md`](.agents/skills/blueberry.md) into your IDE's Custom Instructions, System Rules, or standard agent skills directory.

### Quick Local Dev
To run the server instance locally from source over `stdio`:

```bash
go run ./cmd/blueberry/main.go -transport stdio
```
### Ode to the Admiral
*Blueberry is named after my friends child. In a world that is constantly changing and evolving at the speed of thought, we need a way to verify and build trust in the inputs we as hoomans receive to interact with the world. Much like as a growing tiny baby needs their parents to show them the way.*
