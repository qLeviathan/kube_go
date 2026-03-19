// Package pipeline wires together all CLARA Phase 1 components
// and runs the full evaluation end-to-end via the SuperClaude boss agent.
// No recursion. All inference state is isolated per-datum.
package pipeline

import (
	"fmt"
	"time"

	"github.com/clara-phase1/agents"
	"github.com/clara-phase1/ar"
	"github.com/clara-phase1/compose"
	"github.com/clara-phase1/kinds"
	"github.com/clara-phase1/ml"
	"github.com/clara-phase1/report"
	"github.com/clara-phase1/testdata"
)

type Config struct {
	Strategy compose.CompositionStrategy
	SOAAUROC float64
	Verbose  bool
}

func DefaultConfig() Config {
	return Config{
		Strategy: compose.StrategyARPriority,
		SOAAUROC: 0.60,
		Verbose:  true,
	}
}

type Result struct {
	Report              report.Report
	DatasetReports      map[string]report.DatasetReport
	OrchestratorResults []agents.OrchestratorResult
	SuperClaude         *agents.SuperClaudeAgent
}

// Run executes the full CLARA Phase 1 pipeline via SuperClaude boss.
func Run(cfg Config) Result {
	result := Result{DatasetReports: make(map[string]report.DatasetReport)}

	// === Step 1: Create the SuperClaude boss ===
	boss := agents.NewSuperClaudeAgent("super-claude-boss")
	result.SuperClaude = boss

	// === Step 2: Create and register sub-agents ===
	boss.AddVerifier(agents.NewVerifierAgent("verifier-soundness"))
	boss.AddVerifier(agents.NewVerifierAgent("verifier-completeness"))
	boss.AddPhD(agents.NewPhDAgent("phd-bayesian-lp", "bayesian-lp"))
	boss.AddPhD(agents.NewPhDAgent("phd-logic-programs", "logic-programs"))

	arEngine := buildAREngine()
	mlEngine := buildMLEngine()
	boss.AddModel(agents.NewModelAgent("model-bayesnet", kinds.KindBayesNets, mlEngine))
	boss.AddModel(agents.NewModelAgent("model-logicprog", kinds.KindLogicPrograms, arEngine))

	// === Step 3: Send directive to boss ===
	boss.Process(agents.Message{
		From: "pipeline", To: boss.ID(), Type: "directive",
		Payload:   "Phase 1 evaluation: run all test datasets, verify, report",
		Timestamp: time.Now(),
	})

	if cfg.Verbose {
		fmt.Printf("[SuperClaude] Boss agent %q initialized with %d verifiers, %d PhDs, %d models\n",
			boss.ID(), len(boss.Verifiers), len(boss.PhDs), len(boss.Models))
	}

	// === Step 4: Build composition pipeline for metric evaluation ===
	pipe := compose.NewPipeline("clara-phase1", mlEngine, arEngine,
		kinds.KindBayesNets, kinds.KindLogicPrograms, cfg.Strategy)

	// === Step 5: Run on all test datasets ===
	testSets := testdata.AllTestSets()
	for i := 0; i < len(testSets); i++ {
		ds := testSets[i]

		// Run composed pipeline (each Infer is isolated — no resets needed)
		results, batchMetrics := pipe.RunBatch(ds)
		phase1Metrics := compose.EvaluatePhase1Metrics(batchMetrics, cfg.SOAAUROC)

		dr := report.DatasetReport{
			DatasetName: ds.Name,
			Results:     results,
			Metrics:     batchMetrics,
			Phase1Eval:  phase1Metrics,
			SOAAUROC:    cfg.SOAAUROC,
		}
		result.DatasetReports[ds.Name] = dr

		if cfg.Verbose {
			fmt.Printf("[Pipeline] Dataset: %s | Items: %d | Accuracy: %.2f%% | AUROC: %.4f\n",
				ds.Name, batchMetrics.TotalItems, batchMetrics.Accuracy*100, batchMetrics.MeanAUROC)
		}
	}

	// === Step 6: Run SuperClaude boss on medical test set for full agent interaction ===
	medicalTest := testdata.MedicalTestSet()
	bossMsg := agents.Message{
		From: "pipeline", To: boss.ID(), Type: "request",
		Payload: medicalTest, Timestamp: time.Now(),
	}
	resp, err := boss.Process(bossMsg)
	if err == nil {
		if orchResults, ok := resp.Payload.([]agents.OrchestratorResult); ok {
			result.OrchestratorResults = orchResults
		}
	}

	if cfg.Verbose {
		fmt.Printf("[SuperClaude] Boss completed with %d log entries, %d orchestrator results\n",
			len(boss.Log), len(result.OrchestratorResults))
	}

	// === Step 7: Generate automated report ===
	gen := report.NewGenerator()
	result.Report = gen.GenerateFullReport(result.DatasetReports, result.OrchestratorResults, boss)

	return result
}

func buildAREngine() *ar.Engine {
	engine := ar.NewEngine()

	// Medical treatment rules
	engine.AddRule("med-r1",
		ar.Atom{Predicate: "treat_A_indicated"},
		ar.Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"heart_rate", "high"}},
	)
	engine.AddRule("med-r2",
		ar.Atom{Predicate: "treat_B_indicated"},
		ar.Atom{Predicate: "has_feature", Args: []string{"glucose", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "low"}},
	)
	engine.AddRule("med-r3",
		ar.Atom{Predicate: "treat_C_indicated"},
		ar.Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"glucose", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"heart_rate", "high"}},
	)
	engine.AddRule("med-r4",
		ar.Atom{Predicate: "no_treat_indicated"},
		ar.Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "low"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"glucose", "low"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"heart_rate", "low"}},
	)

	// COA rules
	engine.AddRule("coa-r1",
		ar.Atom{Predicate: "defend_indicated"},
		ar.Atom{Predicate: "has_feature", Args: []string{"threat_level", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"supply_available", "low"}},
	)
	engine.AddRule("coa-r2",
		ar.Atom{Predicate: "advance_indicated"},
		ar.Atom{Predicate: "has_feature", Args: []string{"threat_level", "low"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"supply_available", "high"}},
	)
	engine.AddRule("coa-r3",
		ar.Atom{Predicate: "retreat_indicated"},
		ar.Atom{Predicate: "has_feature", Args: []string{"supply_available", "low"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"terrain_difficulty", "high"}},
	)

	// Supply chain rules
	engine.AddRule("sc-r1",
		ar.Atom{Predicate: "maintain_now_indicated"},
		ar.Atom{Predicate: "has_feature", Args: []string{"equipment_age", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"usage_rate", "high"}},
	)
	engine.AddRule("sc-r2",
		ar.Atom{Predicate: "replace_indicated"},
		ar.Atom{Predicate: "has_feature", Args: []string{"equipment_age", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"failure_history", "high"}},
	)
	engine.AddRule("sc-r3",
		ar.Atom{Predicate: "no_action_indicated"},
		ar.Atom{Predicate: "has_feature", Args: []string{"equipment_age", "low"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"failure_history", "low"}},
	)

	// Labels
	engine.SetLabel("treat_A_indicated", "treat_A")
	engine.SetLabel("treat_B_indicated", "treat_B")
	engine.SetLabel("treat_C_indicated", "treat_C")
	engine.SetLabel("no_treat_indicated", "no_treat")
	engine.SetLabel("defend_indicated", "defend")
	engine.SetLabel("advance_indicated", "advance")
	engine.SetLabel("retreat_indicated", "retreat")
	engine.SetLabel("maintain_now_indicated", "maintain_now")
	engine.SetLabel("replace_indicated", "replace")
	engine.SetLabel("no_action_indicated", "no_action")

	return engine
}

func buildMLEngine() *ml.Engine {
	net := ml.NewBayesNet()

	// Feature nodes
	features := []string{
		"blood_pressure", "glucose", "heart_rate",
		"threat_level", "supply_available", "terrain_difficulty",
		"equipment_age", "usage_rate", "failure_history",
	}
	for _, f := range features {
		net.AddNode(f, []string{"high", "low"}, nil)
		net.SetPrior(f, "high", 0.5)
		net.SetPrior(f, "low", 0.5)
	}

	// Decision output node
	net.AddNode("decision", []string{"high", "low"},
		[]string{"blood_pressure", "glucose", "heart_rate"})
	net.SetCPT("decision", "high", "high", 0.8)
	net.SetCPT("decision", "high", "low", 0.2)
	net.SetCPT("decision", "low", "high", 0.3)
	net.SetCPT("decision", "low", "low", 0.7)

	net.SetLabel("high", "treat_A")
	net.SetLabel("low", "no_treat")

	return ml.NewEngine(net)
}
