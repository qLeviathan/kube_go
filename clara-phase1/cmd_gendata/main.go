// cmd_gendata generates the default JSON rule and BayesNet files.
// Run once: go run cmd_gendata/main.go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/clara-phase1/ar"
	"github.com/clara-phase1/ml"
)

func main() {
	// Generate default rules file
	rules := ar.DefaultRuleSet()
	rulesJSON, _ := json.MarshalIndent(rules, "", "  ")
	os.WriteFile("data/rules/default.json", rulesJSON, 0644)
	fmt.Println("Generated data/rules/default.json")

	// Generate default BayesNet file
	bn := ml.DefaultBayesNetSpec()
	bnJSON, _ := json.MarshalIndent(bn, "", "  ")
	os.WriteFile("data/models/default.json", bnJSON, 0644)
	fmt.Println("Generated data/models/default.json")
}
