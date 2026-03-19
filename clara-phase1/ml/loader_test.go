package ml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/clara-phase1/kinds"
)

func TestLoadBayesNetFromJSON(t *testing.T) {
	spec := DefaultBayesNetSpec()
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	engine, err := LoadBayesNetFromJSON(data)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if len(engine.Net.Nodes) != len(spec.Nodes) {
		t.Errorf("expected %d nodes, got %d", len(spec.Nodes), len(engine.Net.Nodes))
	}

	// Test inference works
	datum := kinds.Datum{
		Features: map[string]float64{"blood_pressure": 0.8, "glucose": 0.7, "heart_rate": 0.6},
		Label:    "treat_A",
	}
	result, err := engine.Infer(datum)
	if err != nil {
		t.Fatalf("inference error: %v", err)
	}
	if result.Prediction == "" {
		t.Error("expected prediction")
	}
	t.Logf("BayesNet loaded from JSON: prediction=%s conf=%.2f", result.Prediction, result.Confidence)
}

func TestLoadBayesNetFromFile(t *testing.T) {
	spec := DefaultBayesNetSpec()
	data, _ := json.MarshalIndent(spec, "", "  ")

	dir := t.TempDir()
	path := filepath.Join(dir, "model.json")
	os.WriteFile(path, data, 0644)

	engine, err := LoadBayesNetFromFile(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(engine.Net.Nodes) == 0 {
		t.Error("expected loaded nodes")
	}
}
