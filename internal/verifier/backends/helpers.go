package backends

import (
	"encoding/json"
	"fmt"
	"strings"
)

// buildEvidenceText constructs a formatted evidence string from span maps.
// Used uniformly across all backends to eliminate duplicated evidence formatting.
func buildEvidenceText(spans []map[string]string) string {
	var sb strings.Builder
	for _, sp := range spans {
		sb.WriteString(fmt.Sprintf("[%s] %s\n", sp["SID"], sp["Text"]))
	}
	return strings.TrimSpace(sb.String())
}

// buildVerifyPrompt constructs the verification prompt for heuristic scoring.
// When enriched=false, asks for a simple float confidence score.
func buildVerifyPrompt(claim string, evidence string, enriched bool) string {
	if enriched {
		return buildEnrichPrompt(claim, evidence)
	}
	if evidence != "" {
		return fmt.Sprintf("Evidence:\n%s\n\nIs the following claim strictly supported by the evidence? Claim: %s\nOutput ONLY a float between 0.00 and 1.00 indicating your confidence.", evidence, claim)
	}
	return fmt.Sprintf("Is the following claim supported by general knowledge? Claim: %s\nOutput ONLY a float between 0.00 and 1.00 indicating your confidence.", claim)
}

// buildEnrichPrompt constructs the enriched verification prompt that returns
// a JSON object with supported, confidence, reason, and corrected fields.
func buildEnrichPrompt(claim string, evidence string) string {
	if evidence != "" {
		return fmt.Sprintf("Evidence:\n%s\n\nIs the following claim strictly supported by the evidence? Claim: %s\nOutput ONLY a valid JSON object with: 1. 'supported' (boolean), 2. 'confidence' (float 0.00 to 1.00), 3. 'reason' (short string tag), 4. 'corrected' (string, corrected claim).", evidence, claim)
	}
	return fmt.Sprintf("Is the following claim supported by general knowledge? Claim: %s\nOutput ONLY a valid JSON object with: 1. 'supported' (boolean), 2. 'confidence' (float 0.00 to 1.00), 3. 'reason' (short string tag), 4. 'corrected' (string, corrected claim).", claim)
}

// buildParseClaimsPrompt constructs the prompt for LLM-based atomic claim parsing.
func buildParseClaimsPrompt(text string) string {
	return fmt.Sprintf("Split the following text into an array of completely atomic, standalone factual claims. Ensure all pronouns are resolved. Output ONLY a valid JSON array of strings, nothing else. Text: %s", text)
}

// stripCodeFences removes markdown code fences from LLM output.
func stripCodeFences(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content)
}

// parseEnrichedJSON parses the enriched verification JSON response from any backend.
func parseEnrichedJSON(raw string) (float64, string, string, error) {
	content := stripCodeFences(raw)

	var enriched struct {
		Supported  bool    `json:"supported"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
		Corrected  string  `json:"corrected"`
	}

	if err := json.Unmarshal([]byte(content), &enriched); err != nil {
		return 0, "", "", fmt.Errorf("JSON unmarshal error: %w", err)
	}

	return enriched.Confidence, enriched.Reason, enriched.Corrected, nil
}

// parseClaimsJSON parses an LLM response expected to contain a JSON array of claim strings.
// Falls back to naive sentence splitting if JSON parsing fails.
func parseClaimsJSON(raw string, originalText string) []string {
	content := stripCodeFences(raw)
	var claims []string
	if err := json.Unmarshal([]byte(content), &claims); err != nil {
		return strings.Split(originalText, ". ")
	}
	return claims
}
