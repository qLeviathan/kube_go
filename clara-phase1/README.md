# CLARA Phase 1 -- Compositional Learning-And-Reasoning for AI

**DARPA-PA-25-07-02 Disruption Opportunity**

A fully dynamic, CLI-driven Go system that composes Automated Reasoning (Logic Programs) with Machine Learning (Bayesian Networks) for high-assurance AI inference. Built from scratch with no recursion, per-inference state isolation, and automated data filing agents.

---

## Prerequisites

### Required

- **Go 1.21+** (tested with Go 1.24)
- **Git** (to clone the repository)

### Install Go

**Linux (Ubuntu/Debian):**
```bash
sudo apt update
sudo apt install -y golang-go

# Or install latest from source:
wget https://go.dev/dl/go1.24.7.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.7.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

**macOS:**
```bash
brew install go
```

**Windows:**
Download from https://go.dev/dl/ and run the installer.

### Verify Installation
```bash
go version
# Should print: go version go1.21+ ...
```

### No External Dependencies

This project uses **only the Go standard library**. No `go get` or dependency downloads needed. Just clone and run.

---

## Quick Start

```bash
# Clone the repo
git clone https://github.com/qLeviathan/kube_go.git
cd kube_go/clara-phase1

# Run in auto mode (all defaults, no questions)
go run main.go --auto

# Run interactively (answer configuration questions)
go run main.go

# Run tests
go test ./... -v
```

---

## Usage Modes

### 1. Interactive Mode (default)
```bash
go run main.go
```
Walks you through a questionnaire to configure every parameter:
- Which datasets to load
- ML and AR kind selection
- Composition strategy and weights
- Agent counts and specialties
- Metric targets
- Output format

Press **Enter** at any prompt to accept the default (shown in brackets).

### 2. Auto Mode
```bash
go run main.go --auto
```
Runs with all defaults. No questions asked.

### 3. Config File Mode
```bash
# Generate a default config file
go run main.go --genconfig

# Edit it
nano clara_config.json

# Run with your config
go run main.go --config clara_config.json
```

### 4. Generate Data Files
```bash
go run main.go --gendata
```
Creates default `data/rules/default.json` and `data/models/default.json`.

---

## Project Structure

```
clara-phase1/
|-- main.go                 # Entry point: CLI routing, --auto, --config, --interactive
|
|-- cli/                    # Interactive questionnaire system
|   +-- cli.go              # Prompter: AskString, AskFloat, AskChoice, AskMultiChoice, AskYesNo
|
|-- config/                 # Dynamic configuration
|   +-- config.go           # Config struct, LoadFromFile, SaveToFile, BuildInteractive
|
|-- kinds/                  # ML/AR kind taxonomy and data types
|   |-- kinds.go            # Kind registry, ValidCategory, MLKinds, ARKinds
|   +-- model.go            # Datum, DataSet, ModelResult, ComposedResult, Metric
|
|-- agents/                 # Agent framework (SuperClaude boss + sub-agents)
|   +-- agents.go           # SuperClaudeAgent, VerifierAgent, PhDAgent, ModelAgent
|
|-- ar/                     # Automated Reasoning engine
|   |-- engine.go           # Logic Programs with iterative forward-chaining
|   +-- loader.go           # Load rules from JSON files (RuleSet, LoadRulesFromFile)
|
|-- ml/                     # Machine Learning engine
|   |-- engine.go           # Bayesian Networks with iterative belief propagation
|   +-- loader.go           # Load BayesNet from JSON files (BayesNetSpec, LoadBayesNetFromFile)
|
|-- compose/                # AR+ML composition pipeline
|   +-- compose.go          # Pipeline, 3 strategies, BatchMetrics, Phase1 metric evaluation, AUROC
|
|-- explain/                # Explainability and proof system
|   +-- explain.go          # Hierarchical natural-deduction proofs, unfolding <= 10
|
|-- dataagents/             # Autonomous data filing agents
|   |-- discovery.go        # DiscoveryAgent: scans directories, inspects CSV/JSON, infers domain
|   |-- loader.go           # LoaderAgent: parses CSV/JSON into kinds.DataSet
|   |-- validator.go        # ValidatorAgent: checks completeness, labels, value ranges
|   |-- quality.go          # QualityAgent: outlier detection (IQR), class balance, distributions
|   +-- registry.go         # RegistryAgent: central catalog, coordinates all data agents
|
|-- report/                 # Automated report generation
|   +-- report.go           # Full Phase 1 report with dynamic DARPA compliance checking
|
|-- pipeline/               # End-to-end orchestration
|   +-- pipeline.go         # Wires config + agents + data + engines + report
|
|-- testdata/               # Built-in fallback datasets
|   +-- datasets.go         # Medical, COA, Supply Chain test sets
|
|-- data/                   # Dynamic data directory (auto-discovered)
|   |-- datasets/           # Drop CSV/JSON files here
|   |   |-- medical.csv
|   |   |-- coa.csv
|   |   +-- supply_chain.csv
|   |-- rules/              # AR rule definitions
|   |   +-- default.json
|   +-- models/             # BayesNet model definitions
|       +-- default.json
|
+-- cmd_gendata/            # Utility to regenerate default data files
    +-- main.go
```

---

## How It Works

### Pipeline Flow

```
1. CLI/Config       User answers questions or loads config file
       |
2. SuperClaude      Boss agent initializes, deploys sub-agents
       |
3. Data Filing      DiscoveryAgent scans data/ directory
       |            LoaderAgent parses CSV/JSON files
       |            ValidatorAgent checks data quality
       |            QualityAgent assesses outliers, balance
       |            RegistryAgent catalogs everything
       |
4. Inference        For each dataset, for each datum:
       |              ML Engine (Bayesian Network) infers
       |              AR Engine (Logic Programs) infers
       |              Composer combines results
       |
5. Verification     VerifierAgent checks soundness, completeness
       |            PhDAgent reviews domain appropriateness
       |
6. Report           Automated report with Phase 1 metrics,
                    proofs, agent logs, DARPA compliance
```

### Agent Architecture

| Agent | Role | What It Does |
|-------|------|-------------|
| **SuperClaude** | Boss | Orchestrates everything, dispatches work, makes final decisions |
| **VerifierAgent** | Verifier | Checks proof soundness, completeness, confidence ranges, unfolding depth |
| **PhDAgent** | Domain Expert | Recommends ML/AR kinds, reviews composed results |
| **ModelAgent** | Inference | Wraps an InferenceEngine, processes datums |
| **DiscoveryAgent** | Data Filing | Scans directories, inspects file schemas |
| **LoaderAgent** | Data Filing | Parses CSV/JSON into typed datasets |
| **ValidatorAgent** | Data Filing | Checks completeness, labels, value ranges |
| **QualityAgent** | Data Filing | Outlier detection (IQR), class balance, distributions |
| **RegistryAgent** | Data Filing | Central catalog, coordinates all data agents |

### State Isolation

Every inference call creates fresh internal state. No facts, beliefs, or derived atoms persist between datums. This is the key architectural fix:

- **AR Engine**: `inferState` struct created per `Infer()` call
- **ML Engine**: `beliefState` struct created per `Infer()` call
- No manual `Reset()` calls needed anywhere

---

## Adding Your Own Data

### CSV Format

Place CSV files in `data/datasets/`. They are auto-discovered.

```csv
feature1,feature2,feature3,label
0.8,0.5,0.7,class_A
0.2,0.9,0.3,class_B
```

Rules:
- First row: headers (feature names + last column is `label`)
- Feature values: floating point numbers (ideally 0.0 to 1.0)
- Last column: classification label (string)

### JSON Dataset Format

```json
[
  {"feature1": 0.8, "feature2": 0.5, "label": "class_A"},
  {"feature1": 0.2, "feature2": 0.9, "label": "class_B"}
]
```

### Custom AR Rules

Edit `data/rules/default.json` or create a new file:

```json
{
  "name": "my-rules",
  "domain": "my-domain",
  "rules": [
    {
      "id": "rule-1",
      "head": {"predicate": "conclusion_A"},
      "body": [
        {"predicate": "has_feature", "args": ["feature1", "high"]},
        {"predicate": "has_feature", "args": ["feature2", "low"]}
      ]
    }
  ],
  "labels": [
    {"atom": "conclusion_A", "label": "class_A"}
  ]
}
```

### Custom BayesNet

Edit `data/models/default.json` or create a new file:

```json
{
  "name": "my-model",
  "nodes": [
    {"name": "feature1", "states": ["high", "low"]},
    {"name": "output", "states": ["high", "low"], "parents": ["feature1"]}
  ],
  "priors": [
    {"node": "feature1", "state": "high", "probability": 0.5},
    {"node": "feature1", "state": "low", "probability": 0.5}
  ],
  "cpts": [
    {"node": "output", "parent_state": "high", "node_state": "high", "probability": 0.8},
    {"node": "output", "parent_state": "high", "node_state": "low", "probability": 0.2}
  ],
  "labels": [
    {"state": "high", "label": "class_A"},
    {"state": "low", "label": "class_B"}
  ]
}
```

---

## Configuration Reference

Generate a full config file with `go run main.go --genconfig`. Key fields:

| Field | Default | Description |
|-------|---------|-------------|
| `data_dir` | `data/datasets` | Directory to scan for CSV/JSON datasets |
| `feature_threshold` | `0.5` | Cutoff for discretizing features into high/low |
| `ml_kinds` | `["bayes-nets"]` | ML kinds to compose |
| `ar_kinds` | `["logic-programs"]` | AR kinds to compose |
| `rules_file` | `data/rules/default.json` | JSON file with AR rules |
| `bayesnet_file` | `data/models/default.json` | JSON file with BayesNet structure |
| `strategy` | `ar_priority` | Composition strategy: `ar_priority`, `weighted_fusion`, `consensus` |
| `ar_weight` | `0.7` | AR weight in composition |
| `ml_weight` | `0.3` | ML weight in composition |
| `num_verifiers` | `2` | Number of Verifier agents |
| `num_phds` | `2` | Number of PhD agents |
| `soa_auroc` | `0.60` | State-of-the-art AUROC baseline |
| `verify_target` | `0.95` | Verifiability target |
| `explain_target` | `0.90` | Explainability target |
| `max_proof_depth` | `10` | Max proof unfolding depth (DARPA: <=10) |
| `output_file` | `clara_phase1_report.txt` | Report output file |
| `save_inferences` | `false` | Save all inference results to JSON |

---

## Phase 1 Metrics (DARPA Figure 1)

| Metric | Target | Implementation |
|--------|--------|---------------|
| Verifiability | Fully verifiable, error <= SOA | Automatic proof soundness/completeness checks |
| Kind Multiplicity | >=1 ML & >=1 AR | Bayesian Networks + Logic Programs |
| Polynomial Inferencing | Polynomial time | Forward-chaining O(R*F^B), belief propagation O(N*S^P) |
| Composed AUROC > SOA | > SOA baseline | Three composition strategies |
| Logical Explainability | Hierarchical, natural deduction, <=10 unfolding | ProofBuilder with caps |

---

## Running Tests

```bash
# Run all tests
go test ./... -v

# Run a specific package's tests
go test ./ar/... -v
go test ./dataagents/... -v

# Run with race detection
go test ./... -race

# Count tests
go test ./... -v 2>&1 | grep -c "^--- PASS"
```

Current test count: **50+ tests** covering:
- AR forward-chaining and state isolation
- ML belief propagation and state isolation
- Agent message passing and boss orchestration
- Data discovery, loading, validation, quality assessment
- Rule and BayesNet JSON loading
- Config save/load and interactive override
- Composition strategies and AUROC computation
- Report generation and dynamic compliance checking

---

## Design Principles

1. **No recursion** — All algorithms use iterative loops (worklists, topological order)
2. **Per-inference isolation** — Fresh state per `Infer()` call, no state leaks
3. **Dynamic everything** — Config, data, rules, models all loaded at runtime
4. **Agent-based** — SuperClaude boss coordinates Verifier, PhD, Model, and Data agents
5. **Phase 1 only** — No training/sample complexity (Phase 2 scope)
6. **DARPA compliant** — Metrics, proofs, and compliance checked against actual results
