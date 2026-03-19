package ml

import (
	"encoding/json"
	"fmt"
	"os"
)

// NodeSpec is the JSON-serializable format for a BayesNet node.
type NodeSpec struct {
	Name    string   `json:"name"`
	States  []string `json:"states"`
	Parents []string `json:"parents,omitempty"`
}

// CPTEntry is a single conditional probability entry.
type CPTEntry struct {
	Node        string  `json:"node"`
	ParentState string  `json:"parent_state"`
	NodeState   string  `json:"node_state"`
	Probability float64 `json:"probability"`
}

// PriorEntry is a prior probability for a root node.
type PriorEntry struct {
	Node        string  `json:"node"`
	State       string  `json:"state"`
	Probability float64 `json:"probability"`
}

// LabelEntry maps a state to a classification label.
type LabelEntry struct {
	State string `json:"state"`
	Label string `json:"label"`
}

// BayesNetSpec is the top-level JSON structure for a BayesNet file.
type BayesNetSpec struct {
	Name   string       `json:"name"`
	Domain string       `json:"domain"`
	Nodes  []NodeSpec   `json:"nodes"`
	CPTs   []CPTEntry   `json:"cpts"`
	Priors []PriorEntry `json:"priors"`
	Labels []LabelEntry `json:"labels"`
}

// LoadBayesNetFromFile reads a BayesNet structure from a JSON file.
func LoadBayesNetFromFile(path string) (*Engine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bayesnet file: %w", err)
	}
	return LoadBayesNetFromJSON(data)
}

// LoadBayesNetFromJSON parses a BayesNet from JSON bytes.
func LoadBayesNetFromJSON(data []byte) (*Engine, error) {
	var spec BayesNetSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse bayesnet: %w", err)
	}

	net := NewBayesNet()

	for i := 0; i < len(spec.Nodes); i++ {
		n := spec.Nodes[i]
		net.AddNode(n.Name, n.States, n.Parents)
	}

	for i := 0; i < len(spec.Priors); i++ {
		p := spec.Priors[i]
		net.SetPrior(p.Node, p.State, p.Probability)
	}

	for i := 0; i < len(spec.CPTs); i++ {
		c := spec.CPTs[i]
		net.SetCPT(c.Node, c.ParentState, c.NodeState, c.Probability)
	}

	for i := 0; i < len(spec.Labels); i++ {
		l := spec.Labels[i]
		net.SetLabel(l.State, l.Label)
	}

	return NewEngine(net), nil
}

// DefaultBayesNetSpec returns the built-in BayesNet as a spec.
func DefaultBayesNetSpec() BayesNetSpec {
	features := []string{
		"blood_pressure", "glucose", "heart_rate",
		"threat_level", "supply_available", "terrain_difficulty",
		"equipment_age", "usage_rate", "failure_history",
	}

	nodes := make([]NodeSpec, 0, len(features)+1)
	priors := make([]PriorEntry, 0, len(features)*2)

	for _, f := range features {
		nodes = append(nodes, NodeSpec{Name: f, States: []string{"high", "low"}})
		priors = append(priors,
			PriorEntry{Node: f, State: "high", Probability: 0.5},
			PriorEntry{Node: f, State: "low", Probability: 0.5},
		)
	}

	nodes = append(nodes, NodeSpec{
		Name:    "decision",
		States:  []string{"high", "low"},
		Parents: []string{"blood_pressure", "glucose", "heart_rate"},
	})

	return BayesNetSpec{
		Name:   "clara-phase1-default",
		Domain: "multi-domain",
		Nodes:  nodes,
		Priors: priors,
		CPTs: []CPTEntry{
			{Node: "decision", ParentState: "high", NodeState: "high", Probability: 0.8},
			{Node: "decision", ParentState: "high", NodeState: "low", Probability: 0.2},
			{Node: "decision", ParentState: "low", NodeState: "high", Probability: 0.3},
			{Node: "decision", ParentState: "low", NodeState: "low", Probability: 0.7},
		},
		Labels: []LabelEntry{
			{State: "high", Label: "treat_A"},
			{State: "low", Label: "no_treat"},
		},
	}
}
