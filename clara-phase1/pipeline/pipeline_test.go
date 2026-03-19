package pipeline

import (
	"testing"

	"github.com/clara-phase1/compose"
)

func TestRunPipeline(t *testing.T) {
	cfg := Config{Strategy: compose.StrategyARPriority, SOAAUROC: 0.60, Verbose: false}
	result := Run(cfg)

	// Verify SuperClaude boss was created and used
	if result.SuperClaude == nil {
		t.Fatal("expected SuperClaude boss")
	}
	if len(result.SuperClaude.Verifiers) != 2 {
		t.Errorf("expected 2 verifiers, got %d", len(result.SuperClaude.Verifiers))
	}
	if len(result.SuperClaude.PhDs) != 2 {
		t.Errorf("expected 2 PhDs, got %d", len(result.SuperClaude.PhDs))
	}
	if len(result.SuperClaude.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(result.SuperClaude.Models))
	}
	if len(result.SuperClaude.Log) == 0 {
		t.Error("expected boss to have log entries")
	}

	// Verify datasets evaluated
	if len(result.DatasetReports) == 0 {
		t.Fatal("expected dataset reports")
	}
	for name, dr := range result.DatasetReports {
		if dr.Metrics.TotalItems == 0 {
			t.Errorf("dataset %s: no items", name)
		}
		if len(dr.Phase1Eval) == 0 {
			t.Errorf("dataset %s: no metrics", name)
		}
		t.Logf("Dataset %s: items=%d accuracy=%.2f%% auroc=%.4f",
			name, dr.Metrics.TotalItems, dr.Metrics.Accuracy*100, dr.Metrics.MeanAUROC)
	}

	// Verify orchestrator results from boss
	if len(result.OrchestratorResults) == 0 {
		t.Error("expected orchestrator results from boss")
	}

	// Verify report
	if len(result.Report.Sections) == 0 {
		t.Error("expected report sections")
	}
	for _, s := range result.Report.Sections {
		t.Logf("  Section: %s (%d chars)", s.Title, len(s.Content))
	}
}

func TestAllStrategies(t *testing.T) {
	for _, strategy := range []compose.CompositionStrategy{
		compose.StrategyARPriority, compose.StrategyWeightedFusion, compose.StrategyConsensus,
	} {
		t.Run(string(strategy), func(t *testing.T) {
			cfg := Config{Strategy: strategy, SOAAUROC: 0.50, Verbose: false}
			result := Run(cfg)
			if len(result.DatasetReports) == 0 {
				t.Fatal("expected dataset reports")
			}
		})
	}
}
