// Package pipeline wires together all CLARA Phase 1 components
// and runs the full evaluation pipeline end-to-end.
// No recursion.
package pipeline

import (
	"fmt"

	"github.com/clara-phase1/agents"
	"github.com/clara-phase1/ar"
	"github.com/clara-phase1/compose"
	"github.com/clara-phase1/kinds"
	"github.com/clara-phase1/ml"
	"github.com/clara-phase1/report"
	"github.com/clara-phase1/testdata"
)

// Config holds pipeline configuration.
type Config struct {
	Strategy   compose.CompositionStrategy
	SOAAUROC   float64 // state-of-the-art baseline to compare against
	Verbose    bool
}

// DefaultConfig returns default Phase 1 configuration.
func DefaultConfig() Config {
	return Config{
		Strategy: compose.StrategyARPriority,
		SOAAUROC: 0.60, // dummy SOA baseline
		Verbose:  true,
	}
}

// Result holds the complete pipeline output.
type Result struct {
	Report               report.Report
	DatasetReports       map[string]report.DatasetReport
	OrchestratorResults  []agents.OrchestratorResult
	Verifiers            []*agents.VerifierAgent
	PhDs                 []*agents.PhDAgent
	Models               []*agents.ModelAgent
}

// Run executes the full CLARA Phase 1 pipeline.
func Run(cfg Config) Result {
	result := Result{
		DatasetReports: make(map[string]report.DatasetReport),
	}

	// === Step 1: Create agents ===
	verifier1 := agents.NewVerifierAgent("verifier-soundness")
	verifier2 := agents.NewVerifierAgent("verifier-completeness")
	result.Verifiers = []*agents.VerifierAgent{verifier1, verifier2}

	phd1 := agents.NewPhDAgent("phd-bayesian-lp", "bayesian-lp")
	phd2 := agents.NewPhDAgent("phd-logic-programs", "logic-programs")
	result.PhDs = []*agents.PhDAgent{phd1, phd2}

	// === Step 2: Build AR engine (Logic Programs) ===
	arEngine := buildAREngine()

	// === Step 3: Build ML engine (Bayesian Network) ===
	mlEngine := buildMLEngine()

	// Create model agents
	mlModelAgent := agents.NewModelAgent("model-bayesnet", kinds.KindBayesNets, mlEngine)
	arModelAgent := agents.NewModelAgent("model-logicprog", kinds.KindLogicPrograms, arEngine)
	result.Models = []*agents.ModelAgent{mlModelAgent, arModelAgent}

	// === Step 4: Create composition pipeline ===
	pipe := compose.NewPipeline(
		"clara-phase1-composed",
		mlEngine, arEngine,
		kinds.KindBayesNets, kinds.KindLogicPrograms,
		cfg.Strategy,
	)

	// === Step 5: Create orchestrator ===
	orch := agents.NewOrchestrator()
	orch.AddVerifier(verifier1)
	orch.AddVerifier(verifier2)
	orch.AddPhD(phd1)
	orch.AddPhD(phd2)
	orch.AddModel(mlModelAgent)
	orch.AddModel(arModelAgent)

	// === Step 6: PhD domain analysis ===
	for i := 0; i < len(result.PhDs); i++ {
		p := result.PhDs[i]
		msg := agents.Message{
			From: "pipeline", To: p.ID(), Type: "request",
			Payload: agents.DomainRequest{
				Domain:      "medical-treatment",
				Constraints: []string{"multi-condition", "verifiable", "polynomial"},
			},
		}
		resp, err := p.Process(msg)
		if err == nil && cfg.Verbose {
			if advice, ok := resp.Payload.(agents.DomainAdvice); ok {
				fmt.Printf("[PhD %s] Domain: %s, Recommended %d kinds\n", p.ID(), advice.Domain, len(advice.RecommendedKinds))
				for j := 0; j < len(advice.Rationale); j++ {
					fmt.Printf("  → %s\n", advice.Rationale[j])
				}
			}
		}
	}

	// === Step 7: Run pipeline on all test datasets ===
	testSets := testdata.AllTestSets()
	for i := 0; i < len(testSets); i++ {
		ds := testSets[i]

		// Reset engines for each dataset
		arEngine.Reset()
		mlEngine.ResetBeliefs()

		// Re-populate AR engine for this dataset
		populateARForDataset(arEngine, ds)

		// Run composed pipeline
		results, batchMetrics := pipe.RunBatch(ds)

		// Evaluate Phase 1 metrics
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
			fmt.Printf("\n[Pipeline] Dataset: %s | Items: %d | Accuracy: %.2f%% | AUROC: %.4f\n",
				ds.Name, batchMetrics.TotalItems, batchMetrics.Accuracy*100, batchMetrics.MeanAUROC)
		}
	}

	// === Step 8: Run orchestrator on medical test set for detailed agent interaction ===
	arEngine.Reset()
	mlEngine.ResetBeliefs()
	medicalTest := testdata.MedicalTestSet()
	populateARForDataset(arEngine, medicalTest)
	orchResults := orch.RunBatch(medicalTest)
	result.OrchestratorResults = orchResults

	// === Step 9: Generate automated report ===
	gen := report.NewGenerator()
	result.Report = gen.GenerateFullReport(
		result.DatasetReports,
		result.OrchestratorResults,
		result.Verifiers,
		result.PhDs,
		result.Models,
	)

	return result
}

// buildAREngine creates a Logic Programs engine with medical/COA/supply rules.
func buildAREngine() *ar.Engine {
	engine := ar.NewEngine()

	// Medical treatment rules
	engine.AddRule("med-r1",
		ar.Atom{Predicate: "treat_A_indicated", Args: nil},
		ar.Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"heart_rate", "high"}},
	)
	engine.AddRule("med-r2",
		ar.Atom{Predicate: "treat_B_indicated", Args: nil},
		ar.Atom{Predicate: "has_feature", Args: []string{"glucose", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "low"}},
	)
	engine.AddRule("med-r3",
		ar.Atom{Predicate: "treat_C_indicated", Args: nil},
		ar.Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"glucose", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"heart_rate", "high"}},
	)
	engine.AddRule("med-r4",
		ar.Atom{Predicate: "no_treat_indicated", Args: nil},
		ar.Atom{Predicate: "has_feature", Args: []string{"blood_pressure", "low"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"glucose", "low"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"heart_rate", "low"}},
	)

	// COA rules
	engine.AddRule("coa-r1",
		ar.Atom{Predicate: "defend_indicated", Args: nil},
		ar.Atom{Predicate: "has_feature", Args: []string{"threat_level", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"supply_available", "low"}},
	)
	engine.AddRule("coa-r2",
		ar.Atom{Predicate: "advance_indicated", Args: nil},
		ar.Atom{Predicate: "has_feature", Args: []string{"threat_level", "low"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"supply_available", "high"}},
	)
	engine.AddRule("coa-r3",
		ar.Atom{Predicate: "retreat_indicated", Args: nil},
		ar.Atom{Predicate: "has_feature", Args: []string{"supply_available", "low"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"terrain_difficulty", "high"}},
	)

	// Supply chain rules
	engine.AddRule("sc-r1",
		ar.Atom{Predicate: "maintain_now_indicated", Args: nil},
		ar.Atom{Predicate: "has_feature", Args: []string{"equipment_age", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"usage_rate", "high"}},
	)
	engine.AddRule("sc-r2",
		ar.Atom{Predicate: "replace_indicated", Args: nil},
		ar.Atom{Predicate: "has_feature", Args: []string{"equipment_age", "high"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"failure_history", "high"}},
	)
	engine.AddRule("sc-r3",
		ar.Atom{Predicate: "no_action_indicated", Args: nil},
		ar.Atom{Predicate: "has_feature", Args: []string{"equipment_age", "low"}},
		ar.Atom{Predicate: "has_feature", Args: []string{"failure_history", "low"}},
	)

	// Label mappings
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

// buildMLEngine creates a Bayesian Network for the medical/COA/supply domains.
func buildMLEngine() *ml.Engine {
	net := ml.NewBayesNet()

	// Feature nodes (observed)
	net.AddNode("blood_pressure", []string{"high", "low"}, nil)
	net.SetPrior("blood_pressure", "high", 0.5)
	net.SetPrior("blood_pressure", "low", 0.5)

	net.AddNode("glucose", []string{"high", "low"}, nil)
	net.SetPrior("glucose", "high", 0.5)
	net.SetPrior("glucose", "low", 0.5)

	net.AddNode("heart_rate", []string{"high", "low"}, nil)
	net.SetPrior("heart_rate", "high", 0.5)
	net.SetPrior("heart_rate", "low", 0.5)

	// For COA/supply chain features, reuse same nodes with generic names
	net.AddNode("threat_level", []string{"high", "low"}, nil)
	net.SetPrior("threat_level", "high", 0.5)
	net.SetPrior("threat_level", "low", 0.5)

	net.AddNode("supply_available", []string{"high", "low"}, nil)
	net.SetPrior("supply_available", "high", 0.5)
	net.SetPrior("supply_available", "low", 0.5)

	net.AddNode("terrain_difficulty", []string{"high", "low"}, nil)
	net.SetPrior("terrain_difficulty", "high", 0.5)
	net.SetPrior("terrain_difficulty", "low", 0.5)

	net.AddNode("equipment_age", []string{"high", "low"}, nil)
	net.SetPrior("equipment_age", "high", 0.5)
	net.SetPrior("equipment_age", "low", 0.5)

	net.AddNode("usage_rate", []string{"high", "low"}, nil)
	net.SetPrior("usage_rate", "high", 0.5)
	net.SetPrior("usage_rate", "low", 0.5)

	net.AddNode("failure_history", []string{"high", "low"}, nil)
	net.SetPrior("failure_history", "high", 0.5)
	net.SetPrior("failure_history", "low", 0.5)

	// Decision node (output) — depends on key features
	decision := net.AddNode("decision", []string{"high", "low"}, []string{"blood_pressure", "glucose", "heart_rate"})
	_ = decision

	// CPT for decision node
	net.SetCPT("decision", "high", "high", 0.8)
	net.SetCPT("decision", "high", "low", 0.2)
	net.SetCPT("decision", "low", "high", 0.3)
	net.SetCPT("decision", "low", "low", 0.7)

	// Label mappings
	net.SetLabel("high", "treat_A")
	net.SetLabel("low", "no_treat")

	return ml.NewEngine(net)
}

// populateARForDataset adds domain-specific facts for a dataset.
func populateARForDataset(engine *ar.Engine, ds kinds.DataSet) {
	// The AR engine's Infer method handles per-datum fact loading,
	// but we can pre-load domain constants here
	switch {
	case len(ds.Items) > 0:
		sample := ds.Items[0]
		if _, ok := sample.Features["blood_pressure"]; ok {
			engine.AddFact("domain", "medical")
		}
		if _, ok := sample.Features["threat_level"]; ok {
			engine.AddFact("domain", "coa")
		}
		if _, ok := sample.Features["equipment_age"]; ok {
			engine.AddFact("domain", "supply_chain")
		}
	}
}
