// Package config defines the dynamic configuration for CARLA.
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

// Config holds all dynamic parameters for a CARLA run.
type Config struct {
	// Data
	DataDir          string   `json:"data_dir"`
	DatasetFiles     []string `json:"dataset_files"`
	FeatureThreshold float64  `json:"feature_threshold"`

	// AR Kinds
	ARKinds []string `json:"ar_kinds"` // e.g. ["logic-programs", "bayesian-lp"]

	// Rules
	RulesFile string `json:"rules_file"` // JSON file with AR rules

	// Inference Strategy
	Strategy            string  `json:"strategy"` // "ar_priority", "confidence_rank", "consensus"
	ConfidenceThreshold float64 `json:"confidence_threshold"`

	// Agents
	NumVerifiers   int      `json:"num_verifiers"`
	NumPhDs        int      `json:"num_phds"`
	PhDSpecialties []string `json:"phd_specialties"`

	// Evaluation
	VerifyTarget  float64 `json:"verify_target"`
	ExplainTarget float64 `json:"explain_target"`
	MaxProofDepth int     `json:"max_proof_depth"`

	// Output
	OutputFile     string `json:"output_file"`
	OutputFormat   string `json:"output_format"` // "text", "json"
	Verbose        bool   `json:"verbose"`
	SaveInferences bool   `json:"save_inferences"`
	InferencesFile string `json:"inferences_file"`

	// Memory
	MemoryFile string `json:"memory_file"`

	// Rewriter
	EnableRewriter   bool `json:"enable_rewriter"`
	RewriterMinFires int  `json:"rewriter_min_fires"`

	// Futures
	EnableFutures    bool `json:"enable_futures"`
	FutureCacheSize  int  `json:"future_cache_size"`
	FutureChainDepth int  `json:"future_chain_depth"`

	// Swarm
	SwarmEnabled     bool `json:"swarm_enabled"`
	MinVerifiers     int  `json:"min_verifiers"`
	MaxVerifiers     int  `json:"max_verifiers"`
	MinPhDs          int  `json:"min_phds"`
	MaxPhDs          int  `json:"max_phds"`
	MinModels        int  `json:"min_models"`
	MaxModels        int  `json:"max_models"`
	ScaleUpThreshold int  `json:"scale_up_threshold"`
	ScaleDownIdleSec int  `json:"scale_down_idle_sec"`
	MaxSwarmSize     int  `json:"max_swarm_size"`
}

// DefaultConfig returns a reasonable default configuration.
func DefaultConfig() Config {
	return Config{
		DataDir:             "data/datasets",
		DatasetFiles:        []string{},
		FeatureThreshold:    0.5,
		ARKinds:             []string{"logic-programs"},
		RulesFile:           "data/rules/default.json",
		Strategy:            "ar_priority",
		ConfidenceThreshold: 0.3,
		NumVerifiers:        2,
		NumPhDs:             2,
		PhDSpecialties:      []string{"logic-programs", "bayesian-lp"},
		VerifyTarget:        0.95,
		ExplainTarget:       0.90,
		MaxProofDepth:       10,
		OutputFile:          "carla_report.txt",
		OutputFormat:        "text",
		Verbose:             true,
		SaveInferences:      false,
		InferencesFile:      "inferences.json",

		// Memory
		MemoryFile:       "data/memory.json",
		EnableRewriter:   true,
		RewriterMinFires: 5,
		EnableFutures:    true,
		FutureCacheSize:  1000,
		FutureChainDepth: 5,
		SwarmEnabled:     true,
		MinVerifiers:     1,
		MaxVerifiers:     6,
		MinPhDs:          1,
		MaxPhDs:          4,
		MinModels:        1,
		MaxModels:        4,
		ScaleUpThreshold: 5,
		ScaleDownIdleSec: 30,
		MaxSwarmSize:     20,
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

	p.Section("CARLA Configuration")

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
	p.Section("AR Kind Selection")

	arChoices := []string{"logic-programs", "bayesian-lp", "propositional-classical"}
	cfg.ARKinds = p.AskMultiChoice("ar_kinds",
		"Select AR kinds:", arChoices, []int{0})
	if len(cfg.ARKinds) == 0 {
		p.Info("Defaulting to logic-programs.")
		cfg.ARKinds = []string{"logic-programs"}
	}

	// === Rules ===
	p.Section("Rules Configuration")
	cfg.RulesFile = p.AskString("rules_file",
		"AR rules file (JSON)", cfg.RulesFile)

	// === Strategy ===
	p.Section("Inference Strategy")
	cfg.Strategy = p.AskChoice("strategy",
		"How should inference results be resolved?",
		[]string{"ar_priority", "confidence_rank", "consensus"}, 0)
	cfg.ConfidenceThreshold = p.AskFloat("confidence_threshold",
		"Minimum confidence for verification", cfg.ConfidenceThreshold)

	// === Agents ===
	p.Section("Agent Configuration")
	cfg.NumVerifiers = p.AskInt("num_verifiers", "Number of Verifier agents", cfg.NumVerifiers)
	cfg.NumPhDs = p.AskInt("num_phds", "Number of PhD agents", cfg.NumPhDs)

	specChoices := []string{"logic-programs", "bayesian-lp", "constraint-programming"}
	cfg.PhDSpecialties = p.AskMultiChoice("phd_specialties",
		"PhD agent specialties:", specChoices, []int{0, 1})

	// === Evaluation ===
	p.Section("Evaluation Targets")
	cfg.VerifyTarget = p.AskFloat("verify_target", "Verifiability target (0.0-1.0)", cfg.VerifyTarget)
	cfg.ExplainTarget = p.AskFloat("explain_target", "Explainability target (0.0-1.0)", cfg.ExplainTarget)
	cfg.MaxProofDepth = p.AskInt("max_proof_depth", "Max proof unfolding depth (<=10)", cfg.MaxProofDepth)

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

	// === Memory ===
	p.Section("Perpetual Memory")
	cfg.MemoryFile = p.AskString("memory_file", "Memory store file (JSON)", cfg.MemoryFile)

	// === Rewriter ===
	p.Section("Recursive Rule Rewriter")
	cfg.EnableRewriter = p.AskYesNo("enable_rewriter", "Enable recursive rule rewriting?", cfg.EnableRewriter)
	if cfg.EnableRewriter {
		cfg.RewriterMinFires = p.AskInt("rewriter_min_fires",
			"Minimum rule fires before evaluating", cfg.RewriterMinFires)
	}

	// === Swarm & Futures ===
	p.Section("Swarm & Future Chaining")
	cfg.SwarmEnabled = p.AskYesNo("swarm_enabled", "Enable autoscaled swarm?", cfg.SwarmEnabled)
	if cfg.SwarmEnabled {
		cfg.MaxSwarmSize = p.AskInt("max_swarm_size", "Maximum swarm size", cfg.MaxSwarmSize)
	}
	cfg.EnableFutures = p.AskYesNo("enable_futures", "Enable future chain prediction?", cfg.EnableFutures)
	if cfg.EnableFutures {
		cfg.FutureChainDepth = p.AskInt("future_chain_depth",
			"Future chain prediction depth", cfg.FutureChainDepth)
	}

	// === Save config ===
	saveConfig := p.AskYesNo("save_config", "Save this configuration for reuse?", false)
	if saveConfig {
		configPath := p.AskString("config_path", "Config file path", "carla_config.json")
		if err := cfg.SaveToFile(configPath); err != nil {
			p.Info("Warning: could not save config: %v", err)
		} else {
			p.Info("Config saved to %s", configPath)
		}
	}

	return cfg
}
