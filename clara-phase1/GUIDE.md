# CARLA: Hybrid AI Memory System — Build Guide

Reproduce this project from scratch. Zero dependencies. Single Go binary.

---

## What This Is

CARLA is a **hybrid AI memory system** disguised as a rules engine. It has four memory layers:

| Layer | What | Where | Lifespan |
|-------|-------|-------|----------|
| **Immediate** | Per-datum inference state | `ar.Engine.Infer()` | One inference call |
| **Working** | Future prediction cache | `futures.Predictor.Cache` | One run |
| **Episodic** | Facts, patterns, rule stats | `memory.Store` (JSON file) | Across all runs |
| **Crystallized** | The rules themselves | `ar.RuleSet` → rewriter evolves them | Permanent, self-modifying |

The system *learns* by cycling through these layers:
1. Rules fire on data (immediate)
2. Outcomes get cached for prediction (working)
3. Patterns and performance get recorded (episodic)
4. The rewriter reads episodic memory and modifies the rules (crystallized)
5. Next run starts with better rules and warm caches

This is the loop that makes it a living system, not a static engine.

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                    CARLA Pipeline                     │
│                                                       │
│  ┌─────────┐   ┌──────────┐   ┌───────────────────┐ │
│  │ Memory   │──▶│ Futures  │──▶│  AR Engine        │ │
│  │ Store    │   │ Predictor│   │  (forward-chain)  │ │
│  │ (JSON)   │◀──│ (cache)  │◀──│  (isolated state) │ │
│  └────┬─────┘   └──────────┘   └───────────────────┘ │
│       │                                               │
│       ▼                                               │
│  ┌─────────┐   ┌──────────┐   ┌───────────────────┐ │
│  │ Rewriter │──▶│ Swarm    │──▶│  Agents           │ │
│  │ (meta-   │   │ Controller│  │  Boss → Verifier  │ │
│  │  rules)  │   │ (mesh)   │   │       → PhD       │ │
│  └──────────┘   └──────────┘   │       → Model     │ │
│                                 └───────────────────┘ │
└─────────────────────────────────────────────────────┘
```

**Data flow**: CSV/data in → data agents validate → AR engine infers → verifiers check proofs → memory records outcomes → rewriter evolves rules → next run is smarter.

---

## Reproduce From Scratch

### Prerequisites

- Go 1.21+ (any recent version)
- That's it. Zero external dependencies.

### Step 0: Project Skeleton

```bash
mkdir carla && cd carla
go mod init github.com/carla
```

Create this directory structure:
```
carla/
├── main.go
├── ar/           # Automated Reasoning engine
├── kinds/        # Shared types
├── agents/       # Agent framework
├── memory/       # Persistent learning store
├── rewriter/     # Rule evolution engine
├── futures/      # Predictive cache
├── swarm/        # Agent orchestration
├── dataagents/   # Data pipeline agents
├── config/       # Dynamic configuration
├── cli/          # Interactive questionnaire
├── pipeline/     # Orchestration
├── explain/      # Proof system
├── report/       # Report generation
├── testdata/     # Built-in datasets
└── data/         # Runtime data
    ├── rules/
    ├── datasets/
    └── memory.json  (auto-created)
```

### Step 1: Shared Types (`kinds/`)

Build bottom-up. `kinds/` has zero internal imports — everything else depends on it.

**`kinds/kinds.go`** — Define what reasoning techniques exist:
```go
type Kind struct {
    ID     string
    Name   string
    Parent string
}

var KindLogicPrograms = Kind{ID: "ar-lp", Name: "Logic Programs"}
```

**`kinds/model.go`** — Define data and results:
```go
// Input
type Datum struct {
    Features map[string]float64
    Label    string
}
type DataSet struct {
    Name  string
    Items []Datum
}

// Output from a single inference
type ModelResult struct {
    Prediction string
    Confidence float64
    Kind       Kind
    ProofTrace []string  // every decision has a proof
}

// Output from the full pipeline
type InferenceResult struct {
    Result     ModelResult
    Final      string
    Verified   bool
    Confidence float64
    Explained  bool
}
```

**Key design decision**: `InferenceResult` replaces what was once a "composed ML+AR result." We stripped ML entirely. Intelligence comes from rule evolution + memory, not statistical models.

### Step 2: Automated Reasoning Engine (`ar/`)

The core inference engine. Forward-chaining logic programs.

**`ar/engine.go`** — The brain:
```go
type RuleSpec struct {
    ID   string
    Head AtomSpec        // what this rule concludes
    Body []AtomSpec      // what must be true for it to fire
}

type Engine struct {
    Rules  []rule
    Labels map[string]string  // atom → human label
}

func (e *Engine) Infer(datum Datum) (ModelResult, error)
```

**How inference works** (iterative, never recursive):
1. Convert datum features to facts: `{blood_pressure: 0.8}` → `has_feature(blood_pressure, high)`
2. Put all facts in a worklist
3. Loop: for each rule, check if all body atoms are satisfied
4. If satisfied, derive the head atom, add to worklist
5. Continue until no new atoms derived (fixpoint)
6. Map derived atoms to labels via the label table
7. Return prediction + confidence + proof trace

**Critical**: Each `Infer()` call creates fresh state. No state bleeds between calls. This is what makes it safe for concurrent swarm execution.

**`ar/loader.go`** — Load rules from JSON files:
```go
func LoadRulesFromFile(path string) (*Engine, error)
func LoadRulesFromJSON(data []byte) (*Engine, error)
func DefaultRuleSet() RuleSet  // built-in rules for testing
```

### Step 3: Memory System (`memory/`) — The Episodic Layer

This is what makes the system learn. JSON-backed, survives across runs.

**`memory/memory.go`**:
```go
type Store struct {
    Path       string
    Facts      []MemoryFact        // learned facts
    RuleStats  []RulePerformance   // per-rule accuracy tracking
    RunHistory []RunSummary        // past execution summaries
    Patterns   []InferencePattern  // recurring feature→prediction
    mu         sync.Mutex          // thread-safe for agents
}
```

**Four things memory tracks:**

1. **Facts** — What the system has learned: `"dataset_accuracy": "0.83" (domain: supply-chain)`. Deduped, usage-counted.

2. **Rule Performance** — Per-rule: fire count, correct count, accuracy, which domains. This is what the rewriter reads.

3. **Patterns** — Feature signatures that map to predictions: `[blood_pressure, heart_rate] → treat_A (confidence: 0.85, seen 12 times)`. This is what the futures predictor reads.

4. **Run History** — Timestamp, datasets processed, accuracy, duration, swarm peak. Longitudinal performance tracking.

**Key functions:**
```go
NewStore(path) *Store          // load from file or create empty
Save() error                   // persist to JSON
RecordRuleFire(ruleID, correct, domain)  // track rule quality
RecordPattern(features, prediction, confidence)  // track patterns
PredictFromHistory(features) (label, confidence, found)  // pattern-match
LearnFact(key, value, domain, source)  // store knowledge
```

### Step 4: Recursive Rule Rewriter (`rewriter/`) — The Meta-Cognition Layer

Meta-rules that evaluate rules. Rules about rules.

**`rewriter/rewriter.go`**:
```go
type MetaRule struct {
    ID        string
    Name      string
    Condition func(stats RulePerformance) bool
    Action    func(rule RuleSpec) RewriteAction
}
```

**Built-in meta-rules:**

| Meta-Rule | Trigger | Action |
|-----------|---------|--------|
| Retire underperformers | accuracy < 40%, 10+ fires | Remove the rule |
| Strengthen high performers | accuracy > 90%, 5+ fires | Increase priority weight |
| Split ambiguous | fires correctly in domain A, fails in domain B | Create domain-specific variants |
| Weaken early warning | 3+ fires, accuracy < 60% | Flag for review |

**`ProposeFromPatterns()`** — The most interesting function. Reads memory patterns and proposes NEW rules that don't exist yet:
```
Pattern seen 5+ times: [temp, humidity] → storm (confidence 0.85)
No existing rule matches this pattern.
Proposed new rule: temp_high ∧ humidity_high → storm_pattern_indicated
```

Rules literally write themselves from observed data.

### Step 5: Future Chain Predictor (`futures/`) — The Working Memory Layer

Predicts outcomes before full inference runs.

**`futures/futures.go`**:
```go
type Predictor struct {
    Memory    *memory.Store
    RuleIndex map[string]RuleSpec  // fast lookup
    Labels    map[string]string
    Cache     map[string]Prediction
    MaxDepth  int
}
```

**How future chaining works:**
1. Given current facts, scan all rules for partially-satisfied bodies
2. Estimate probability of remaining body atoms (from memory patterns)
3. If a rule would fire, check what NEW rules that enables (chain forward)
4. Continue until no more rules would fire or max depth reached
5. Cache the prediction for fast lookup on similar inputs

**Three prediction sources** (checked in order):
1. Cache hit → instant return
2. Memory pattern match → prediction from episodic memory
3. Chain analysis → build predicted rule firing chain

**`WarmCache(n)`** — Pre-loads predictions from memory patterns at startup. Second run is faster than the first.

### Step 6: Agent Framework (`agents/`)

Four agent types, message-passing coordination.

**`agents/agents.go`**:
```go
type Agent interface {
    ID() string
    Role() Role
    Process(msg Message) (Message, error)
}
```

| Agent | Role | What It Does |
|-------|------|-------------|
| **SuperClaude** | Boss | Orchestrates everything. Dispatches work, collects results. |
| **Verifier** | Proof checker | Validates inference results: proof traces exist, confidence in [0,1], no empty steps. |
| **PhD** | Domain expert | Analyzes domains, recommends reasoning kinds, reviews results. |
| **Model** | Inference runner | Wraps an AR engine. Runs `Infer()` on individual datums. |

**Message types**: `directive`, `request`, `result`, `verify`, `review`, `report`.

All agents are thread-safe (`sync.Mutex`). All state is per-datum isolated.

### Step 7: Swarm Engine (`swarm/`)

Dynamic agent lifecycle with mesh networking.

**Three components:**

**`swarm/mesh.go`** — Peer-to-peer agent communication:
```go
type MeshNetwork struct {
    Channels map[string]chan Message  // per-agent inbox
    Routes   map[string][]string     // who can talk to whom
}
```
Register/unregister agents, send/receive/broadcast messages, bidirectional routing.

**`swarm/autoscaler.go`** — Dynamic scaling:
```go
// Scale UP when: queue depth > threshold AND agents < max
// Scale DOWN when: agent idle > timeout AND agents > min
func (as *AutoScaler) Evaluate() []ScaleDecision
```

**`swarm/controller.go`** — Lifecycle management:
```go
type Controller struct {
    Agents     map[string]Agent
    Mesh       *MeshNetwork
    Scaler     *AutoScaler
    Memory     *memory.Store
    Boss       *SuperClaudeAgent
}
```
Spawns agents, retires idle ones, submits tasks, runs datasets through the swarm.

### Step 8: Data Pipeline (`dataagents/`)

Five micro-agents that handle data:

1. **Discovery** — Scans directories for CSV files
2. **Loader** — Parses CSV into `DataSet` structs
3. **Validator** — Checks data quality (missing values, feature ranges)
4. **Quality** — Scores datasets (completeness, consistency, feature count)
5. **Registry** — Central catalog of all datasets with metadata

### Step 9: Config & CLI (`config/`, `cli/`)

**`config/config.go`** — ~30 parameters, all with defaults:
```go
type Config struct {
    // Data, Rules, Strategy, Agents, Evaluation,
    // Output, Memory, Rewriter, Futures, Swarm
}
func DefaultConfig() Config  // sensible defaults for everything
```

**`cli/cli.go`** — Interactive questionnaire. Users answer questions to configure. Supports auto mode (all defaults) and JSON file loading.

### Step 10: Pipeline (`pipeline/`)

Wires everything together. The `Run(cfg)` function:

```
1.  Load memory store
2.  Load AR engine (from file or built-in defaults)
3.  Create swarm controller (or traditional boss)
4.  Initialize future predictor, warm cache
5.  Send directive to boss
6.  Data filing: discover, load, validate datasets
7.  Run inference on all valid datasets
8.  Run SuperClaude boss evaluation
9.  Recursive rule rewriting
10. Record run summary in memory
11. Save memory to disk
12. Generate report
```

### Step 11: Report & Explain (`report/`, `explain/`)

**`explain/`** — Builds hierarchical natural-deduction proofs:
```
[+] 1. [conclusion] FINAL: defend (Confidence=0.42, Verified=true)
  [+] 2. [evidence] AR[Logic Programs]: prediction=defend
    [+] 3. [premise] has_feature(threat_level, high)
    [+] 4. [premise] has_feature(supply_available, low)
    [+] 5. [rule] rule[coa-r1]: threat_high ∧ supply_low => defend
```

**`report/`** — Generates full evaluation reports: executive summary, per-dataset results, metrics, agent logs, proofs, compliance checklist.

### Step 12: Entry Point (`main.go`)

```go
func main() {
    cfg := resolveConfig()  // --auto, --config, or interactive
    result := pipeline.Run(cfg)
    fmt.Println(report.FormatReport(result.Report))
    // Print memory, rewriter, swarm, futures summaries
}
```

---

## The Memory Loop in Detail

This is the core insight. Here's exactly how CARLA learns:

**Run 1** (cold start):
```
Memory: empty
Futures: no cache
Rules fire → outcomes recorded in memory
Memory after: 6 facts, 8 rule stats, 9 patterns
Rewriter: 1 rule strengthened (enough data to evaluate)
```

**Run 2** (warm):
```
Memory: loaded from disk (6 facts, 8 rule stats, 9 patterns)
Futures: warmed 9 cache entries from memory patterns
Predictions are faster (cache hits)
More data → rewriter can evaluate more rules
Rewriter: 5 rules strengthened
Memory after: same facts (deduped), updated stats, 1 more run in history
```

**Run N** (converged):
```
Memory: rich pattern database
Futures: high cache hit rate, predictions before inference
Rewriter: bad rules retired, good rules strengthened, new rules proposed
The rule set has evolved to match your domain
```

**This is the key**: the rules at run N are different from run 1. They've been tested against real data, scored by performance, and modified by meta-rules. The system wrote better rules than you started with.

---

## Adding Your Own Domain

1. **Write rules** (`data/rules/mydomain.json`):
```json
{
  "name": "fraud-detection",
  "domain": "finance",
  "rules": [
    {
      "id": "fraud-r1",
      "head": {"predicate": "suspicious_transaction"},
      "body": [
        {"predicate": "has_feature", "args": ["amount", "high"]},
        {"predicate": "has_feature", "args": ["frequency", "high"]}
      ]
    }
  ],
  "labels": [
    {"atom": "suspicious_transaction", "label": "flag_for_review"}
  ]
}
```

2. **Add data** (`data/datasets/transactions.csv`):
```csv
amount,frequency,velocity,label
0.9,0.8,0.3,flag_for_review
0.2,0.1,0.5,normal
```

3. **Run**:
```bash
go run . --auto
# or with custom config:
go run . --config myconfig.json
```

4. **Watch it learn**: Run multiple times. Check `data/memory.json` to see patterns accumulate. Check rewriter output to see rules evolving.

---

## Testing

```bash
go test ./...          # all 12 packages
go test ./memory/...   # just memory
go test ./swarm/...    # just swarm
go test -v ./...       # verbose
go build .             # compile binary
./carla --auto         # run compiled binary
```

---

## File Count

| Package | Files | Test Files | Purpose |
|---------|-------|-----------|---------|
| `kinds/` | 2 | 1 | Shared types |
| `ar/` | 2 | 2 | Reasoning engine |
| `agents/` | 1 | 1 | Agent framework |
| `memory/` | 1 | 1 | Persistent store |
| `rewriter/` | 1 | 1 | Rule evolution |
| `futures/` | 1 | 1 | Predictive cache |
| `swarm/` | 3 | 1 | Mesh + autoscale + controller |
| `dataagents/` | 5 | 1 | Data pipeline |
| `config/` | 1 | 1 | Configuration |
| `cli/` | 1 | 0 | Interactive CLI |
| `pipeline/` | 1 | 1 | Orchestration |
| `explain/` | 1 | 1 | Proof system |
| `report/` | 1 | 1 | Report generation |
| `testdata/` | 1 | 0 | Built-in datasets |
| root | 1 | 0 | Entry point |
| **Total** | **23** | **13** | **~2,500 lines** |

Zero external dependencies. Single `go build .` produces one binary.

---

## What Makes This a Hybrid Memory System

Traditional AI memory is one layer: a vector database, a KV store, or model weights.

CARLA has four layers that feed each other:

```
Crystallized (rules)     ← Rewriter reads episodic memory
       ↓
Episodic (memory.json)   ← Pipeline records outcomes
       ↓
Working (futures cache)  ← Warmed from episodic at startup
       ↓
Immediate (infer state)  ← Fresh per datum, garbage collected
```

The feedback loop runs **across runs**, not just within a session. Run it Monday, it remembers on Tuesday. Run it for a month, the rules at the end are different from the rules at the start — they've been tested, scored, evolved, and proposed by the system itself.

This is not machine learning. There are no weights, no gradients, no training. It's **logical reasoning with persistent memory and self-modifying rules**. The intelligence is in the loop.
