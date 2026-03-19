package ar

import (
	"encoding/json"
	"fmt"
	"os"
)

// RuleSpec is the JSON-serializable format for AR rules.
type RuleSpec struct {
	ID     string     `json:"id"`
	Head   AtomSpec   `json:"head"`
	Body   []AtomSpec `json:"body"`
}

// AtomSpec is the JSON-serializable format for atoms.
type AtomSpec struct {
	Predicate string   `json:"predicate"`
	Args      []string `json:"args,omitempty"`
}

// LabelSpec maps derived atoms to classification labels.
type LabelSpec struct {
	Atom  string `json:"atom"`
	Label string `json:"label"`
}

// RuleSet is the top-level JSON structure for an AR rules file.
type RuleSet struct {
	Name   string      `json:"name"`
	Domain string      `json:"domain"`
	Rules  []RuleSpec  `json:"rules"`
	Labels []LabelSpec `json:"labels"`
}

// LoadRulesFromFile reads AR rules from a JSON file and configures an Engine.
func LoadRulesFromFile(path string) (*Engine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read rules file: %w", err)
	}
	return LoadRulesFromJSON(data)
}

// LoadRulesFromJSON parses AR rules from JSON bytes and configures an Engine.
func LoadRulesFromJSON(data []byte) (*Engine, error) {
	var rs RuleSet
	if err := json.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("parse rules: %w", err)
	}

	engine := NewEngine()

	for i := 0; i < len(rs.Rules); i++ {
		spec := rs.Rules[i]
		head := Atom{Predicate: spec.Head.Predicate, Args: spec.Head.Args}
		body := make([]Atom, len(spec.Body))
		for j := 0; j < len(spec.Body); j++ {
			body[j] = Atom{Predicate: spec.Body[j].Predicate, Args: spec.Body[j].Args}
		}
		engine.AddRule(spec.ID, head, body...)
	}

	for i := 0; i < len(rs.Labels); i++ {
		engine.SetLabel(rs.Labels[i].Atom, rs.Labels[i].Label)
	}

	return engine, nil
}

// DefaultRuleSet returns the built-in rules as a RuleSet (for generating sample files).
func DefaultRuleSet() RuleSet {
	return RuleSet{
		Name:   "clara-phase1-default",
		Domain: "multi-domain",
		Rules: []RuleSpec{
			{ID: "med-r1", Head: AtomSpec{Predicate: "treat_A_indicated"}, Body: []AtomSpec{
				{Predicate: "has_feature", Args: []string{"blood_pressure", "high"}},
				{Predicate: "has_feature", Args: []string{"heart_rate", "high"}},
			}},
			{ID: "med-r2", Head: AtomSpec{Predicate: "treat_B_indicated"}, Body: []AtomSpec{
				{Predicate: "has_feature", Args: []string{"glucose", "high"}},
				{Predicate: "has_feature", Args: []string{"blood_pressure", "low"}},
			}},
			{ID: "med-r3", Head: AtomSpec{Predicate: "treat_C_indicated"}, Body: []AtomSpec{
				{Predicate: "has_feature", Args: []string{"blood_pressure", "high"}},
				{Predicate: "has_feature", Args: []string{"glucose", "high"}},
				{Predicate: "has_feature", Args: []string{"heart_rate", "high"}},
			}},
			{ID: "med-r4", Head: AtomSpec{Predicate: "no_treat_indicated"}, Body: []AtomSpec{
				{Predicate: "has_feature", Args: []string{"blood_pressure", "low"}},
				{Predicate: "has_feature", Args: []string{"glucose", "low"}},
				{Predicate: "has_feature", Args: []string{"heart_rate", "low"}},
			}},
			{ID: "coa-r1", Head: AtomSpec{Predicate: "defend_indicated"}, Body: []AtomSpec{
				{Predicate: "has_feature", Args: []string{"threat_level", "high"}},
				{Predicate: "has_feature", Args: []string{"supply_available", "low"}},
			}},
			{ID: "coa-r2", Head: AtomSpec{Predicate: "advance_indicated"}, Body: []AtomSpec{
				{Predicate: "has_feature", Args: []string{"threat_level", "low"}},
				{Predicate: "has_feature", Args: []string{"supply_available", "high"}},
			}},
			{ID: "coa-r3", Head: AtomSpec{Predicate: "retreat_indicated"}, Body: []AtomSpec{
				{Predicate: "has_feature", Args: []string{"supply_available", "low"}},
				{Predicate: "has_feature", Args: []string{"terrain_difficulty", "high"}},
			}},
			{ID: "sc-r1", Head: AtomSpec{Predicate: "maintain_now_indicated"}, Body: []AtomSpec{
				{Predicate: "has_feature", Args: []string{"equipment_age", "high"}},
				{Predicate: "has_feature", Args: []string{"usage_rate", "high"}},
			}},
			{ID: "sc-r2", Head: AtomSpec{Predicate: "replace_indicated"}, Body: []AtomSpec{
				{Predicate: "has_feature", Args: []string{"equipment_age", "high"}},
				{Predicate: "has_feature", Args: []string{"failure_history", "high"}},
			}},
			{ID: "sc-r3", Head: AtomSpec{Predicate: "no_action_indicated"}, Body: []AtomSpec{
				{Predicate: "has_feature", Args: []string{"equipment_age", "low"}},
				{Predicate: "has_feature", Args: []string{"failure_history", "low"}},
			}},
		},
		Labels: []LabelSpec{
			{Atom: "treat_A_indicated", Label: "treat_A"},
			{Atom: "treat_B_indicated", Label: "treat_B"},
			{Atom: "treat_C_indicated", Label: "treat_C"},
			{Atom: "no_treat_indicated", Label: "no_treat"},
			{Atom: "defend_indicated", Label: "defend"},
			{Atom: "advance_indicated", Label: "advance"},
			{Atom: "retreat_indicated", Label: "retreat"},
			{Atom: "maintain_now_indicated", Label: "maintain_now"},
			{Atom: "replace_indicated", Label: "replace"},
			{Atom: "no_action_indicated", Label: "no_action"},
		},
	}
}
