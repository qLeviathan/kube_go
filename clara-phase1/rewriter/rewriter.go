// Package rewriter implements recursive rule-checking and rewriting for CARLA.
// Meta-rules evaluate rule effectiveness and propose improvements.
// Rules evolve across runs: underperformers get retired, high performers
// get strengthened, and new rules get proposed from observed patterns.
// The rewriter checks itself — each rewrite pass validates its own output.
package rewriter

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/clara-phase1/ar"
	"github.com/clara-phase1/memory"
)

// MetaRule evaluates a rule's performance and proposes actions.
// Meta-rules are the recursive layer: rules that check rules.
type MetaRule struct {
	ID        string
	Name      string
	Condition func(stats memory.RulePerformance) bool
	Action    func(rule ar.RuleSpec, stats memory.RulePerformance) []RewriteAction
}

// RewriteAction describes a single rewrite operation.
type RewriteAction struct {
	Type      string       `json:"type"` // "strengthen", "weaken", "retire", "propose", "split"
	RuleID    string       `json:"rule_id"`
	Reason    string       `json:"reason"`
	OldRule   *ar.RuleSpec `json:"old_rule,omitempty"`
	NewRules  []ar.RuleSpec `json:"new_rules,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
}

// RewriteReport summarizes the outcome of a rewrite pass.
type RewriteReport struct {
	Evaluated    int             `json:"evaluated"`
	Strengthened int             `json:"strengthened"`
	Weakened     int             `json:"weakened"`
	Retired      int             `json:"retired"`
	Proposed     int             `json:"proposed"`
	Split        int             `json:"split"`
	Actions      []RewriteAction `json:"actions"`
}

// Rewriter is the recursive rule-checking engine.
type Rewriter struct {
	Memory    *memory.Store
	MetaRules []MetaRule
	History   []RewriteAction
	MinFires  int // minimum fires before evaluating a rule
}

// NewRewriter creates a rewriter with default meta-rules.
func NewRewriter(mem *memory.Store, minFires int) *Rewriter {
	rw := &Rewriter{
		Memory:   mem,
		MinFires: minFires,
	}
	rw.MetaRules = rw.defaultMetaRules()
	return rw
}

// defaultMetaRules returns the built-in meta-rules for rule evaluation.
// These are the "rules about rules" — the recursive checking layer.
func (rw *Rewriter) defaultMetaRules() []MetaRule {
	minFires := rw.MinFires
	if minFires <= 0 {
		minFires = 5
	}

	return []MetaRule{
		{
			ID:   "meta-retire-underperformer",
			Name: "Retire underperforming rules",
			Condition: func(stats memory.RulePerformance) bool {
				return stats.FireCount >= minFires && stats.Accuracy < 0.4
			},
			Action: func(rule ar.RuleSpec, stats memory.RulePerformance) []RewriteAction {
				return []RewriteAction{{
					Type:   "retire",
					RuleID: rule.ID,
					Reason: fmt.Sprintf("accuracy %.1f%% over %d fires (below 40%% threshold)",
						stats.Accuracy*100, stats.FireCount),
					OldRule:   &rule,
					Timestamp: time.Now(),
				}}
			},
		},
		{
			ID:   "meta-strengthen-performer",
			Name: "Strengthen high-performing rules",
			Condition: func(stats memory.RulePerformance) bool {
				return stats.FireCount >= minFires && stats.Accuracy >= 0.9
			},
			Action: func(rule ar.RuleSpec, stats memory.RulePerformance) []RewriteAction {
				return []RewriteAction{{
					Type:   "strengthen",
					RuleID: rule.ID,
					Reason: fmt.Sprintf("accuracy %.1f%% over %d fires (above 90%% threshold)",
						stats.Accuracy*100, stats.FireCount),
					Timestamp: time.Now(),
				}}
			},
		},
		{
			ID:   "meta-split-ambiguous",
			Name: "Split domain-ambiguous rules",
			Condition: func(stats memory.RulePerformance) bool {
				return stats.FireCount >= minFires && len(stats.Domains) > 1 &&
					stats.Accuracy >= 0.4 && stats.Accuracy < 0.7
			},
			Action: func(rule ar.RuleSpec, stats memory.RulePerformance) []RewriteAction {
				// Create domain-specific variants
				actions := make([]RewriteAction, 0, 1)
				newRules := make([]ar.RuleSpec, 0, len(stats.Domains))
				for _, domain := range stats.Domains {
					variant := ar.RuleSpec{
						ID:   fmt.Sprintf("%s-%s", rule.ID, domain),
						Head: rule.Head,
						Body: make([]ar.AtomSpec, len(rule.Body)),
					}
					copy(variant.Body, rule.Body)
					// Add domain guard to body
					variant.Body = append(variant.Body, ar.AtomSpec{
						Predicate: "in_domain",
						Args:      []string{domain},
					})
					newRules = append(newRules, variant)
				}
				actions = append(actions, RewriteAction{
					Type:     "split",
					RuleID:   rule.ID,
					Reason:   fmt.Sprintf("ambiguous across %d domains: %s", len(stats.Domains), strings.Join(stats.Domains, ", ")),
					OldRule:  &rule,
					NewRules: newRules,
					Timestamp: time.Now(),
				})
				return actions
			},
		},
		{
			ID:   "meta-weaken-overconfident",
			Name: "Weaken overconfident rules with low volume",
			Condition: func(stats memory.RulePerformance) bool {
				return stats.FireCount >= 2 && stats.FireCount < minFires &&
					stats.Accuracy < 0.5
			},
			Action: func(rule ar.RuleSpec, stats memory.RulePerformance) []RewriteAction {
				return []RewriteAction{{
					Type:   "weaken",
					RuleID: rule.ID,
					Reason: fmt.Sprintf("early signal: %.1f%% accuracy over %d fires",
						stats.Accuracy*100, stats.FireCount),
					Timestamp: time.Now(),
				}}
			},
		},
	}
}

// Evaluate runs all meta-rules against all rules in the rule set.
// This is the recursive check: rules examining rules.
func (rw *Rewriter) Evaluate(ruleSet ar.RuleSet) RewriteReport {
	report := RewriteReport{}
	allStats := rw.Memory.AllRuleStats()

	// Build stats index
	statsMap := make(map[string]memory.RulePerformance)
	for _, s := range allStats {
		statsMap[s.RuleID] = s
	}

	report.Evaluated = len(ruleSet.Rules)

	for i := 0; i < len(ruleSet.Rules); i++ {
		rule := ruleSet.Rules[i]
		stats, exists := statsMap[rule.ID]
		if !exists {
			continue
		}

		for _, meta := range rw.MetaRules {
			if meta.Condition(stats) {
				actions := meta.Action(rule, stats)
				for _, action := range actions {
					report.Actions = append(report.Actions, action)
					switch action.Type {
					case "strengthen":
						report.Strengthened++
					case "weaken":
						report.Weakened++
					case "retire":
						report.Retired++
					case "propose":
						report.Proposed++
					case "split":
						report.Split++
					}
				}
			}
		}
	}

	// Self-check: verify rewrite report consistency
	rw.validateReport(&report)

	rw.History = append(rw.History, report.Actions...)
	return report
}

// ProposeFromPatterns generates new rule proposals from observed inference patterns.
// This is future chaining at the rule level: patterns predict rules that should exist.
func (rw *Rewriter) ProposeFromPatterns(patterns []memory.InferencePattern, existingRules ar.RuleSet) []ar.RuleSpec {
	// Build set of existing rule heads for dedup
	existingHeads := make(map[string]bool)
	for _, r := range existingRules.Rules {
		existingHeads[r.Head.Predicate] = true
	}

	var proposals []ar.RuleSpec

	for _, pattern := range patterns {
		if pattern.Frequency < 3 || pattern.Confidence < 0.6 {
			continue
		}

		// Create a rule head from the prediction
		headPred := fmt.Sprintf("%s_pattern_indicated", pattern.Prediction)
		if existingHeads[headPred] {
			continue
		}

		// Parse feature key back into body atoms
		features := strings.Split(pattern.FeatureKey, ",")
		body := make([]ar.AtomSpec, 0, len(features))
		for _, f := range features {
			if f == "" {
				continue
			}
			body = append(body, ar.AtomSpec{
				Predicate: "has_feature",
				Args:      []string{f, "high"},
			})
		}

		if len(body) == 0 {
			continue
		}

		proposals = append(proposals, ar.RuleSpec{
			ID:   fmt.Sprintf("auto-%s-%d", pattern.Prediction, pattern.Frequency),
			Head: ar.AtomSpec{Predicate: headPred},
			Body: body,
		})

		existingHeads[headPred] = true
	}

	return proposals
}

// Apply applies approved rewrite actions to a rule set.
// Returns the number of changes applied.
func (rw *Rewriter) Apply(ruleSet *ar.RuleSet, report RewriteReport) int {
	changes := 0

	// Collect retired rule IDs
	retired := make(map[string]bool)
	for _, action := range report.Actions {
		if action.Type == "retire" {
			retired[action.RuleID] = true
		}
	}

	// Remove retired rules
	if len(retired) > 0 {
		kept := make([]ar.RuleSpec, 0, len(ruleSet.Rules))
		for _, r := range ruleSet.Rules {
			if !retired[r.ID] {
				kept = append(kept, r)
			} else {
				changes++
			}
		}
		ruleSet.Rules = kept
	}

	// Add split variants
	for _, action := range report.Actions {
		if action.Type == "split" && len(action.NewRules) > 0 {
			ruleSet.Rules = append(ruleSet.Rules, action.NewRules...)
			changes += len(action.NewRules)
		}
	}

	return changes
}

// validateReport is the self-check: the rewriter validates its own output.
// Ensures no contradictory actions (e.g., retire AND strengthen same rule).
func (rw *Rewriter) validateReport(report *RewriteReport) {
	// Check for contradictions
	actionMap := make(map[string][]string) // ruleID -> action types
	for _, action := range report.Actions {
		actionMap[action.RuleID] = append(actionMap[action.RuleID], action.Type)
	}

	// Remove contradictions: if a rule is both strengthened and retired, keep only retire
	contradictions := make(map[string]bool)
	for ruleID, types := range actionMap {
		if len(types) > 1 {
			contradictions[ruleID] = true
		}
	}

	if len(contradictions) > 0 {
		// Filter: keep highest-priority action per rule
		// Priority: retire > split > weaken > strengthen
		priority := map[string]int{"retire": 4, "split": 3, "weaken": 2, "strengthen": 1, "propose": 0}
		filtered := make([]RewriteAction, 0, len(report.Actions))
		kept := make(map[string]bool) // ruleID -> already kept

		// Sort by priority descending
		sort.Slice(report.Actions, func(i, j int) bool {
			return priority[report.Actions[i].Type] > priority[report.Actions[j].Type]
		})

		for _, action := range report.Actions {
			if contradictions[action.RuleID] {
				if !kept[action.RuleID] {
					filtered = append(filtered, action)
					kept[action.RuleID] = true
				}
			} else {
				filtered = append(filtered, action)
			}
		}

		report.Actions = filtered

		// Recount
		report.Strengthened = 0
		report.Weakened = 0
		report.Retired = 0
		report.Proposed = 0
		report.Split = 0
		for _, a := range report.Actions {
			switch a.Type {
			case "strengthen":
				report.Strengthened++
			case "weaken":
				report.Weakened++
			case "retire":
				report.Retired++
			case "propose":
				report.Proposed++
			case "split":
				report.Split++
			}
		}
	}
}

// Summary returns a human-readable summary of the rewriter state.
func (rw *Rewriter) Summary() string {
	return fmt.Sprintf("Rewriter: %d meta-rules, %d history actions, min-fires=%d",
		len(rw.MetaRules), len(rw.History), rw.MinFires)
}
