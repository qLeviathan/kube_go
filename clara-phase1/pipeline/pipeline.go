// Package pipeline wires together all CLARA components
// and runs the full evaluation end-to-end.
// Phase 2 adds: persistent memory, swarm coordination,
// recursive rule rewriting, and future chain prediction.
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
	"github.com/clara-phase1/futures"
	"github.com/clara-phase1/kinds"
	"github.com/clara-phase1/memory"
	"github.com/clara-phase1/ml"
	"github.com/clara-phase1/report"
	"github.com/clara-phase1/rewriter"
	"github.com/clara-phase1/swarm"
	"github.com/clara-phase1/testdata"
)

// Result holds the complete pipeline output.
type Result struct {
	Report              report.Report
	DatasetReports      map[string]report.DatasetReport
	OrchestratorResults []agents.OrchestratorResult
	SuperClaude         *agents.SuperClaudeAgent
	Registry            *dataagents.RegistryAgent

	// Phase 2 results
	Memory         *memory.Store
	Swarm          *swarm.Controller
	RewriteReport  *rewriter.RewriteReport
	FuturePredictor *futures.Predictor
}

// Run executes the full CLARA pipeline with dynamic configuration.
func Run(cfg config.Config) Result {
	startTime := time.Now()
	result := Result{DatasetReports: make(map[string]report.DatasetReport)}

	// === Step 1: Load persistent memory ===
	mem := memory.NewStore(cfg.MemoryFile)
	result.Memory = mem

	if cfg.Verbose {
		fmt.Printf("[Memory] %s\n", mem.Summary())
	}

	// === Step 2: Load engines from config files ===
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

	// Get the rule set for rewriter and futures
	currentRuleSet := loadRuleSet(cfg)

	// === Step 3: Create swarm or traditional boss ===
	var boss *agents.SuperClaudeAgent

	if cfg.SwarmEnabled {
		swarmCfg := swarm.SwarmConfig{
			MinVerifiers:     cfg.MinVerifiers,
			MaxVerifiers:     cfg.MaxVerifiers,
			MinPhDs:          cfg.MinPhDs,
			MaxPhDs:          cfg.MaxPhDs,
			MinModels:        cfg.MinModels,
			MaxModels:        cfg.MaxModels,
			ScaleUpThreshold: cfg.ScaleUpThreshold,
			ScaleDownIdleSec: cfg.ScaleDownIdleSec,
			MaxSwarmSize:     cfg.MaxSwarmSize,
		}

		ctrl := swarm.NewController(swarmCfg, mem)
		result.Swarm = ctrl

		// Spawn initial agents
		for i := 0; i < cfg.NumVerifiers; i++ {
			ctrl.SpawnVerifier(fmt.Sprintf("verifier-%d", i+1))
		}
		for i := 0; i < cfg.NumPhDs && i < len(cfg.PhDSpecialties); i++ {
			ctrl.SpawnPhD(fmt.Sprintf("phd-%s", cfg.PhDSpecialties[i]), cfg.PhDSpecialties[i])
		}
		for i := len(cfg.PhDSpecialties); i < cfg.NumPhDs; i++ {
			ctrl.SpawnPhD(fmt.Sprintf("phd-%d", i+1), "general")
		}
		ctrl.SpawnModel("model-bayesnet", kinds.KindBayesNets, mlEngine)
		ctrl.SpawnModel("model-logicprog", kinds.KindLogicPrograms, arEngine)

		boss = ctrl.Boss

		if cfg.Verbose {
			status := ctrl.Status()
			fmt.Printf("[Swarm] %d agents, mesh network active\n", status.TotalAgents)
		}
	} else {
		// Traditional boss mode (Phase 1 compatible)
		boss = agents.NewSuperClaudeAgent("super-claude-boss")
		for i := 0; i < cfg.NumVerifiers; i++ {
			boss.AddVerifier(agents.NewVerifierAgent(fmt.Sprintf("verifier-%d", i+1)))
		}
		for i := 0; i < cfg.NumPhDs && i < len(cfg.PhDSpecialties); i++ {
			boss.AddPhD(agents.NewPhDAgent(
				fmt.Sprintf("phd-%s", cfg.PhDSpecialties[i]),
				cfg.PhDSpecialties[i],
			))
		}
		for i := len(cfg.PhDSpecialties); i < cfg.NumPhDs; i++ {
			boss.AddPhD(agents.NewPhDAgent(fmt.Sprintf("phd-%d", i+1), "general"))
		}
		boss.AddModel(agents.NewModelAgent("model-bayesnet", kinds.KindBayesNets, mlEngine))
		boss.AddModel(agents.NewModelAgent("model-logicprog", kinds.KindLogicPrograms, arEngine))
	}

	result.SuperClaude = boss

	// === Step 4: Initialize future predictor ===
	var predictor *futures.Predictor
	if cfg.EnableFutures {
		predictor = futures.NewPredictor(mem, currentRuleSet, cfg.FutureChainDepth)
		warmed := predictor.WarmCache(cfg.FutureCacheSize)
		result.FuturePredictor = predictor

		if cfg.Verbose {
			fmt.Printf("[Futures] Warmed %d cache entries from memory\n", warmed)
		}
	}

	// === Step 5: Send directive to boss ===
	boss.Process(agents.Message{
		From: "pipeline", To: boss.ID(), Type: "directive",
		Payload:   fmt.Sprintf("Phase 2 evaluation: strategy=%s, soa=%.2f, swarm=%v, rewriter=%v, futures=%v",
			cfg.Strategy, cfg.SOAAUROC, cfg.SwarmEnabled, cfg.EnableRewriter, cfg.EnableFutures),
		Timestamp: time.Now(),
	})

	if cfg.Verbose {
		fmt.Printf("[SuperClaude] Boss %q: %d verifiers, %d PhDs, %d models\n",
			boss.ID(), len(boss.Verifiers), len(boss.PhDs), len(boss.Models))
	}

	// === Step 6: Data Filing — discover, load, validate datasets ===
	registry := dataagents.NewRegistryAgent("data-registry", cfg.FeatureThreshold)
	result.Registry = registry

	discoverErr := registry.DiscoverAndFile(cfg.DataDir)
	if discoverErr != nil && cfg.Verbose {
		fmt.Printf("[DataAgents] Discovery from %s: %v\n", cfg.DataDir, discoverErr)
	}

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

	// === Step 7: Build composition pipeline ===
	strategy := compose.CompositionStrategy(cfg.Strategy)
	pipe := compose.NewPipeline("clara-phase1", mlEngine, arEngine,
		kinds.KindBayesNets, kinds.KindLogicPrograms, strategy)

	// === Step 8: Run pipeline on all registered valid datasets ===
	entries := registry.GetValid()
	rulesFired := make(map[string]int) // track rule fires for memory

	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		ds := entry.Dataset

		// Future prediction: predict outcomes before running
		if predictor != nil {
			for _, datum := range ds.Items {
				facts := datumToFacts(datum)
				pred := predictor.Predict(facts)
				if pred.LikelyLabel != "" && cfg.Verbose {
					fmt.Printf("[Futures] Predicted %s (conf=%.2f, depth=%d) for datum in %s\n",
						pred.LikelyLabel, pred.Confidence, pred.ChainDepth, ds.Name)
				}
			}
		}

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

		// Record patterns and rule fires in memory
		domain := entry.Meta.Domain
		for _, cr := range results {
			// Record inference pattern
			featureNames := make([]string, 0)
			for k := range ds.Items[0].Features {
				featureNames = append(featureNames, k)
			}
			mem.RecordPattern(featureNames, cr.Final, cr.AUROC)

			// Track rule fires (from AR proof traces)
			for _, trace := range cr.ARResult.ProofTrace {
				if len(trace) > 5 && trace[:5] == "rule[" {
					// Extract rule ID from "rule[id]: ..."
					end := 5
					for end < len(trace) && trace[end] != ']' {
						end++
					}
					if end < len(trace) {
						ruleID := trace[5:end]
						correct := cr.Final == cr.ARResult.Prediction
						mem.RecordRuleFire(ruleID, correct, domain)
						rulesFired[ruleID]++
					}
				}
			}

			// Record future prediction outcome
			if predictor != nil {
				facts := datumToFactsFromResult(cr)
				predictor.RecordOutcome(facts, cr.Final)
			}
		}

		// Learn domain facts from results
		mem.LearnFact("dataset_accuracy", fmt.Sprintf("%.4f", batchMetrics.Accuracy), domain, "pipeline")
		mem.LearnFact("dataset_auroc", fmt.Sprintf("%.4f", batchMetrics.MeanAUROC), domain, "pipeline")

		if cfg.Verbose {
			fmt.Printf("[Pipeline] %s | Items: %d | Accuracy: %.2f%% | AUROC: %.4f | Quality: %.2f\n",
				ds.Name, batchMetrics.TotalItems, batchMetrics.Accuracy*100,
				batchMetrics.MeanAUROC, entry.Quality.OverallScore)
		}
	}

	// === Step 9: Run SuperClaude boss on first valid dataset ===
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

	// === Step 10: Recursive rule rewriting ===
	if cfg.EnableRewriter {
		rw := rewriter.NewRewriter(mem, cfg.RewriterMinFires)
		rwReport := rw.Evaluate(currentRuleSet)

		// Propose new rules from observed patterns
		topPatterns := mem.GetTopPatterns(20)
		proposals := rw.ProposeFromPatterns(topPatterns, currentRuleSet)
		for _, p := range proposals {
			rwReport.Actions = append(rwReport.Actions, rewriter.RewriteAction{
				Type:     "propose",
				RuleID:   p.ID,
				Reason:   fmt.Sprintf("proposed from pattern frequency"),
				NewRules: []ar.RuleSpec{p},
				Timestamp: time.Now(),
			})
			rwReport.Proposed++
		}

		result.RewriteReport = &rwReport

		if cfg.Verbose {
			fmt.Printf("[Rewriter] Evaluated %d rules: %d strengthened, %d weakened, %d retired, %d proposed, %d split\n",
				rwReport.Evaluated, rwReport.Strengthened, rwReport.Weakened,
				rwReport.Retired, rwReport.Proposed, rwReport.Split)
		}
	}

	// === Step 11: Record run summary in memory ===
	duration := time.Since(startTime)
	rewrites := 0
	if result.RewriteReport != nil {
		rewrites = len(result.RewriteReport.Actions)
	}
	peakAgents := 0
	if result.Swarm != nil {
		peakAgents = result.Swarm.Status().PeakAgents
	}

	mem.RecordRun(memory.RunSummary{
		Timestamp:  startTime,
		Datasets:   len(entries),
		Accuracy:   overallAccuracy(result.DatasetReports),
		RulesUsed:  len(currentRuleSet.Rules),
		RulesFired: len(rulesFired),
		DurationMs: duration.Milliseconds(),
		Rewrites:   rewrites,
		SwarmPeak:  peakAgents,
	})

	// === Step 12: Save memory ===
	if err := mem.Save(); err != nil {
		if cfg.Verbose {
			fmt.Printf("[Memory] Save error: %v\n", err)
		}
	} else if cfg.Verbose {
		fmt.Printf("[Memory] Saved: %s\n", mem.Summary())
	}

	// === Step 13: Save inferences if configured ===
	if cfg.SaveInferences && cfg.InferencesFile != "" {
		saveInferences(cfg.InferencesFile, result.DatasetReports)
	}

	// === Step 14: Generate automated report ===
	gen := report.NewGenerator()
	result.Report = gen.GenerateFullReport(result.DatasetReports, result.OrchestratorResults, boss)

	if cfg.Verbose {
		fmt.Printf("[Pipeline] Complete in %v\n", duration.Round(time.Millisecond))
	}

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

// loadRuleSet loads the rule set for rewriter/futures use.
func loadRuleSet(cfg config.Config) ar.RuleSet {
	data, err := os.ReadFile(cfg.RulesFile)
	if err != nil {
		return ar.DefaultRuleSet()
	}
	var rs ar.RuleSet
	if err := json.Unmarshal(data, &rs); err != nil {
		return ar.DefaultRuleSet()
	}
	return rs
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

// datumToFacts converts a datum's features to fact strings for the futures predictor.
func datumToFacts(d kinds.Datum) []string {
	facts := make([]string, 0, len(d.Features))
	for k, v := range d.Features {
		level := "low"
		if v > 0.5 {
			level = "high"
		}
		facts = append(facts, fmt.Sprintf("has_feature(%s, %s)", k, level))
	}
	return facts
}

// datumToFactsFromResult extracts feature facts from a composed result's proof trace.
func datumToFactsFromResult(cr kinds.ComposedResult) []string {
	var facts []string
	for _, trace := range cr.ARResult.ProofTrace {
		if len(trace) > 6 && trace[:6] == "fact: " {
			facts = append(facts, trace[6:])
		}
	}
	return facts
}

// overallAccuracy computes the average accuracy across all datasets.
func overallAccuracy(reports map[string]report.DatasetReport) float64 {
	if len(reports) == 0 {
		return 0
	}
	total := 0.0
	for _, r := range reports {
		total += r.Metrics.Accuracy
	}
	return total / float64(len(reports))
}
