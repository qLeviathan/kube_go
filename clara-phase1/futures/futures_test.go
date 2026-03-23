package futures

import (
	"testing"

	"github.com/clara-phase1/ar"
	"github.com/clara-phase1/memory"
)

func testRuleSet() ar.RuleSet {
	return ar.RuleSet{
		Name:   "test",
		Domain: "test",
		Rules: []ar.RuleSpec{
			{
				ID:   "r1",
				Head: ar.AtomSpec{Predicate: "treat_A_indicated"},
				Body: []ar.AtomSpec{
					{Predicate: "has_feature", Args: []string{"blood_pressure", "high"}},
					{Predicate: "has_feature", Args: []string{"heart_rate", "high"}},
				},
			},
			{
				ID:   "r2",
				Head: ar.AtomSpec{Predicate: "treat_B_indicated"},
				Body: []ar.AtomSpec{
					{Predicate: "has_feature", Args: []string{"glucose", "high"}},
				},
			},
		},
		Labels: []ar.LabelSpec{
			{Atom: "treat_A_indicated", Label: "treat_A"},
			{Atom: "treat_B_indicated", Label: "treat_B"},
		},
	}
}

func TestNewPredictor(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rs := testRuleSet()
	p := NewPredictor(mem, rs, 5)

	if p == nil {
		t.Fatal("expected non-nil predictor")
	}
	if len(p.RuleIndex) != 2 {
		t.Errorf("expected 2 rules indexed, got %d", len(p.RuleIndex))
	}
	if len(p.Labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(p.Labels))
	}
}

func TestPredictAllFactsSatisfied(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rs := testRuleSet()
	p := NewPredictor(mem, rs, 5)

	// Provide all facts for r1
	facts := []string{
		"has_feature(blood_pressure, high)",
		"has_feature(heart_rate, high)",
	}

	pred := p.Predict(facts)
	if pred.LikelyLabel != "treat_A" {
		t.Errorf("expected treat_A, got %s", pred.LikelyLabel)
	}
	if pred.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", pred.Confidence)
	}
	if pred.CacheHit {
		t.Error("first prediction should not be cache hit")
	}
}

func TestPredictCacheHit(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rs := testRuleSet()
	p := NewPredictor(mem, rs, 5)

	facts := []string{
		"has_feature(glucose, high)",
	}

	p.Predict(facts) // first call
	pred := p.Predict(facts) // should hit cache

	if !pred.CacheHit {
		t.Error("second prediction should be cache hit")
	}
	if pred.LikelyLabel != "treat_B" {
		t.Errorf("expected treat_B, got %s", pred.LikelyLabel)
	}
}

func TestPredictFromMemory(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")

	// Record enough patterns for memory-based prediction
	mem.RecordPattern([]string{"temp"}, "storm", 0.9)
	mem.RecordPattern([]string{"temp"}, "storm", 0.85)
	mem.RecordPattern([]string{"temp"}, "storm", 0.88)

	rs := testRuleSet()
	p := NewPredictor(mem, rs, 5)

	facts := []string{
		"has_feature(temp, high)",
	}

	pred := p.Predict(facts)
	if pred.LikelyLabel != "storm" {
		t.Errorf("expected storm from memory, got %s", pred.LikelyLabel)
	}
}

func TestBuildChain(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rs := testRuleSet()
	p := NewPredictor(mem, rs, 5)

	facts := []string{
		"has_feature(blood_pressure, high)",
		"has_feature(heart_rate, high)",
	}

	chain := p.BuildChain(facts)
	if len(chain.Steps) == 0 {
		t.Error("expected non-empty chain")
	}

	foundR1 := false
	for _, step := range chain.Steps {
		if step.RuleID == "r1" && step.Probability == 1.0 {
			foundR1 = true
		}
	}
	if !foundR1 {
		t.Error("expected r1 in chain with probability 1.0")
	}
}

func TestPartialSatisfaction(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rs := testRuleSet()
	p := NewPredictor(mem, rs, 5)

	// Only provide one of two required facts for r1
	facts := []string{
		"has_feature(blood_pressure, high)",
	}

	chain := p.BuildChain(facts)
	// r1 is 50% satisfied, so it should appear with probability 0.5
	foundPartial := false
	for _, step := range chain.Steps {
		if step.RuleID == "r1" && step.Probability == 0.5 {
			foundPartial = true
		}
	}
	if !foundPartial {
		t.Error("expected r1 with probability 0.5")
	}
}

func TestWarmCache(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	mem.RecordPattern([]string{"blood_pressure"}, "treat_A", 0.8)

	rs := testRuleSet()
	p := NewPredictor(mem, rs, 5)

	warmed := p.WarmCache(10)
	if warmed != 1 {
		t.Errorf("expected 1 warmed, got %d", warmed)
	}
	if len(p.Cache) != 1 {
		t.Errorf("expected 1 cache entry, got %d", len(p.Cache))
	}
}

func TestRecordOutcome(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rs := testRuleSet()
	p := NewPredictor(mem, rs, 5)

	facts := []string{"has_feature(glucose, high)"}
	p.Predict(facts) // prediction: treat_B

	p.RecordOutcome(facts, "treat_B") // correct
	if p.Stats.CorrectPredictions != 1 {
		t.Errorf("expected 1 correct, got %d", p.Stats.CorrectPredictions)
	}
}

func TestClearCache(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rs := testRuleSet()
	p := NewPredictor(mem, rs, 5)

	p.Predict([]string{"has_feature(glucose, high)"})
	if len(p.Cache) == 0 {
		t.Error("cache should not be empty")
	}

	p.ClearCache()
	if len(p.Cache) != 0 {
		t.Error("cache should be empty after clear")
	}
}

func TestSummary(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rs := testRuleSet()
	p := NewPredictor(mem, rs, 5)

	s := p.Summary()
	if s == "" {
		t.Error("expected non-empty summary")
	}
}

func TestExtractFeatureNames(t *testing.T) {
	facts := []string{
		"has_feature(blood_pressure, high)",
		"has_feature(heart_rate, low)",
		"something_else",
	}
	names := extractFeatureNames(facts)
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}
