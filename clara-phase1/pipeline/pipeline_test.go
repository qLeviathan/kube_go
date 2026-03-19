package pipeline

import (
	"testing"

	"github.com/clara-phase1/compose"
)

func TestRunPipeline(t *testing.T) {
	cfg := Config{
		Strategy: compose.StrategyARPriority,
		SOAAUROC: 0.60,
		Verbose:  false,
	}

	result := Run(cfg)

	// Verify agents were created
	if len(result.Verifiers) != 2 {
		t.Errorf("expected 2 verifiers, got %d", len(result.Verifiers))
	}
	if len(result.PhDs) != 2 {
		t.Errorf("expected 2 PhDs, got %d", len(result.PhDs))
	}
	if len(result.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(result.Models))
	}

	// Verify datasets were evaluated
	if len(result.DatasetReports) == 0 {
		t.Fatal("expected dataset reports")
	}

	for name, dr := range result.DatasetReports {
		if dr.Metrics.TotalItems == 0 {
			t.Errorf("dataset %s: no items processed", name)
		}
		if len(dr.Phase1Eval) == 0 {
			t.Errorf("dataset %s: no Phase 1 metrics evaluated", name)
		}
		t.Logf("Dataset %s: items=%d accuracy=%.2f%% auroc=%.4f",
			name, dr.Metrics.TotalItems, dr.Metrics.Accuracy*100, dr.Metrics.MeanAUROC)
	}

	// Verify orchestrator ran
	if len(result.OrchestratorResults) == 0 {
		t.Error("expected orchestrator results")
	}

	// Verify report was generated
	if len(result.Report.Sections) == 0 {
		t.Error("expected report sections")
	}
	t.Logf("Report sections: %d", len(result.Report.Sections))
	for _, s := range result.Report.Sections {
		t.Logf("  Section: %s (%d chars)", s.Title, len(s.Content))
	}
}

func TestAllStrategies(t *testing.T) {
	strategies := []compose.CompositionStrategy{
		compose.StrategyARPriority,
		compose.StrategyWeightedFusion,
		compose.StrategyConsensus,
	}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			cfg := Config{
				Strategy: strategy,
				SOAAUROC: 0.50,
				Verbose:  false,
			}
			result := Run(cfg)
			if len(result.DatasetReports) == 0 {
				t.Fatal("expected dataset reports")
			}
		})
	}
}
