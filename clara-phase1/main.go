// CARLA -- Compositional Autonomous Reasoning & Learning Architecture
// Self-evolving rules engine with agent orchestration,
// recursive rule rewriting, future chaining, and autoscaled swarm.
//
// Usage:
//   go run main.go                    # Interactive mode (questionnaire)
//   go run main.go --auto             # Auto mode (all defaults)
//   go run main.go --config file.json # Load config from file
//   go run main.go --genconfig        # Generate default config file
//   go run main.go --gendata          # Generate default rule files
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/clara-phase1/ar"
	"github.com/clara-phase1/cli"
	"github.com/clara-phase1/config"
	"github.com/clara-phase1/pipeline"
	"github.com/clara-phase1/report"
)

func main() {
	fmt.Println("================================================================")
	fmt.Println("  CARLA -- Autonomous Intelligence Platform")
	fmt.Println("  Self-Evolving Rules Engine | Future Chaining | Swarm")
	fmt.Println("  Boss: SuperClaude Agent | Dynamic Configuration")
	fmt.Println("================================================================")
	fmt.Println()

	cfg := resolveConfig()

	result := pipeline.Run(cfg)

	// Output report
	formatted := report.FormatReport(result.Report)
	fmt.Println(formatted)

	// Write report to file
	if err := os.WriteFile(cfg.OutputFile, []byte(formatted), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write report: %v\n", err)
	} else {
		fmt.Printf("[Output] Report written to %s\n", cfg.OutputFile)
	}

	// Print data agent summary
	if result.Registry != nil {
		fmt.Printf("\nData Filing Summary:\n")
		fmt.Print(result.Registry.Summary())
	}

	fmt.Printf("\nAgents deployed by SuperClaude boss:\n")
	fmt.Printf("  Verifiers: %d\n", len(result.SuperClaude.Verifiers))
	fmt.Printf("  PhDs:      %d\n", len(result.SuperClaude.PhDs))
	fmt.Printf("  Models:    %d\n", len(result.SuperClaude.Models))
	fmt.Printf("  Boss log:  %d entries\n", len(result.SuperClaude.Log))
	fmt.Printf("  Datasets:  %d\n", len(result.DatasetReports))
	fmt.Printf("  Results:   %d\n", len(result.OrchestratorResults))

	// Memory summary
	if result.Memory != nil {
		fmt.Printf("\nPerpetual Memory:\n")
		fmt.Printf("  %s\n", result.Memory.Summary())
		fmt.Printf("  Total runs: %d\n", result.Memory.TotalRuns())
	}

	// Rewriter summary
	if result.RewriteReport != nil {
		rr := result.RewriteReport
		fmt.Printf("\nRecursive Rule Rewriter:\n")
		fmt.Printf("  Evaluated:    %d rules\n", rr.Evaluated)
		fmt.Printf("  Strengthened: %d\n", rr.Strengthened)
		fmt.Printf("  Weakened:     %d\n", rr.Weakened)
		fmt.Printf("  Retired:      %d\n", rr.Retired)
		fmt.Printf("  Proposed:     %d\n", rr.Proposed)
		fmt.Printf("  Split:        %d\n", rr.Split)
	}

	// Swarm summary
	if result.Swarm != nil {
		status := result.Swarm.Status()
		fmt.Printf("\nSwarm Status:\n")
		fmt.Printf("  Total agents:    %d\n", status.TotalAgents)
		fmt.Printf("  Peak agents:     %d\n", status.PeakAgents)
		fmt.Printf("  Tasks completed: %d\n", status.CompletedTasks)
		for role, count := range status.AgentsByRole {
			fmt.Printf("  %-16s %d\n", role+":", count)
		}
	}

	// Future predictor summary
	if result.FuturePredictor != nil {
		fmt.Printf("\nFuture Chain Predictor:\n")
		fmt.Printf("  %s\n", result.FuturePredictor.Summary())
	}
}

func resolveConfig() config.Config {
	args := os.Args[1:]

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--genconfig":
			cfg := config.DefaultConfig()
			cfg.SaveToFile("carla_config.json")
			fmt.Println("Generated carla_config.json")
			os.Exit(0)

		case "--gendata":
			rulesJSON, _ := json.MarshalIndent(ar.DefaultRuleSet(), "", "  ")
			os.WriteFile("data/rules/default.json", rulesJSON, 0644)
			fmt.Println("Generated data/rules/default.json")
			os.Exit(0)

		case "--auto":
			fmt.Println("[Mode] Auto — using all defaults")
			return config.DefaultConfig()

		case "--config":
			if i+1 < len(args) {
				path := args[i+1]
				cfg, err := config.LoadFromFile(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("[Mode] Config loaded from %s\n", path)
				return cfg
			}
			fmt.Fprintln(os.Stderr, "Error: --config requires a file path")
			os.Exit(1)

		case "--help", "-h":
			printUsage()
			os.Exit(0)
		}
	}

	// Default: interactive mode
	fmt.Println("[Mode] Interactive — answer questions to configure the run")
	fmt.Println("  (Press Enter to accept defaults, shown in brackets)")
	fmt.Println()
	p := cli.NewPrompter()
	return config.BuildInteractive(p)
}

func printUsage() {
	fmt.Println(`CARLA — Autonomous Intelligence Platform

Usage:
  go run main.go                    Interactive mode (questionnaire)
  go run main.go --auto             Auto mode (all defaults)
  go run main.go --config file.json Load config from JSON file
  go run main.go --genconfig        Generate default config file
  go run main.go --gendata          Generate default rule JSON files
  go run main.go --help             Show this help

Data Files:
  Place CSV datasets in data/datasets/ (auto-discovered)
  Place AR rules in data/rules/ (JSON format)

CSV Format:
  feature1,feature2,...,label
  0.8,0.5,...,class_A

Config File:
  Run --genconfig to see all available options.`)
}
