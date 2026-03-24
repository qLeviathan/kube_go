package report

import (
	"strings"
	"testing"

	"github.com/clara-phase1/agents"
	"github.com/clara-phase1/kinds"
)

func TestComplianceDynamic(t *testing.T) {
	gen := NewGenerator()

	failingDatasets := map[string]DatasetReport{
		"test-dataset": {
			DatasetName: "test-dataset",
			Results:     []kinds.InferenceResult{{Final: "wrong", Verified: false}},
			Metrics: BatchMetrics{
				TotalItems: 1, CorrectCount: 0, VerifiedCount: 0, ExplainedCount: 0,
				Accuracy: 0.0, MeanConfidence: 0.1, VerifyRate: 0.0, ExplainRate: 0.0,
			},
			Eval: []kinds.Metric{
				{Name: "Verifiability", Value: 0.0, Target: 1.0, Pass: false},
			},
		},
	}

	boss := agents.NewSuperClaudeAgent("test-boss")
	report := gen.GenerateFullReport(failingDatasets, nil, boss)

	var complianceContent string
	for _, s := range report.Sections {
		if s.Title == "Compliance" {
			complianceContent = s.Content
			break
		}
	}

	if complianceContent == "" {
		t.Fatal("no compliance section found")
	}

	if !strings.Contains(complianceContent, "FAIL") {
		t.Error("compliance section should show FAIL for failing metrics")
	}

	t.Log(complianceContent)
}

func TestComplianceAllPass(t *testing.T) {
	gen := NewGenerator()

	passingDatasets := map[string]DatasetReport{
		"test": {
			Metrics: BatchMetrics{
				TotalItems: 10, CorrectCount: 9, VerifiedCount: 10, ExplainedCount: 10,
				Accuracy: 0.9, MeanConfidence: 0.85, VerifyRate: 1.0, ExplainRate: 1.0,
			},
			Eval: []kinds.Metric{
				{Name: "Verifiability", Pass: true},
			},
		},
	}

	boss := agents.NewSuperClaudeAgent("test-boss")
	report := gen.GenerateFullReport(passingDatasets, nil, boss)

	var complianceContent string
	for _, s := range report.Sections {
		if s.Title == "Compliance" {
			complianceContent = s.Content
		}
	}

	if strings.Contains(complianceContent, "[FAIL]") {
		t.Errorf("all-pass scenario should not contain FAIL:\n%s", complianceContent)
	}
}

func TestFormatReport(t *testing.T) {
	r := Report{
		Title:    "Test Report",
		Sections: []Section{{Title: "Section 1", Content: "content here"}},
	}
	formatted := FormatReport(r)
	if !strings.Contains(formatted, "Test Report") {
		t.Error("formatted report should contain title")
	}
	if !strings.Contains(formatted, "Section 1") {
		t.Error("formatted report should contain section title")
	}
	if !strings.Contains(formatted, "END OF REPORT") {
		t.Error("formatted report should have ending marker")
	}
}

func TestComputeBatchMetrics(t *testing.T) {
	dataset := kinds.DataSet{
		Items: []kinds.Datum{{Label: "A"}, {Label: "B"}, {Label: "A"}},
	}
	results := []kinds.InferenceResult{
		{Final: "A", Verified: true, Explained: true, Confidence: 0.9},
		{Final: "B", Verified: true, Explained: true, Confidence: 0.8},
		{Final: "B", Verified: false, Explained: false, Confidence: 0.5},
	}

	m := ComputeBatchMetrics(results, dataset)
	if m.CorrectCount != 2 {
		t.Errorf("expected 2 correct, got %d", m.CorrectCount)
	}
	if m.VerifiedCount != 2 {
		t.Errorf("expected 2 verified, got %d", m.VerifiedCount)
	}
}

func TestEvaluateMetrics(t *testing.T) {
	bm := BatchMetrics{
		TotalItems: 10, CorrectCount: 8, VerifiedCount: 10, ExplainedCount: 9,
		Accuracy: 0.8, MeanConfidence: 0.75, VerifyRate: 1.0, ExplainRate: 0.9,
	}
	metrics := EvaluateMetrics(bm)
	if len(metrics) < 4 {
		t.Fatalf("expected >=4 metrics, got %d", len(metrics))
	}
	for _, m := range metrics {
		t.Logf("[%v] %s: %.4f (target %.4f)", m.Pass, m.Name, m.Value, m.Target)
	}
}
