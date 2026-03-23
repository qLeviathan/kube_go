package rewriter

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
			{ID: "r1", Head: ar.AtomSpec{Predicate: "a"}, Body: []ar.AtomSpec{{Predicate: "b"}}},
			{ID: "r2", Head: ar.AtomSpec{Predicate: "c"}, Body: []ar.AtomSpec{{Predicate: "d"}}},
			{ID: "r3", Head: ar.AtomSpec{Predicate: "e"}, Body: []ar.AtomSpec{{Predicate: "f"}}},
		},
		Labels: []ar.LabelSpec{
			{Atom: "a", Label: "label_a"},
			{Atom: "c", Label: "label_c"},
		},
	}
}

func TestNewRewriter(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rw := NewRewriter(mem, 5)

	if rw == nil {
		t.Fatal("expected non-nil rewriter")
	}
	if len(rw.MetaRules) != 4 {
		t.Errorf("expected 4 default meta-rules, got %d", len(rw.MetaRules))
	}
	if rw.Summary() == "" {
		t.Error("expected non-empty summary")
	}
}

func TestEvaluateNoStats(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rw := NewRewriter(mem, 5)
	rs := testRuleSet()

	report := rw.Evaluate(rs)
	if report.Evaluated != 3 {
		t.Errorf("expected 3 evaluated, got %d", report.Evaluated)
	}
	// No stats recorded, so no actions
	if len(report.Actions) != 0 {
		t.Errorf("expected 0 actions with no stats, got %d", len(report.Actions))
	}
}

func TestEvaluateRetireUnderperformer(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")

	// Record poor performance for r1
	for i := 0; i < 10; i++ {
		mem.RecordRuleFire("r1", false, "test")
	}

	rw := NewRewriter(mem, 5)
	rs := testRuleSet()

	report := rw.Evaluate(rs)
	if report.Retired != 1 {
		t.Errorf("expected 1 retired, got %d", report.Retired)
	}

	found := false
	for _, a := range report.Actions {
		if a.Type == "retire" && a.RuleID == "r1" {
			found = true
		}
	}
	if !found {
		t.Error("expected retire action for r1")
	}
}

func TestEvaluateStrengthenPerformer(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")

	// Record great performance for r2
	for i := 0; i < 10; i++ {
		mem.RecordRuleFire("r2", true, "test")
	}

	rw := NewRewriter(mem, 5)
	rs := testRuleSet()

	report := rw.Evaluate(rs)
	if report.Strengthened != 1 {
		t.Errorf("expected 1 strengthened, got %d", report.Strengthened)
	}
}

func TestEvaluateSplitAmbiguous(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")

	// Record ambiguous performance across domains
	for i := 0; i < 5; i++ {
		mem.RecordRuleFire("r3", true, "medical")
	}
	for i := 0; i < 5; i++ {
		mem.RecordRuleFire("r3", false, "coa")
	}

	rw := NewRewriter(mem, 5)
	rs := testRuleSet()

	report := rw.Evaluate(rs)
	if report.Split != 1 {
		t.Errorf("expected 1 split, got %d", report.Split)
	}
}

func TestApplyRetire(t *testing.T) {
	rs := testRuleSet()
	report := RewriteReport{
		Actions: []RewriteAction{
			{Type: "retire", RuleID: "r2"},
		},
	}

	mem := memory.NewStore("/tmp/no.json")
	rw := NewRewriter(mem, 5)
	changes := rw.Apply(&rs, report)

	if changes != 1 {
		t.Errorf("expected 1 change, got %d", changes)
	}
	if len(rs.Rules) != 2 {
		t.Errorf("expected 2 rules after retire, got %d", len(rs.Rules))
	}
	for _, r := range rs.Rules {
		if r.ID == "r2" {
			t.Error("r2 should have been retired")
		}
	}
}

func TestProposeFromPatterns(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")
	rw := NewRewriter(mem, 5)
	rs := testRuleSet()

	patterns := []memory.InferencePattern{
		{FeatureKey: "temp,humidity", Prediction: "storm", Confidence: 0.85, Frequency: 5},
		{FeatureKey: "speed", Prediction: "fast", Confidence: 0.3, Frequency: 10}, // low confidence
		{FeatureKey: "x", Prediction: "y", Confidence: 0.9, Frequency: 1},          // low frequency
	}

	proposals := rw.ProposeFromPatterns(patterns, rs)
	if len(proposals) != 1 {
		t.Errorf("expected 1 proposal, got %d", len(proposals))
	}
	if len(proposals) > 0 && proposals[0].Head.Predicate != "storm_pattern_indicated" {
		t.Errorf("expected storm_pattern_indicated, got %s", proposals[0].Head.Predicate)
	}
}

func TestValidateReportContradictions(t *testing.T) {
	mem := memory.NewStore("/tmp/no.json")

	// Make r1 both bad (retire) and ... we need to trigger two meta-rules
	// Record enough fires for both retire and weaken to trigger
	for i := 0; i < 3; i++ {
		mem.RecordRuleFire("r1", false, "test")
	}

	rw := NewRewriter(mem, 2) // low threshold so weaken triggers at 2 fires
	rs := testRuleSet()

	report := rw.Evaluate(rs)

	// Should not have contradictory actions for same rule
	actionsByRule := make(map[string]int)
	for _, a := range report.Actions {
		actionsByRule[a.RuleID]++
	}
	for ruleID, count := range actionsByRule {
		if count > 1 {
			t.Errorf("contradictory actions for rule %s: %d actions", ruleID, count)
		}
	}
}
