// CLARA Phase 1 -- Compositional Learning-And-Reasoning for AI
// DARPA-PA-25-07-02 Disruption Opportunity
//
// Dynamic CLI-driven system. All configuration via interactive questionnaire,
// JSON config files, or CLI flags. No hardcoded parameters.
//
// Usage:
//   go run main.go                    # Interactive mode (questionnaire)
//   go run main.go --auto             # Auto mode (all defaults)
//   go run main.go --config file.json # Load config from file
//   go run main.go --genconfig        # Generate default config file
//   go run main.go --gendata          # Generate default rule/model files
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/clara-phase1/ar"
	"github.com/clara-phase1/cli"
	"github.com/clara-phase1/config"
	"github.com/clara-phase1/ml"
	"github.com/clara-phase1/pipeline"
	"github.com/clara-phase1/report"
)

func main() {
	fmt.Println("================================================================")
	fmt.Println("  CLARA Phase 1 -- AR+ML Composed Inference System")
	fmt.Println("  DARPA-PA-25-07-02 Disruption Opportunity")
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
}

func resolveConfig() config.Config {
	args := os.Args[1:]

	// Check for special commands
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--genconfig":
			cfg := config.DefaultConfig()
			cfg.SaveToFile("clara_config.json")
			fmt.Println("Generated clara_config.json")
			os.Exit(0)

		case "--gendata":
			rulesJSON, _ := json.MarshalIndent(ar.DefaultRuleSet(), "", "  ")
			os.WriteFile("data/rules/default.json", rulesJSON, 0644)
			fmt.Println("Generated data/rules/default.json")

			bnJSON, _ := json.MarshalIndent(ml.DefaultBayesNetSpec(), "", "  ")
			os.WriteFile("data/models/default.json", bnJSON, 0644)
			fmt.Println("Generated data/models/default.json")
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
	fmt.Println(`CLARA Phase 1 — Usage:

  go run main.go                    Interactive mode (questionnaire)
  go run main.go --auto             Auto mode (all defaults)
  go run main.go --config file.json Load config from JSON file
  go run main.go --genconfig        Generate default config file
  go run main.go --gendata          Generate default rule/model JSON files
  go run main.go --help             Show this help

Data Files:
  Place CSV datasets in data/datasets/ (auto-discovered)
  Place AR rules in data/rules/ (JSON format)
  Place BayesNet models in data/models/ (JSON format)

CSV Format:
  feature1,feature2,...,label
  0.8,0.5,...,class_A

Config File:
  Run --genconfig to see all available options.`)
}
