package ml

import (
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

	datum := kinds.Datum{
		Features: map[string]float64{"feature_a": 0.8},
		Label:    "positive",
	}

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
	t.Logf("ML result: %s (conf=%.2f) trace=%v", result.Prediction, result.Confidence, result.ProofTrace)
}

func TestResetBeliefs(t *testing.T) {
	net := NewBayesNet()
	net.AddNode("x", []string{"a", "b"}, nil)

	engine := NewEngine(net)

	// Modify beliefs
	net.Nodes["x"].Belief["a"] = 0.9
	net.Nodes["x"].Belief["b"] = 0.1

	engine.ResetBeliefs()

	if net.Nodes["x"].Belief["a"] != 0.5 {
		t.Errorf("expected 0.5 after reset, got %f", net.Nodes["x"].Belief["a"])
	}
}

func TestNormalize(t *testing.T) {
	node := &Node{
		Name:   "test",
		States: []string{"a", "b", "c"},
		Belief: map[string]float64{"a": 2.0, "b": 3.0, "c": 5.0},
	}
	normalize(node)

	total := node.Belief["a"] + node.Belief["b"] + node.Belief["c"]
	if total < 0.999 || total > 1.001 {
		t.Errorf("beliefs don't sum to 1: %f", total)
	}
}
