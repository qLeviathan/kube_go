package ar

import (
	"testing"

	"github.com/clara-phase1/kinds"
)

func TestForwardChainBasic(t *testing.T) {
	engine := NewEngine()
	engine.AddFact("a")
	engine.AddFact("b")
	engine.AddRule("r1", Atom{Predicate: "c"}, Atom{Predicate: "a"}, Atom{Predicate: "b"})

	engine.ForwardChain()

	derived, trace := engine.Query("c")
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
	engine.AddFact("x")
	engine.AddRule("r1", Atom{Predicate: "y"}, Atom{Predicate: "x"})
	engine.AddRule("r2", Atom{Predicate: "z"}, Atom{Predicate: "y"})

	engine.ForwardChain()

	if derived, _ := engine.Query("y"); !derived {
		t.Fatal("expected 'y' to be derived")
	}
	if derived, _ := engine.Query("z"); !derived {
		t.Fatal("expected 'z' to be derived via chain")
	}
}

func TestForwardChainNoInfiniteLoop(t *testing.T) {
	engine := NewEngine()
	engine.AddFact("start")
	// Add many rules to test polynomial bound
	for i := 0; i < 100; i++ {
		engine.AddRule("r-padding",
			Atom{Predicate: "never_derived"},
			Atom{Predicate: "impossible_fact"},
		)
	}
	engine.AddRule("r-real", Atom{Predicate: "end"}, Atom{Predicate: "start"})

	engine.ForwardChain()

	if derived, _ := engine.Query("end"); !derived {
		t.Fatal("expected 'end' to be derived")
	}
	if derived, _ := engine.Query("never_derived"); derived {
		t.Fatal("'never_derived' should not be derived")
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

func TestReset(t *testing.T) {
	engine := NewEngine()
	engine.AddFact("a")
	engine.ForwardChain()

	if len(engine.Derived) == 0 {
		t.Fatal("expected derived facts before reset")
	}

	engine.Reset()
	if len(engine.Derived) != 0 {
		t.Fatal("expected empty derived set after reset")
	}
}
