package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStoreEmpty(t *testing.T) {
	s := NewStore("/tmp/nonexistent_memory.json")
	if s == nil {
		t.Fatal("expected non-nil store")
	}
	if len(s.Facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(s.Facts))
	}
	if len(s.RuleStats) != 0 {
		t.Errorf("expected 0 rule stats, got %d", len(s.RuleStats))
	}
	if s.Summary() == "" {
		t.Error("expected non-empty summary")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test_memory.json")

	// Create and populate store
	s := NewStore(path)
	s.LearnFact("test_key", "test_value", "medical", "test")
	s.RecordRuleFire("rule-1", true, "medical")
	s.RecordRuleFire("rule-1", false, "medical")
	s.RecordPattern([]string{"blood_pressure", "heart_rate"}, "treat_A", 0.85)

	if err := s.Save(); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("memory file not created")
	}

	// Load and verify
	s2 := NewStore(path)
	if len(s2.Facts) != 1 {
		t.Errorf("expected 1 fact, got %d", len(s2.Facts))
	}
	if s2.Facts[0].Key != "test_key" {
		t.Errorf("expected key test_key, got %s", s2.Facts[0].Key)
	}
	if len(s2.RuleStats) != 1 {
		t.Errorf("expected 1 rule stat, got %d", len(s2.RuleStats))
	}
	if s2.RuleStats[0].FireCount != 2 {
		t.Errorf("expected 2 fires, got %d", s2.RuleStats[0].FireCount)
	}
	if s2.RuleStats[0].CorrectCount != 1 {
		t.Errorf("expected 1 correct, got %d", s2.RuleStats[0].CorrectCount)
	}
	if len(s2.Patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(s2.Patterns))
	}
}

func TestRuleFire(t *testing.T) {
	s := NewStore("/tmp/no.json")

	s.RecordRuleFire("r1", true, "medical")
	s.RecordRuleFire("r1", true, "medical")
	s.RecordRuleFire("r1", false, "coa")

	stats := s.GetRuleStats("r1")
	if stats == nil {
		t.Fatal("expected stats for r1")
	}
	if stats.FireCount != 3 {
		t.Errorf("expected 3 fires, got %d", stats.FireCount)
	}
	if stats.CorrectCount != 2 {
		t.Errorf("expected 2 correct, got %d", stats.CorrectCount)
	}
	if len(stats.Domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(stats.Domains))
	}

	// Missing rule
	if s.GetRuleStats("nonexistent") != nil {
		t.Error("expected nil for nonexistent rule")
	}
}

func TestLearnFactDedupe(t *testing.T) {
	s := NewStore("/tmp/no.json")
	s.LearnFact("k", "v", "d", "src")
	s.LearnFact("k", "v", "d", "src")
	s.LearnFact("k", "v", "d", "src")

	if len(s.Facts) != 1 {
		t.Errorf("expected 1 fact (deduped), got %d", len(s.Facts))
	}
	if s.Facts[0].Uses != 3 {
		t.Errorf("expected 3 uses, got %d", s.Facts[0].Uses)
	}
}

func TestPatternRecording(t *testing.T) {
	s := NewStore("/tmp/no.json")
	s.RecordPattern([]string{"b", "a"}, "treat_A", 0.8)
	s.RecordPattern([]string{"a", "b"}, "treat_A", 0.9) // same features, different order
	s.RecordPattern([]string{"c"}, "treat_B", 0.7)

	if len(s.Patterns) != 2 {
		t.Errorf("expected 2 patterns, got %d", len(s.Patterns))
	}

	// First pattern should have frequency 2
	for _, p := range s.Patterns {
		if p.Prediction == "treat_A" && p.Frequency != 2 {
			t.Errorf("expected frequency 2 for treat_A, got %d", p.Frequency)
		}
	}
}

func TestPredictFromHistory(t *testing.T) {
	s := NewStore("/tmp/no.json")

	// Need at least 3 observations to predict
	s.RecordPattern([]string{"a", "b"}, "treat_A", 0.8)
	_, _, found := s.PredictFromHistory([]string{"a", "b"})
	if found {
		t.Error("should not predict with frequency < 3")
	}

	s.RecordPattern([]string{"a", "b"}, "treat_A", 0.9)
	s.RecordPattern([]string{"a", "b"}, "treat_A", 0.85)

	label, conf, found := s.PredictFromHistory([]string{"b", "a"}) // order shouldn't matter
	if !found {
		t.Error("should predict with frequency >= 3")
	}
	if label != "treat_A" {
		t.Errorf("expected treat_A, got %s", label)
	}
	if conf <= 0 {
		t.Error("expected positive confidence")
	}
}

func TestGetTopPatterns(t *testing.T) {
	s := NewStore("/tmp/no.json")
	s.RecordPattern([]string{"a"}, "x", 0.5)
	s.RecordPattern([]string{"b"}, "y", 0.6)
	s.RecordPattern([]string{"b"}, "y", 0.7)
	s.RecordPattern([]string{"b"}, "y", 0.8)

	top := s.GetTopPatterns(1)
	if len(top) != 1 {
		t.Fatalf("expected 1 top pattern, got %d", len(top))
	}
	if top[0].Prediction != "y" {
		t.Errorf("expected top pattern y, got %s", top[0].Prediction)
	}
}

func TestRunHistory(t *testing.T) {
	s := NewStore("/tmp/no.json")
	s.RecordRun(RunSummary{Datasets: 3, Accuracy: 0.85})
	s.RecordRun(RunSummary{Datasets: 5, Accuracy: 0.90})

	if s.TotalRuns() != 2 {
		t.Errorf("expected 2 runs, got %d", s.TotalRuns())
	}
}

func TestAllRuleStats(t *testing.T) {
	s := NewStore("/tmp/no.json")
	s.RecordRuleFire("r1", true, "d1")
	s.RecordRuleFire("r2", false, "d2")

	all := s.AllRuleStats()
	if len(all) != 2 {
		t.Errorf("expected 2 stats, got %d", len(all))
	}
}

func TestQueryFacts(t *testing.T) {
	s := NewStore("/tmp/no.json")
	s.LearnFact("k1", "v1", "medical", "test")
	s.LearnFact("k2", "v2", "coa", "test")

	med := s.QueryFacts("medical")
	if len(med) != 1 {
		t.Errorf("expected 1 medical fact, got %d", len(med))
	}

	all := s.QueryFacts("")
	if len(all) != 2 {
		t.Errorf("expected 2 total facts, got %d", len(all))
	}
}
