---
name: Blueberry Verification Engine
description: Agent Skill to intercept the /blueberry command and orchestrate the hallucination verification pipeline.
---

# Blueberry Verification Engine

When the user types a message starting with `/blueberry evaluate`, followed by an argument or statement, you must strictly follow this workflow:

## 1. Gather Context
Identify the core argument the user wants to evaluate. Review the current agent conversation history to find the relevant context, excerpts, or facts previously established. If no specific context has been established in the active conversation, you will evaluate against general knowledge.

## 2. Execute `evaluate_argument`
Call the `evaluate_argument` MCP tool provided by the Blueberry server with the following parameters:
- `text`: The exact argument the user provided after `/blueberry evaluate`.
- `context_text`: The relevant facts or context you gathered from the conversation (leave empty if none).
- `enrich`: Set to `true` (so the verification engine yields reasoning tags and proposes corrected claims).

## 3. Output the Findings
The `evaluate_argument` tool handles all the heavy lifting—atomization, multi-step verification, and result aggregation. Provide the tool's raw output back to the user exactly as formatted, ensuring you preserve the emojis (`✅`, `❌`), percentage confidence metrics, textual reasonings, and corrected claims. Do not summarize or omit the returned findings.
