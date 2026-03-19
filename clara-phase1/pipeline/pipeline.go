// Package pipeline wires together all CLARA Phase 1 components
// and runs the full evaluation end-to-end via the SuperClaude boss agent.
// All configuration is dynamic — loaded from config, files, and data agents.
// No recursion. All inference state is isolated per-datum.
package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/clara-phase1/agents"
	"github.com/clara-phase1/ar"
	"github.com/clara-phase1/compose"
	"github.com/clara-phase1/config"
	"github.com/clara-phase1/dataagents"
	"github.com/clara-phase1/kinds"
	"github.com/clara-phase1/ml"
	"github.com/clara-phase1/report"
	"github.com/clara-phase1/testdata"
)

// Result holds the complete pipeline output.
type Result struct {
	Report              report.Report
	DatasetReports      map[string]report.DatasetReport
	OrchestratorResults []agents.OrchestratorResult
	SuperClaude         *agents.SuperClaudeAgent
	Registry            *dataagents.RegistryAgent
}

// Run executes the full CLARA Phase 1 pipeline with dynamic configuration.
func Run(cfg config.Config) Result {
	result := Result{DatasetReports: make(map[string]report.DatasetReport)}

	// === Step 1: Create the SuperClaude boss ===
	boss := agents.NewSuperClaudeAgent("super-claude-boss")
	result.SuperClaude = boss

	// === Step 2: Create agents from config ===
	for i := 0; i < cfg.NumVerifiers; i++ {
		boss.AddVerifier(agents.NewVerifierAgent(fmt.Sprintf("verifier-%d", i+1)))
	}
	for i := 0; i < cfg.NumPhDs && i < len(cfg.PhDSpecialties); i++ {
		boss.AddPhD(agents.NewPhDAgent(
			fmt.Sprintf("phd-%s", cfg.PhDSpecialties[i]),
			cfg.PhDSpecialties[i],
		))
	}
	// If more PhDs requested than specialties, fill with defaults
	for i := len(cfg.PhDSpecialties); i < cfg.NumPhDs; i++ {
		boss.AddPhD(agents.NewPhDAgent(fmt.Sprintf("phd-%d", i+1), "general"))
	}

	// === Step 3: Load engines from config files ===
	arEngine, err := loadAREngine(cfg)
	if err != nil && cfg.Verbose {
		fmt.Printf("[Pipeline] AR rules file not found, using defaults: %v\n", err)
	}
	if arEngine == nil {
		arEngine = buildDefaultAREngine()
	}

	mlEngine, err := loadMLEngine(cfg)
	if err != nil && cfg.Verbose {
		fmt.Printf("[Pipeline] BayesNet file not found, using defaults: %v\n", err)
	}
	if mlEngine == nil {
		mlEngine = buildDefaultMLEngine()
	}

	boss.AddModel(agents.NewModelAgent("model-bayesnet", kinds.KindBayesNets, mlEngine))
	boss.AddModel(agents.NewModelAgent("model-logicprog", kinds.KindLogicPrograms, arEngine))

	// === Step 4: Send directive to boss ===
	boss.Process(agents.Message{
		From: "pipeline", To: boss.ID(), Type: "directive",
		Payload:   fmt.Sprintf("Phase 1 evaluation: strategy=%s, soa=%.2f", cfg.Strategy, cfg.SOAAUROC),
		Timestamp: time.Now(),
	})

	if cfg.Verbose {
		fmt.Printf("[SuperClaude] Boss %q: %d verifiers, %d PhDs, %d models\n",
			boss.ID(), len(boss.Verifiers), len(boss.PhDs), len(boss.Models))
	}

	// === Step 5: Data Filing — discover, load, validate datasets ===
	registry := dataagents.NewRegistryAgent("data-registry", cfg.FeatureThreshold)
	result.Registry = registry

	// Try to discover datasets from configured directory
	discoverErr := registry.DiscoverAndFile(cfg.DataDir)
	if discoverErr != nil && cfg.Verbose {
		fmt.Printf("[DataAgents] Discovery from %s: %v\n", cfg.DataDir, discoverErr)
	}

	// Also register built-in datasets as fallback
	if len(registry.GetAll()) == 0 {
		if cfg.Verbose {
			fmt.Println("[DataAgents] No files found, registering built-in datasets")
		}
		for _, ds := range testdata.AllTestSets() {
			domain := dataagents.InferDomainFromFeatures(ds)
			registry.RegisterBuiltin(ds, domain)
		}
	}

	if cfg.Verbose {
		fmt.Print(registry.Summary())
	}

	// === Step 6: Build composition pipeline ===
	strategy := compose.CompositionStrategy(cfg.Strategy)
	pipe := compose.NewPipeline("clara-phase1", mlEngine, arEngine,
		kinds.KindBayesNets, kinds.KindLogicPrograms, strategy)

	// === Step 7: Run pipeline on all registered valid datasets ===
	entries := registry.GetValid()
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		ds := entry.Dataset

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
			fmt.Printf("[Pipeline] %s | Items: %d | Accuracy: %.2f%% | AUROC: %.4f | Quality: %.2f\n",
				ds.Name, batchMetrics.TotalItems, batchMetrics.Accuracy*100,
				batchMetrics.MeanAUROC, entry.Quality.OverallScore)
		}
	}

	// === Step 8: Run SuperClaude boss on first valid dataset ===
	if len(entries) > 0 {
		firstDS := entries[0].Dataset
		bossMsg := agents.Message{
			From: "pipeline", To: boss.ID(), Type: "request",
			Payload: firstDS, Timestamp: time.Now(),
		}
		resp, err := boss.Process(bossMsg)
		if err == nil {
			if orchResults, ok := resp.Payload.([]agents.OrchestratorResult); ok {
				result.OrchestratorResults = orchResults
			}
		}
	}

	if cfg.Verbose {
		fmt.Printf("[SuperClaude] Boss: %d log entries, %d orchestrator results\n",
			len(boss.Log), len(result.OrchestratorResults))
	}

	// === Step 9: Save inferences if configured ===
	if cfg.SaveInferences && cfg.InferencesFile != "" {
		saveInferences(cfg.InferencesFile, result.DatasetReports)
	}

	// === Step 10: Generate automated report ===
	gen := report.NewGenerator()
	result.Report = gen.GenerateFullReport(result.DatasetReports, result.OrchestratorResults, boss)

	return result
}

// loadAREngine tries to load AR rules from a JSON file.
func loadAREngine(cfg config.Config) (*ar.Engine, error) {
	return ar.LoadRulesFromFile(cfg.RulesFile)
}

// loadMLEngine tries to load a BayesNet from a JSON file.
func loadMLEngine(cfg config.Config) (*ml.Engine, error) {
	return ml.LoadBayesNetFromFile(cfg.BayesNetFile)
}

// buildDefaultAREngine creates the built-in AR engine (fallback).
func buildDefaultAREngine() *ar.Engine {
	data, _ := json.Marshal(ar.DefaultRuleSet())
	engine, _ := ar.LoadRulesFromJSON(data)
	return engine
}

// buildDefaultMLEngine creates the built-in ML engine (fallback).
func buildDefaultMLEngine() *ml.Engine {
	data, _ := json.Marshal(ml.DefaultBayesNetSpec())
	engine, _ := ml.LoadBayesNetFromJSON(data)
	return engine
}

func saveInferences(path string, reports map[string]report.DatasetReport) {
	data, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0644)
}
