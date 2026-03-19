package ml

import (
	"math"
	"testing"

	"github.com/clara-phase1/kinds"
)

func TestBayesNetInfer(t *testing.T) {
	net := NewBayesNet()
	net.AddNode("feature_a", []string{"high", "low"}, nil)
	net.SetPrior("feature_a", "high", 0.6)
	net.SetPrior("feature_a", "low", 0.4)

	net.AddNode("output", []string{"high", "low"}, []string{"feature_a"})
	net.SetCPT("output", "high", "high", 0.8)
	net.SetCPT("output", "high", "low", 0.2)
	net.SetCPT("output", "low", "high", 0.3)
	net.SetCPT("output", "low", "low", 0.7)

	net.SetLabel("high", "positive")
	net.SetLabel("low", "negative")

	engine := NewEngine(net)

	datum := kinds.Datum{Features: map[string]float64{"feature_a": 0.8}, Label: "positive"}
	result, err := engine.Infer(datum)
	if err != nil {
		t.Fatalf("inference error: %v", err)
	}
	if result.Prediction == "" {
		t.Fatal("expected non-empty prediction")
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("confidence out of range: %f", result.Confidence)
	}
	if len(result.ProofTrace) == 0 {
		t.Error("expected non-empty proof trace")
	}
	t.Logf("ML result: %s (conf=%.2f)", result.Prediction, result.Confidence)
}

// CRITICAL TEST: Verify consecutive Infer calls don't share belief state.
func TestInferIsolation(t *testing.T) {
	net := NewBayesNet()
	net.AddNode("feature_a", []string{"high", "low"}, nil)
	net.SetPrior("feature_a", "high", 0.5)
	net.SetPrior("feature_a", "low", 0.5)

	net.AddNode("output", []string{"high", "low"}, []string{"feature_a"})
	net.SetCPT("output", "high", "high", 0.9)
	net.SetCPT("output", "high", "low", 0.1)
	net.SetCPT("output", "low", "high", 0.2)
	net.SetCPT("output", "low", "low", 0.8)

	net.SetLabel("high", "positive")
	net.SetLabel("low", "negative")

	engine := NewEngine(net)

	// Infer with high feature value
	d1 := kinds.Datum{Features: map[string]float64{"feature_a": 0.9}, Label: "positive"}
	r1, _ := engine.Infer(d1)

	// Infer with low feature value — should produce different result
	d2 := kinds.Datum{Features: map[string]float64{"feature_a": 0.1}, Label: "negative"}
	r2, _ := engine.Infer(d2)

	// Results should differ if state is properly isolated
	if r1.Prediction == r2.Prediction && r1.Confidence == r2.Confidence {
		t.Error("consecutive inferences produced identical results — possible state leak")
	}

	// Run d1 again — should reproduce original result
	r3, _ := engine.Infer(d1)
	if r3.Prediction != r1.Prediction {
		t.Errorf("reproducibility failed: first=%s third=%s", r1.Prediction, r3.Prediction)
	}
	if math.Abs(r3.Confidence-r1.Confidence) > 0.001 {
		t.Errorf("confidence not reproducible: first=%.4f third=%.4f", r1.Confidence, r3.Confidence)
	}
}

func TestProofTraceNeverNil(t *testing.T) {
	net := NewBayesNet()
	net.AddNode("x", []string{"high", "low"}, nil)
	engine := NewEngine(net)

	datum := kinds.Datum{Features: map[string]float64{"x": 0.5}, Label: "test"}
	result, err := engine.Infer(datum)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.ProofTrace == nil {
		t.Error("proof trace should never be nil")
	}
}
