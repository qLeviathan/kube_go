package ar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/clara-phase1/kinds"
)

func TestLoadRulesFromJSON(t *testing.T) {
	rs := DefaultRuleSet()
	data, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	engine, err := LoadRulesFromJSON(data)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}

	if len(engine.KB.Rules) != len(rs.Rules) {
		t.Errorf("expected %d rules, got %d", len(rs.Rules), len(engine.KB.Rules))
	}
	if len(engine.KB.Labels) != len(rs.Labels) {
		t.Errorf("expected %d labels, got %d", len(rs.Labels), len(engine.KB.Labels))
	}

	// Test inference works with loaded rules
	datum := kinds.Datum{
		Features: map[string]float64{"blood_pressure": 0.8, "heart_rate": 0.7, "glucose": 0.3},
		Label:    "treat_A",
	}
	result, err := engine.Infer(datum)
	if err != nil {
		t.Fatalf("inference error: %v", err)
	}
	if result.Prediction != "treat_A" {
		t.Errorf("expected treat_A, got %s", result.Prediction)
	}
}

func TestLoadRulesFromFile(t *testing.T) {
	rs := DefaultRuleSet()
	data, _ := json.MarshalIndent(rs, "", "  ")

	dir := t.TempDir()
	path := filepath.Join(dir, "rules.json")
	os.WriteFile(path, data, 0644)

	engine, err := LoadRulesFromFile(path)
	if err != nil {
		t.Fatalf("load file error: %v", err)
	}
	if len(engine.KB.Rules) == 0 {
		t.Error("expected loaded rules")
	}
}

func TestLoadRulesFromFileMissing(t *testing.T) {
	_, err := LoadRulesFromFile("/nonexistent/rules.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
