package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/blueberry/mcp/internal/config"
)

func TestLocalStore_Concurrency(t *testing.T) {
	s := NewLocalStore()
	run, err := s.StartRun("R_concurrent")
	if err != nil {
		t.Fatalf("failed to start run: %v", err)
	}

	var wg sync.WaitGroup
	workers := 100
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.AddSpan(run, "concurrent text", "manual", nil)
		}(i)
	}

	wg.Wait()

	if len(run.Spans) != workers {
		t.Errorf("expected %d spans, got %d", workers, len(run.Spans))
	}
}

func TestLocalStore_StartGet(t *testing.T) {
	s := NewLocalStore()
	run1, err := s.StartRun("R123")
	if err != nil {
		t.Fatalf("StartRun failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	s2 := NewLocalStore()
	run2, err := s2.GetRun("R123")
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if run1.RunID != run2.RunID {
		t.Errorf("expected %s, got %s", run1.RunID, run2.RunID)
	}
}

func TestLocalStore_GarbageJSON(t *testing.T) {
	s := NewLocalStore()
	id := "R_GARBAGE"

	d := config.RunDir(id)
	os.RemoveAll(d)
	_ = os.MkdirAll(d, 0755)
	
	p := filepath.Join(d, "run.json")
	_ = os.WriteFile(p, []byte("}{invalid json}"), 0644)

	_, err := s.GetRun(id)
	if err == nil {
		t.Error("expected error loading garbage json, got nil")
	}
}
