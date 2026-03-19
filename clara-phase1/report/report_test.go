package report

import (
	"strings"
	"testing"

	"github.com/clara-phase1/agents"
	"github.com/clara-phase1/compose"
	"github.com/clara-phase1/kinds"
)

func TestComplianceDynamic(t *testing.T) {
	gen := NewGenerator()

	// Create a dataset report where some metrics FAIL
	failingDatasets := map[string]DatasetReport{
		"test-dataset": {
			DatasetName: "test-dataset",
			Results:     []kinds.ComposedResult{{Final: "wrong", Verified: false}},
			Metrics: compose.BatchMetrics{
				TotalItems: 1, CorrectCount: 0, VerifiedCount: 0, ExplainedCount: 0,
				Accuracy: 0.0, MeanAUROC: 0.1, VerifyRate: 0.0, ExplainRate: 0.0,
			},
			Phase1Eval: []kinds.Metric{
				{Name: "Verifiability", Value: 0.0, Target: 1.0, Pass: false},
			},
			SOAAUROC: 0.8,
		},
	}

	boss := agents.NewSuperClaudeAgent("test-boss")
	report := gen.GenerateFullReport(failingDatasets, nil, boss)

	// Find compliance section
	var complianceContent string
	for _, s := range report.Sections {
		if s.Title == "DARPA Compliance" {
			complianceContent = s.Content
			break
		}
	}

	if complianceContent == "" {
		t.Fatal("no compliance section found")
	}

	// Compliance should show FAIL for verification, AUROC, error rate, explainability
	if !strings.Contains(complianceContent, "FAIL") {
		t.Error("compliance section should show FAIL for failing metrics (not hardcoded PASS)")
	}

	t.Log(complianceContent)
}

func TestComplianceAllPass(t *testing.T) {
	gen := NewGenerator()

	passingDatasets := map[string]DatasetReport{
		"test": {
			Metrics: compose.BatchMetrics{
				TotalItems: 10, CorrectCount: 9, VerifiedCount: 10, ExplainedCount: 10,
				Accuracy: 0.9, MeanAUROC: 0.85, VerifyRate: 1.0, ExplainRate: 1.0,
			},
			Phase1Eval: []kinds.Metric{
				{Name: "Verifiability", Pass: true},
			},
			SOAAUROC: 0.8,
		},
	}

	boss := agents.NewSuperClaudeAgent("test-boss")
	report := gen.GenerateFullReport(passingDatasets, nil, boss)

	var complianceContent string
	for _, s := range report.Sections {
		if s.Title == "DARPA Compliance" {
			complianceContent = s.Content
		}
	}

	// Should NOT contain FAIL when all metrics pass
	if strings.Contains(complianceContent, "[FAIL]") {
		t.Errorf("all-pass scenario should not contain FAIL:\n%s", complianceContent)
	}
}

func TestFormatReport(t *testing.T) {
	r := Report{
		Title:   "Test Report",
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
