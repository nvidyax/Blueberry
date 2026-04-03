package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/blueberry/mcp/internal/config"
	"github.com/blueberry/mcp/internal/domain"
)

type LocalStore struct {
	mu          sync.Mutex
	runs        map[string]*domain.RunState
	activeRunID string
}

func NewLocalStore() *LocalStore {
	return &LocalStore{
		runs: make(map[string]*domain.RunState),
	}
}

func (s *LocalStore) StartRun(runID string) (*domain.RunState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if runID == "" {
		runID = fmt.Sprintf("R%d", time.Now().UnixNano())
	}
	run := domain.NewRunState(runID)
	run.CreatedAt = float64(time.Now().UnixNano()) / 1e9

	s.runs[runID] = run
	s.activeRunID = runID

	return run, s.persist(run)
}

func (s *LocalStore) GetRun(runID string) (*domain.RunState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := runID
	if id == "" {
		id = s.activeRunID
	}
	if run, ok := s.runs[id]; ok {
		return run, nil
	}
	
	// Load from disk
	p := filepath.Join(config.RunDir(id), "run.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("run not found: %s", id)
	}

	var run domain.RunState
	if err := json.Unmarshal(b, &run); err != nil {
		return nil, fmt.Errorf("failed to parse run: %w", err)
	}
	s.runs[id] = &run
	s.activeRunID = id
	return &run, nil
}

func (s *LocalStore) persist(run *domain.RunState) error {
	p := filepath.Join(config.RunDir(run.RunID), "run.json")
	b, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0644)
}

func (s *LocalStore) AddSpan(run *domain.RunState, text, source string, meta map[string]interface{}) *domain.SpanRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	sid := fmt.Sprintf("S%d", run.NextSpanIdx)
	run.NextSpanIdx++

	rec := domain.SpanRecord{
		SID:       sid,
		Text:      text,
		Source:    source,
		CreatedAt: float64(time.Now().UnixNano()) / 1e9,
		Meta:      meta,
	}

	run.Spans[sid] = rec
	run.SpanOrder = append(run.SpanOrder, sid)
	s.persist(run)
	return &rec
}

func (s *LocalStore) AddAttempt(run *domain.RunState, claimID, hypothesis string, budgetMinutes float64) *domain.AttemptRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	aid := fmt.Sprintf("A%d", run.NextAttemptIdx)
	run.NextAttemptIdx++

	rec := domain.AttemptRecord{
		AttemptID:     aid,
		ClaimID:       claimID,
		Hypothesis:    hypothesis,
		BudgetMinutes: budgetMinutes,
		CreatedAt:     float64(time.Now().UnixNano()) / 1e9,
	}

	run.Attempts = append(run.Attempts, rec)
	s.persist(run)
	return &rec
}
