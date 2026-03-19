package dataagents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clara-phase1/kinds"
)

func TestDiscoveryAgent(t *testing.T) {
	// Create temp dir with a CSV file
	dir := t.TempDir()
	csvContent := "x,y,label\n0.5,0.8,A\n0.1,0.3,B\n"
	os.WriteFile(filepath.Join(dir, "test.csv"), []byte(csvContent), 0644)

	agent := NewDiscoveryAgent("disc-1")
	metas, err := agent.Discover(dir)
	if err != nil {
		t.Fatalf("discovery error: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(metas))
	}
	if metas[0].Name != "test" {
		t.Errorf("expected name 'test', got %q", metas[0].Name)
	}
	if metas[0].NumItems != 2 {
		t.Errorf("expected 2 items, got %d", metas[0].NumItems)
	}
	if metas[0].NumFeatures != 2 {
		t.Errorf("expected 2 features, got %d", metas[0].NumFeatures)
	}
	t.Logf("Discovered: %+v", metas[0])
}

func TestDiscoveryAgentMissingDir(t *testing.T) {
	agent := NewDiscoveryAgent("disc-1")
	_, err := agent.Discover("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing directory")
	}
}

func TestLoaderAgentCSV(t *testing.T) {
	dir := t.TempDir()
	csv := "a,b,label\n0.1,0.9,X\n0.8,0.2,Y\n0.5,0.5,X\n"
	path := filepath.Join(dir, "data.csv")
	os.WriteFile(path, []byte(csv), 0644)

	meta := DataMeta{Name: "data", FilePath: path, Format: "csv",
		FeatureNames: []string{"a", "b"}, LabelName: "label", NumFeatures: 2, NumItems: 3}

	loader := NewLoaderAgent("loader-1", 0.5)
	ds, err := loader.Load(meta)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(ds.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(ds.Items))
	}
	if ds.Items[0].Label != "X" {
		t.Errorf("expected label X, got %q", ds.Items[0].Label)
	}
	if ds.Items[0].Features["a"] != 0.1 {
		t.Errorf("expected feature a=0.1, got %f", ds.Items[0].Features["a"])
	}
}

func TestLoaderAgentJSON(t *testing.T) {
	dir := t.TempDir()
	jsonData := `[{"x": 0.5, "y": 0.8, "label": "A"}, {"x": 0.1, "y": 0.3, "label": "B"}]`
	path := filepath.Join(dir, "data.json")
	os.WriteFile(path, []byte(jsonData), 0644)

	meta := DataMeta{Name: "data", FilePath: path, Format: "json", NumItems: 2}

	loader := NewLoaderAgent("loader-1", 0.5)
	ds, err := loader.Load(meta)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(ds.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(ds.Items))
	}
}

func TestValidatorAgent(t *testing.T) {
	validator := NewValidatorAgent("val-1")

	// Good dataset
	good := kinds.DataSet{
		Name: "good",
		Items: []kinds.Datum{
			{Features: map[string]float64{"a": 0.5, "b": 0.8}, Label: "X"},
			{Features: map[string]float64{"a": 0.1, "b": 0.3}, Label: "Y"},
		},
	}
	report := validator.Validate(good)
	if !report.Valid {
		t.Errorf("expected valid, got issues: %v", report.Issues)
	}

	// Empty dataset
	empty := kinds.DataSet{Name: "empty"}
	report = validator.Validate(empty)
	if report.Valid {
		t.Error("empty dataset should be invalid")
	}

	// Missing label
	bad := kinds.DataSet{
		Name: "bad",
		Items: []kinds.Datum{
			{Features: map[string]float64{"a": 0.5}, Label: ""},
		},
	}
	report = validator.Validate(bad)
	if report.Valid {
		t.Error("missing label should be invalid")
	}
}

func TestQualityAgent(t *testing.T) {
	qa := NewQualityAgent("qa-1")

	ds := kinds.DataSet{
		Name: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"a": 0.8, "b": 0.2}, Label: "X"},
			{Features: map[string]float64{"a": 0.3, "b": 0.7}, Label: "Y"},
			{Features: map[string]float64{"a": 0.5, "b": 0.5}, Label: "X"},
		},
	}

	score := qa.Assess(ds)
	if score.OverallScore <= 0 {
		t.Error("expected positive quality score")
	}
	if score.CompletenessScore != 1.0 {
		t.Errorf("expected completeness 1.0, got %f", score.CompletenessScore)
	}
	if len(score.Details) == 0 {
		t.Error("expected quality details")
	}
	t.Logf("Quality: overall=%.2f details=%v", score.OverallScore, score.Details)
}

func TestRegistryAgent(t *testing.T) {
	dir := t.TempDir()
	csv := "a,b,label\n0.1,0.9,X\n0.8,0.2,Y\n"
	os.WriteFile(filepath.Join(dir, "test.csv"), []byte(csv), 0644)

	reg := NewRegistryAgent("reg-1", 0.5)
	err := reg.DiscoverAndFile(dir)
	if err != nil {
		t.Fatalf("registry error: %v", err)
	}
	if len(reg.GetAll()) != 1 {
		t.Fatalf("expected 1 registered dataset, got %d", len(reg.GetAll()))
	}

	entry := reg.GetAll()[0]
	if !entry.Filed {
		t.Error("expected filed=true")
	}
	if len(entry.Dataset.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(entry.Dataset.Items))
	}

	t.Log(reg.Summary())
}

func TestRegistryBuiltin(t *testing.T) {
	reg := NewRegistryAgent("reg-1", 0.5)
	ds := kinds.DataSet{
		Name: "builtin-test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"blood_pressure": 0.8}, Label: "treat_A"},
		},
	}
	reg.RegisterBuiltin(ds, "medical")

	entries := reg.GetByDomain("medical")
	if len(entries) != 1 {
		t.Fatalf("expected 1 medical dataset, got %d", len(entries))
	}
}

func TestInferDomainFromFeatures(t *testing.T) {
	medical := kinds.DataSet{Items: []kinds.Datum{
		{Features: map[string]float64{"blood_pressure": 0.5, "glucose": 0.3}},
	}}
	if InferDomainFromFeatures(medical) != "medical" {
		t.Error("expected medical domain")
	}

	coa := kinds.DataSet{Items: []kinds.Datum{
		{Features: map[string]float64{"threat_level": 0.5, "supply_available": 0.3}},
	}}
	if InferDomainFromFeatures(coa) != "coa" {
		t.Error("expected coa domain")
	}
}
