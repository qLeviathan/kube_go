package ar

import (
	"testing"

	"github.com/clara-phase1/kinds"
)

func TestForwardChainBasic(t *testing.T) {
	engine := NewEngine()
	engine.AddRule("r1", Atom{Predicate: "c"}, Atom{Predicate: "a"}, Atom{Predicate: "b"})

	facts := []Atom{{Predicate: "a"}, {Predicate: "b"}}
	derived, trace := engine.Query(facts, "c")
	if !derived {
		t.Fatal("expected 'c' to be derived")
	}
	if len(trace) == 0 {
		t.Fatal("expected non-empty proof trace")
	}
	t.Logf("Proof trace for 'c': %v", trace)
}

func TestForwardChainChained(t *testing.T) {
	engine := NewEngine()
	engine.AddRule("r1", Atom{Predicate: "y"}, Atom{Predicate: "x"})
	engine.AddRule("r2", Atom{Predicate: "z"}, Atom{Predicate: "y"})

	facts := []Atom{{Predicate: "x"}}
	if derived, _ := engine.Query(facts, "y"); !derived {
		t.Fatal("expected 'y' derived")
	}
	if derived, _ := engine.Query(facts, "z"); !derived {
		t.Fatal("expected 'z' derived via chain")
	}
}

func TestForwardChainBounded(t *testing.T) {
	engine := NewEngine()
	for i := 0; i < 100; i++ {
		engine.AddRule("r-noop", Atom{Predicate: "never"}, Atom{Predicate: "impossible"})
	}
	engine.AddRule("r-real", Atom{Predicate: "end"}, Atom{Predicate: "start"})

	facts := []Atom{{Predicate: "start"}}
	if derived, _ := engine.Query(facts, "end"); !derived {
		t.Fatal("expected 'end' derived")
	}
	if derived, _ := engine.Query(facts, "never"); derived {
		t.Fatal("'never' should not be derived")
	}
}

func TestInfer(t *testing.T) {
	engine := NewEngine()
	engine.AddRule("med-r1",
		Atom{Predicate: "treat_A_indicated"},
		Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "high"}},
		Atom{Predicate: "has_feature", Args: []string{"heart_rate", "high"}},
	)
	engine.SetLabel("treat_A_indicated", "treat_A")

	datum := kinds.Datum{
		Features: map[string]float64{"blood_pressure": 0.8, "heart_rate": 0.7},
		Label:    "treat_A",
	}
	result, err := engine.Infer(datum)
	if err != nil {
		t.Fatalf("inference error: %v", err)
	}
	if result.Prediction != "treat_A" {
		t.Errorf("expected treat_A, got %s", result.Prediction)
	}
	if len(result.ProofTrace) == 0 {
		t.Error("expected non-empty proof trace")
	}
	t.Logf("AR result: %s (conf=%.2f) trace=%v", result.Prediction, result.Confidence, result.ProofTrace)
}

// CRITICAL TEST: Verify consecutive Infer calls don't share state.
func TestInferIsolation(t *testing.T) {
	engine := NewEngine()
	engine.AddRule("r1",
		Atom{Predicate: "treat_A_indicated"},
		Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "high"}},
		Atom{Predicate: "has_feature", Args: []string{"heart_rate", "high"}},
	)
	engine.AddRule("r2",
		Atom{Predicate: "treat_B_indicated"},
		Atom{Predicate: "has_feature", Args: []string{"glucose", "high"}},
		Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "low"}},
	)
	engine.SetLabel("treat_A_indicated", "treat_A")
	engine.SetLabel("treat_B_indicated", "treat_B")

	// Datum 1: high BP + high HR => treat_A
	d1 := kinds.Datum{
		Features: map[string]float64{"blood_pressure": 0.8, "heart_rate": 0.7, "glucose": 0.3},
		Label:    "treat_A",
	}
	r1, _ := engine.Infer(d1)
	if r1.Prediction != "treat_A" {
		t.Errorf("datum1: expected treat_A, got %s", r1.Prediction)
	}

	// Datum 2: low BP + high glucose => treat_B
	// If state leaked, old facts (BP high) would still be present and r2's body wouldn't match
	d2 := kinds.Datum{
		Features: map[string]float64{"blood_pressure": 0.2, "heart_rate": 0.3, "glucose": 0.9},
		Label:    "treat_B",
	}
	r2, _ := engine.Infer(d2)
	if r2.Prediction != "treat_B" {
		t.Errorf("datum2: expected treat_B, got %s (state leak from datum1!)", r2.Prediction)
	}

	// Datum 3: run datum1 again to verify reproducibility
	r3, _ := engine.Infer(d1)
	if r3.Prediction != r1.Prediction {
		t.Errorf("datum3: expected same as datum1 (%s), got %s", r1.Prediction, r3.Prediction)
	}
}

func TestInferProofTraceNeverNil(t *testing.T) {
	engine := NewEngine()
	// No rules, no labels — should still return non-nil trace
	datum := kinds.Datum{Features: map[string]float64{"x": 0.5}, Label: "test"}
	result, err := engine.Infer(datum)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.ProofTrace == nil {
		t.Error("proof trace should never be nil")
	}
}
