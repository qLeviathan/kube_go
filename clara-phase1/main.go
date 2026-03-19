// CLARA Phase 1 -- Compositional Learning-And-Reasoning for AI
// DARPA-PA-25-07-02 Disruption Opportunity
//
// Runs the full Phase 1 pipeline via the SuperClaude boss agent:
// 1. SuperClaude deploys Verifier, PhD, and Model agents
// 2. AR (Logic Programs) + ML (Bayesian Networks) composed inference
// 3. Evaluates on dummy test datasets (Medical, COA, Supply Chain)
// 4. Generates automated output report with all Phase 1 metrics
//
// No recursion. All inference state is isolated per-datum.
package main

import (
	"fmt"
	"os"

	"github.com/clara-phase1/pipeline"
	"github.com/clara-phase1/report"
)

func main() {
	fmt.Println("================================================================")
	fmt.Println("  CLARA Phase 1 -- AR+ML Composed Inference System")
	fmt.Println("  DARPA-PA-25-07-02 Disruption Opportunity")
	fmt.Println("  AR Kind: Logic Programs  |  ML Kind: Bayesian Networks")
	fmt.Println("  Boss: SuperClaude Agent")
	fmt.Println("================================================================")
	fmt.Println()

	cfg := pipeline.DefaultConfig()
	result := pipeline.Run(cfg)

	// Print the full automated report
	formatted := report.FormatReport(result.Report)
	fmt.Println(formatted)

	// Write report to file
	err := os.WriteFile("clara_phase1_report.txt", []byte(formatted), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write report file: %v\n", err)
	} else {
		fmt.Println("[Output] Report written to clara_phase1_report.txt")
	}

	// Summary
	fmt.Printf("\nAgents deployed by SuperClaude boss:\n")
	fmt.Printf("  Verifiers: %d\n", len(result.SuperClaude.Verifiers))
	fmt.Printf("  PhDs:      %d\n", len(result.SuperClaude.PhDs))
	fmt.Printf("  Models:    %d\n", len(result.SuperClaude.Models))
	fmt.Printf("  Boss log entries: %d\n", len(result.SuperClaude.Log))
	fmt.Printf("  Datasets evaluated: %d\n", len(result.DatasetReports))
	fmt.Printf("  Orchestrator results: %d\n", len(result.OrchestratorResults))
}
