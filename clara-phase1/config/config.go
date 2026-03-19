// Package config defines the dynamic configuration for CLARA Phase 1.
// All pipeline parameters are configurable — nothing is hardcoded.
// Config can be built interactively via CLI questionnaire, loaded from
// JSON file, or constructed programmatically for tests.
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/clara-phase1/cli"
)

// Config holds all dynamic parameters for a CLARA Phase 1 run.
type Config struct {
	// Data
	DataDir        string   `json:"data_dir"`
	DatasetFiles   []string `json:"dataset_files"`   // CSV filenames to load
	FeatureThreshold float64 `json:"feature_threshold"` // discretization cutoff

	// Kinds
	MLKinds []string `json:"ml_kinds"` // e.g. ["bayes-nets", "decision-tree"]
	ARKinds []string `json:"ar_kinds"` // e.g. ["logic-programs", "bayesian-lp"]

	// Rules & Models
	RulesFile    string `json:"rules_file"`    // JSON file with AR rules
	BayesNetFile string `json:"bayesnet_file"` // JSON file with BayesNet structure

	// Composition
	Strategy          string  `json:"strategy"` // "ar_priority", "weighted_fusion", "consensus"
	ARWeight          float64 `json:"ar_weight"`
	MLWeight          float64 `json:"ml_weight"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`

	// Agents
	NumVerifiers int      `json:"num_verifiers"`
	NumPhDs      int      `json:"num_phds"`
	PhDSpecialties []string `json:"phd_specialties"`

	// Metrics
	SOAAUROC       float64 `json:"soa_auroc"`
	VerifyTarget   float64 `json:"verify_target"`
	ExplainTarget  float64 `json:"explain_target"`
	ErrorTolerance float64 `json:"error_tolerance"`
	MaxProofDepth  int     `json:"max_proof_depth"`

	// Output
	OutputFile   string `json:"output_file"`
	OutputFormat string `json:"output_format"` // "text", "json"
	Verbose      bool   `json:"verbose"`
	SaveInferences bool `json:"save_inferences"`
	InferencesFile string `json:"inferences_file"`
}

// DefaultConfig returns a reasonable default configuration.
func DefaultConfig() Config {
	return Config{
		DataDir:            "data/datasets",
		DatasetFiles:       []string{},
		FeatureThreshold:   0.5,
		MLKinds:            []string{"bayes-nets"},
		ARKinds:            []string{"logic-programs"},
		RulesFile:          "data/rules/default.json",
		BayesNetFile:       "data/models/default.json",
		Strategy:           "ar_priority",
		ARWeight:           0.7,
		MLWeight:           0.3,
		ConfidenceThreshold: 0.3,
		NumVerifiers:       2,
		NumPhDs:            2,
		PhDSpecialties:     []string{"bayesian-lp", "logic-programs"},
		SOAAUROC:           0.60,
		VerifyTarget:       0.95,
		ExplainTarget:      0.90,
		ErrorTolerance:     0.05,
		MaxProofDepth:      10,
		OutputFile:         "clara_phase1_report.txt",
		OutputFormat:       "text",
		Verbose:            true,
		SaveInferences:     false,
		InferencesFile:     "inferences.json",
	}
}

// LoadFromFile reads config from a JSON file.
func LoadFromFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// SaveToFile writes config to a JSON file.
func (c Config) SaveToFile(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// BuildInteractive runs the full CLI questionnaire to build a config.
func BuildInteractive(p *cli.Prompter) Config {
	cfg := DefaultConfig()

	p.Section("CLARA Phase 1 Configuration")

	// === Data ===
	p.Section("Data Sources")
	cfg.DataDir = p.AskString("data_dir", "Data directory", cfg.DataDir)
	cfg.FeatureThreshold = p.AskFloat("feature_threshold",
		"Feature discretization threshold (features > threshold = 'high')", cfg.FeatureThreshold)

	loadFromFiles := p.AskYesNo("load_files", "Load datasets from CSV files in data directory?", true)
	if !loadFromFiles {
		p.Info("Will use built-in dummy datasets")
		cfg.DatasetFiles = []string{}
	}

	// === Kind Selection ===
	p.Section("AI Kind Selection (Phase 1: >=1 ML + >=1 AR)")

	mlChoices := []string{"bayes-nets", "decision-tree", "bayesian"}
	cfg.MLKinds = p.AskMultiChoice("ml_kinds",
		"Select ML kinds to compose:", mlChoices, []int{0})
	if len(cfg.MLKinds) == 0 {
		p.Info("Phase 1 requires >=1 ML kind. Defaulting to bayes-nets.")
		cfg.MLKinds = []string{"bayes-nets"}
	}

	arChoices := []string{"logic-programs", "bayesian-lp", "propositional-classical"}
	cfg.ARKinds = p.AskMultiChoice("ar_kinds",
		"Select AR kinds to compose:", arChoices, []int{0})
	if len(cfg.ARKinds) == 0 {
		p.Info("Phase 1 requires >=1 AR kind. Defaulting to logic-programs.")
		cfg.ARKinds = []string{"logic-programs"}
	}

	// === Rules & Models ===
	p.Section("Rules & Model Configuration")
	cfg.RulesFile = p.AskString("rules_file",
		"AR rules file (JSON)", cfg.RulesFile)
	cfg.BayesNetFile = p.AskString("bayesnet_file",
		"BayesNet structure file (JSON)", cfg.BayesNetFile)

	// === Composition ===
	p.Section("Composition Strategy")
	cfg.Strategy = p.AskChoice("strategy",
		"How should ML and AR results be composed?",
		[]string{"ar_priority", "weighted_fusion", "consensus"}, 0)
	cfg.ARWeight = p.AskFloat("ar_weight", "AR weight in composition", cfg.ARWeight)
	cfg.MLWeight = p.AskFloat("ml_weight", "ML weight in composition", cfg.MLWeight)
	cfg.ConfidenceThreshold = p.AskFloat("confidence_threshold",
		"Minimum confidence for verification", cfg.ConfidenceThreshold)

	// === Agents ===
	p.Section("Agent Configuration")
	cfg.NumVerifiers = p.AskInt("num_verifiers", "Number of Verifier agents", cfg.NumVerifiers)
	cfg.NumPhDs = p.AskInt("num_phds", "Number of PhD agents", cfg.NumPhDs)

	specChoices := []string{"bayesian-lp", "logic-programs", "neural-symbolic", "constraint-programming"}
	cfg.PhDSpecialties = p.AskMultiChoice("phd_specialties",
		"PhD agent specialties:", specChoices, []int{0, 1})

	// === Metrics ===
	p.Section("Phase 1 Metric Targets (per DARPA-PA-25-07-02)")
	cfg.SOAAUROC = p.AskFloat("soa_auroc", "State-of-the-art AUROC baseline", cfg.SOAAUROC)
	cfg.VerifyTarget = p.AskFloat("verify_target", "Verifiability target (0.0-1.0)", cfg.VerifyTarget)
	cfg.ExplainTarget = p.AskFloat("explain_target", "Explainability target (0.0-1.0)", cfg.ExplainTarget)
	cfg.ErrorTolerance = p.AskFloat("error_tolerance", "Error rate tolerance above SOA", cfg.ErrorTolerance)
	cfg.MaxProofDepth = p.AskInt("max_proof_depth", "Max proof unfolding depth (DARPA: <=10)", cfg.MaxProofDepth)

	// === Output ===
	p.Section("Output Configuration")
	cfg.OutputFile = p.AskString("output_file", "Report output file", cfg.OutputFile)
	cfg.OutputFormat = p.AskChoice("output_format", "Output format:",
		[]string{"text", "json"}, 0)
	cfg.Verbose = p.AskYesNo("verbose", "Verbose logging?", cfg.Verbose)
	cfg.SaveInferences = p.AskYesNo("save_inferences", "Save all inference results to file?", cfg.SaveInferences)
	if cfg.SaveInferences {
		cfg.InferencesFile = p.AskString("inferences_file", "Inferences output file", cfg.InferencesFile)
	}

	// === Save config ===
	saveConfig := p.AskYesNo("save_config", "Save this configuration for reuse?", false)
	if saveConfig {
		configPath := p.AskString("config_path", "Config file path", "clara_config.json")
		if err := cfg.SaveToFile(configPath); err != nil {
			p.Info("Warning: could not save config: %v", err)
		} else {
			p.Info("Config saved to %s", configPath)
		}
	}

	return cfg
}
