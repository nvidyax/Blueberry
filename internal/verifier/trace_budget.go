package verifier

import (
	"context"
)

type Step struct {
	Idx        int      `json:"idx"`
	Claim      string   `json:"claim"`
	Cites      []string `json:"cites"`
	Confidence float64  `json:"confidence"`
}

type TraceResult struct {
	Idx              int      `json:"idx"`
	Claim            string   `json:"claim"`
	Cites            []string `json:"cites"`
	Target           float64  `json:"target"`
	Flagged          bool     `json:"flagged"`
	RequiredBitsMin  float64  `json:"required_bits_min"`
	RequiredBitsMax  float64  `json:"required_bits_max"`
	ObservedBitsMin  float64  `json:"observed_bits_min"`
	ObservedBitsMax  float64  `json:"observed_bits_max"`
	BudgetGapMin     float64  `json:"budget_gap_min"`
	BudgetGapMax     float64  `json:"budget_gap_max"`
	ConfidenceScore  float64  `json:"confidence_score"`
}

type Backend interface {
	Name() string
	// Verify measures the evidence budget or confidence of steps
	Verify(ctx context.Context, answer string, steps []Step, spans []map[string]string) ([]TraceResult, error)
}
