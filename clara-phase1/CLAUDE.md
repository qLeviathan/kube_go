# CLARA: Compositional Learning-And-Reasoning for AI

## Architecture Principles

1. **Zero external dependencies** — stdlib only. Single binary deployment.
2. **Iterative inference, never recursive** — all engines use worklists/topological order. DARPA requirement.
3. **Per-datum state isolation** — fresh inferState/beliefState per Infer() call. No state bleeds.
4. **Think two steps ahead** — every change must consider its downstream effects AND what it enables next. Before adding a feature, ask: "what does this unlock?" and "what will break?"
5. **Recursive rule checking** — rules validate themselves. Meta-rules evaluate rule quality. Rules evolve across runs.
6. **Swarm-first coordination** — agents communicate via mesh topology, not star. Autoscale based on real demand.
7. **Perpetual memory** — the system learns from every run. Patterns, rule performance, and facts persist in `data/memory.json`.

## Two Steps Ahead Protocol

When modifying any component:
- Step 0: What does the current code do?
- Step 1: What does this change accomplish?
- Step 2: What does this change ENABLE that wasn't possible before?
- If Step 2 is empty, reconsider the change.

## Package Boundaries

- `ar/` — Automated Reasoning engine. Rules, forward-chaining. No imports from ml/ or agents/.
- `ml/` — Machine Learning engine. BayesNet, belief propagation. No imports from ar/ or agents/.
- `kinds/` — Shared types (Datum, DataSet, ModelResult, ComposedResult, Kind). No imports from other project packages.
- `agents/` — Agent framework. Imports kinds/ only.
- `compose/` — AR+ML composition. Imports agents/, kinds/.
- `memory/` — Persistent store. Imports kinds/ only.
- `rewriter/` — Rule evolution. Imports ar/, memory/.
- `futures/` — Predictive inference. Imports ar/, memory/, kinds/.
- `swarm/` — Agent lifecycle, mesh, autoscaling. Imports agents/, memory/.
- `dataagents/` — Data filing pipeline. Imports kinds/.
- `config/` — Configuration. Imports cli/.
- `pipeline/` — Orchestration. Imports everything.
- `cli/` — Interactive prompts. No project imports.

## Development Guidelines

- All new packages must have `_test.go` files with meaningful coverage.
- Test with `go test ./...` before committing.
- Build with `go build .` — must compile clean with zero warnings.
- Run with `--auto` flag for quick validation.
- Use iterative loops, not recursive function calls, for any computation.
- Agent communication goes through the mesh network, not direct method calls on other agents.
- Every config parameter has a sensible default in `DefaultConfig()`.

## File Layout

```
clara-phase1/
├── CLAUDE.md              ← you are here
├── main.go                ← CLI entry point
├── ar/                    ← Automated Reasoning
├── ml/                    ← Machine Learning
├── kinds/                 ← Shared types
├── agents/                ← Agent framework
├── compose/               ← AR+ML composition
├── memory/                ← Persistent learning store
├── rewriter/              ← Recursive rule evolution
├── futures/               ← Predictive future chaining
├── swarm/                 ← Autoscaled agent swarm
├── dataagents/            ← Data filing agents
├── config/                ← Dynamic configuration
├── cli/                   ← Interactive questionnaire
├── pipeline/              ← End-to-end orchestration
├── explain/               ← Proof system
├── report/                ← Report generation
├── testdata/              ← Built-in datasets
└── data/                  ← Runtime data (memory, rules, models, datasets)
```
