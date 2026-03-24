package pipeline

import (
	"testing"

	"github.com/clara-phase1/config"
)

func TestRunPipeline(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Verbose = false
	result := Run(cfg)

	if result.SuperClaude == nil {
		t.Fatal("expected SuperClaude boss")
	}
	if len(result.SuperClaude.Verifiers) != cfg.NumVerifiers {
		t.Errorf("expected %d verifiers, got %d", cfg.NumVerifiers, len(result.SuperClaude.Verifiers))
	}
	if len(result.SuperClaude.Models) != 1 {
		t.Errorf("expected 1 model (AR), got %d", len(result.SuperClaude.Models))
	}
	if len(result.SuperClaude.Log) == 0 {
		t.Error("expected boss log entries")
	}

	if len(result.DatasetReports) == 0 {
		t.Fatal("expected dataset reports")
	}
	for name, dr := range result.DatasetReports {
		if dr.Metrics.TotalItems == 0 {
			t.Errorf("dataset %s: no items", name)
		}
		t.Logf("%s: items=%d accuracy=%.2f%% confidence=%.4f",
			name, dr.Metrics.TotalItems, dr.Metrics.Accuracy*100, dr.Metrics.MeanConfidence)
	}

	if result.Registry == nil {
		t.Error("expected data registry")
	}

	if len(result.Report.Sections) == 0 {
		t.Error("expected report sections")
	}
}

func TestCustomAgentConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Verbose = false
	cfg.NumVerifiers = 3
	cfg.NumPhDs = 4
	cfg.PhDSpecialties = []string{"logic-programs", "bayesian-lp", "constraint-programming"}

	result := Run(cfg)

	if len(result.SuperClaude.Verifiers) != 3 {
		t.Errorf("expected 3 verifiers, got %d", len(result.SuperClaude.Verifiers))
	}
	// 3 with specialties + 1 filled with "general"
	if len(result.SuperClaude.PhDs) != 4 {
		t.Errorf("expected 4 PhDs, got %d", len(result.SuperClaude.PhDs))
	}
}
