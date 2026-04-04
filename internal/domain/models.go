package domain

// RunState represents the state of a run.
type RunState struct {
	RunID          string                `json:"run_id"`
	CreatedAt      float64               `json:"created_at"`
	DeliverableSID string                `json:"deliverable_sid,omitempty"`
	NextSpanIdx    int                   `json:"next_span_idx"`
	NextAttemptIdx int                   `json:"next_attempt_idx"`
	SpanOrder      []string              `json:"span_order"`
	Spans          map[string]SpanRecord `json:"spans"`
	Attempts       []AttemptRecord       `json:"attempts"`
}

func NewRunState(runID string) *RunState {
	return &RunState{
		RunID:    runID,
		Spans:    make(map[string]SpanRecord),
		Attempts: make([]AttemptRecord, 0),
	}
}

type SpanRecord struct {
	SID       string                 `json:"sid"`
	Text      string                 `json:"text"`
	Source    string                 `json:"source"`
	CreatedAt float64                `json:"created_at"`
	Meta      map[string]interface{} `json:"meta"`
}

type AttemptRecord struct {
	AttemptID     string     `json:"attempt_id"`
	CreatedAt     float64    `json:"created_at"`
	ClaimID       string     `json:"claim_id"`
	Hypothesis    string     `json:"hypothesis"`
	AtomicClaims  []string   `json:"atomic_claims,omitempty"`
	Action        string     `json:"action"`
	BudgetMinutes float64    `json:"budget_minutes"`
	InputSIDs     []string   `json:"input_sids"`
	OutputSIDs    []string   `json:"output_sids"`
	AuditStatus   string     `json:"audit_status"`
	Decision      string     `json:"decision"`
	NextStep      string     `json:"next_step"`
	RAGTriad      *RAGTriad  `json:"rag_triad,omitempty"`
}

type RAGTriad struct {
	ContextRelevance float64 `json:"context_relevance"`
	Faithfulness     float64 `json:"faithfulness"`
	AnswerRelevance  float64 `json:"answer_relevance"`
}
