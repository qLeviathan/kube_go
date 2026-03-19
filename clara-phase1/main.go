// CLARA Phase 1 — Compositional Learning-And-Reasoning for AI
// DARPA-PA-25-07-02 Disruption Opportunity
//
// This is the main entry point that runs the full Phase 1 pipeline:
// 1. Deploys Verifier, PhD, and Model agents
// 2. Runs AR (Logic Programs) + ML (Bayesian Networks) composition
// 3. Evaluates on dummy test datasets (Medical, COA, Supply Chain)
// 4. Generates automated output report with all Phase 1 metrics
//
// No recursion used anywhere in the codebase.
package main

import (
	"fmt"
	"os"

	"github.com/clara-phase1/pipeline"
	"github.com/clara-phase1/report"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  CLARA Phase 1 — AR+ML Composed Inference System           ║")
	fmt.Println("║  DARPA-PA-25-07-02 Disruption Opportunity                  ║")
	fmt.Println("║  Kinds: Logic Programs (AR) + Bayesian Networks (ML)       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
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
	fmt.Printf("\nAgents deployed:\n")
	fmt.Printf("  Verifiers: %d\n", len(result.Verifiers))
	fmt.Printf("  PhDs:      %d\n", len(result.PhDs))
	fmt.Printf("  Models:    %d\n", len(result.Models))
	fmt.Printf("  Datasets evaluated: %d\n", len(result.DatasetReports))
	fmt.Printf("  Orchestrator results: %d\n", len(result.OrchestratorResults))
}
