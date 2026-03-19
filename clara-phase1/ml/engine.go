// Package ml implements the Machine Learning engine for CLARA Phase 1.
// Uses Bayesian Network inference with iterative belief propagation.
// No recursion. Each call to Infer is fully isolated — beliefs are computed fresh.
package ml

import (
	"fmt"
	"math"
	"sort"

	"github.com/clara-phase1/kinds"
)

// Node defines a Bayesian Network node (structure only, no mutable state).
type Node struct {
	Name    string
	States  []string
	Parents []string
	CPT     map[string]float64 // "parent_state|node_state" -> probability
	Prior   map[string]float64 // "|node_state" -> prior probability
}

// BayesNet defines the network structure and parameters (immutable after construction).
type BayesNet struct {
	Nodes  map[string]*Node
	Order  []string          // topological order
	Labels map[string]string // state -> class label
}

func NewBayesNet() *BayesNet {
	return &BayesNet{
		Nodes:  make(map[string]*Node),
		Labels: make(map[string]string),
	}
}

// AddNode adds a node to the network structure.
func (bn *BayesNet) AddNode(name string, states []string, parents []string) {
	bn.Nodes[name] = &Node{
		Name:    name,
		States:  states,
		Parents: parents,
		CPT:     make(map[string]float64),
		Prior:   make(map[string]float64),
	}
	bn.Order = append(bn.Order, name)
}

// SetCPT sets a conditional probability entry.
func (bn *BayesNet) SetCPT(nodeName, parentState, nodeState string, prob float64) {
	node, ok := bn.Nodes[nodeName]
	if !ok {
		return
	}
	node.CPT[parentState+"|"+nodeState] = prob
}

// SetPrior sets an unconditional prior for a root node.
func (bn *BayesNet) SetPrior(nodeName, state string, prob float64) {
	node, ok := bn.Nodes[nodeName]
	if !ok {
		return
	}
	node.Prior[state] = prob
}

// SetLabel maps a state to a classification label.
func (bn *BayesNet) SetLabel(state, label string) {
	bn.Labels[state] = label
}

// Engine wraps a BayesNet for the CLARA agent framework.
type Engine struct {
	Net *BayesNet
}

func NewEngine(net *BayesNet) *Engine {
	return &Engine{Net: net}
}

func (e *Engine) Name() string { return "BayesianNetwork-BeliefProp" }

// beliefState holds per-inference beliefs. Created fresh per Infer call.
type beliefState struct {
	beliefs map[string]map[string]float64 // nodeName -> state -> probability
}

func newBeliefState(net *BayesNet) *beliefState {
	bs := &beliefState{
		beliefs: make(map[string]map[string]float64),
	}
	// Initialize all beliefs to uniform
	for name, node := range net.Nodes {
		bs.beliefs[name] = make(map[string]float64)
		for _, s := range node.States {
			bs.beliefs[name][s] = 1.0 / float64(len(node.States))
		}
	}
	return bs
}

func (bs *beliefState) normalize(nodeName string, states []string) {
	total := 0.0
	for _, s := range states {
		total += bs.beliefs[nodeName][s]
	}
	if total > 0 && !math.IsNaN(total) && !math.IsInf(total, 0) {
		for _, s := range states {
			bs.beliefs[nodeName][s] /= total
		}
	}
}

// Infer runs fully isolated belief propagation on a single datum.
// Creates fresh belief state — no state persists between calls.
func (e *Engine) Infer(datum kinds.Datum) (kinds.ModelResult, error) {
	net := e.Net
	bs := newBeliefState(net)

	proofTrace := make([]string, 0, len(net.Order)+2)
	proofTrace = append(proofTrace, fmt.Sprintf("evidence: %d features observed", len(datum.Features)))

	// Step 1: Set evidence from datum features
	featureKeys := make([]string, 0, len(datum.Features))
	for k := range datum.Features {
		featureKeys = append(featureKeys, k)
	}
	sort.Strings(featureKeys)

	for _, fname := range featureKeys {
		fval := datum.Features[fname]
		node, ok := net.Nodes[fname]
		if !ok {
			continue
		}
		for _, state := range node.States {
			if state == "high" && fval > 0.5 {
				bs.beliefs[fname][state] = fval
			} else if state == "low" && fval <= 0.5 {
				bs.beliefs[fname][state] = 1.0 - fval
			} else if state == "high" && fval <= 0.5 {
				bs.beliefs[fname][state] = fval // low confidence for high
			} else if state == "low" && fval > 0.5 {
				bs.beliefs[fname][state] = 1.0 - fval // low confidence for low
			}
		}
		bs.normalize(fname, node.States)
	}

	// Step 2: Forward belief propagation along topological order (iterative)
	for oi := 0; oi < len(net.Order); oi++ {
		nodeName := net.Order[oi]
		node := net.Nodes[nodeName]

		if len(node.Parents) == 0 {
			// Root node: combine evidence with prior using Bayesian update
			hasPrior := false
			for _, s := range node.States {
				if p, ok := node.Prior[s]; ok {
					bs.beliefs[nodeName][s] *= p
					hasPrior = true
				}
			}
			if hasPrior {
				bs.normalize(nodeName, node.States)
				proofTrace = append(proofTrace,
					fmt.Sprintf("prior(%s): P(high)=%.3f P(low)=%.3f",
						nodeName, bs.beliefs[nodeName]["high"], bs.beliefs[nodeName]["low"]))
			}
			continue
		}

		// Non-root: propagate from parents
		for _, state := range node.States {
			totalProb := 0.0
			for _, parentName := range node.Parents {
				parent, ok := net.Nodes[parentName]
				if !ok {
					continue
				}
				for _, pState := range parent.States {
					key := pState + "|" + state
					cpt, ok := node.CPT[key]
					if !ok {
						cpt = 1.0 / float64(len(node.States))
					}
					totalProb += cpt * bs.beliefs[parentName][pState]
				}
			}
			if totalProb > 0 {
				bs.beliefs[nodeName][state] = totalProb
			}
		}
		bs.normalize(nodeName, node.States)
		proofTrace = append(proofTrace,
			fmt.Sprintf("propagate(%s): P(high)=%.3f P(low)=%.3f",
				nodeName, bs.beliefs[nodeName]["high"], bs.beliefs[nodeName]["low"]))
	}

	// Step 3: Extract prediction from output node (last in topological order)
	prediction := "unknown"
	confidence := 0.0

	if len(net.Order) > 0 {
		outputName := net.Order[len(net.Order)-1]

		bestState := ""
		bestProb := 0.0
		for _, s := range net.Nodes[outputName].States {
			if bs.beliefs[outputName][s] > bestProb {
				bestProb = bs.beliefs[outputName][s]
				bestState = s
			}
		}

		if label, ok := net.Labels[bestState]; ok {
			prediction = label
		} else {
			prediction = bestState
		}
		confidence = clamp(bestProb, 0.0, 1.0)
		proofTrace = append(proofTrace,
			fmt.Sprintf("conclusion: %s=P(%.3f) via %s", prediction, confidence, outputName))
	}

	return kinds.NewModelResult(prediction, confidence, kinds.KindBayesNets, proofTrace), nil
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
