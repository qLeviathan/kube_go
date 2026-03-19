// Package ml implements the Machine Learning engine for CLARA Phase 1.
// Uses Bayesian Network inference with iterative belief propagation.
// No recursion — all computation is iterative.
package ml

import (
	"fmt"
	"math"
	"sort"

	"github.com/clara-phase1/kinds"
)

// Node is a Bayesian Network node.
type Node struct {
	Name    string
	States  []string            // possible states
	Parents []string            // parent node names
	CPT     map[string]float64  // conditional probability table: "parent_state|node_state" -> prob
	Belief  map[string]float64  // current belief for each state
}

// BayesNet is a Bayesian Network for ML inference.
type BayesNet struct {
	Nodes  map[string]*Node
	Order  []string // topological order (pre-computed, no recursion needed)
	Labels map[string]string // node_state -> class label
}

func NewBayesNet() *BayesNet {
	return &BayesNet{
		Nodes:  make(map[string]*Node),
		Labels: make(map[string]string),
	}
}

// AddNode adds a node to the network.
func (bn *BayesNet) AddNode(name string, states []string, parents []string) *Node {
	node := &Node{
		Name:    name,
		States:  states,
		Parents: parents,
		CPT:     make(map[string]float64),
		Belief:  make(map[string]float64),
	}
	// Initialize uniform belief
	for _, s := range states {
		node.Belief[s] = 1.0 / float64(len(states))
	}
	bn.Nodes[name] = node
	bn.Order = append(bn.Order, name)
	return node
}

// SetCPT sets a conditional probability entry.
func (bn *BayesNet) SetCPT(nodeName, parentState, nodeState string, prob float64) {
	node, ok := bn.Nodes[nodeName]
	if !ok {
		return
	}
	key := parentState + "|" + nodeState
	node.CPT[key] = prob
}

// SetPrior sets an unconditional probability for a root node.
func (bn *BayesNet) SetPrior(nodeName, state string, prob float64) {
	node, ok := bn.Nodes[nodeName]
	if !ok {
		return
	}
	node.CPT["|"+state] = prob
}

// SetLabel maps a node state to a classification label.
func (bn *BayesNet) SetLabel(nodeState, label string) {
	bn.Labels[nodeState] = label
}

// Engine wraps the BayesNet for the CLARA agent framework.
type Engine struct {
	Net *BayesNet
}

func NewEngine(net *BayesNet) *Engine {
	return &Engine{Net: net}
}

func (e *Engine) Name() string { return "BayesianNetwork-BeliefProp" }

// Infer runs iterative belief propagation on the network given observed features.
func (e *Engine) Infer(datum kinds.Datum) (kinds.ModelResult, error) {
	net := e.Net

	// Step 1: Set evidence from datum features (iterative)
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
		// Set observed evidence: update beliefs based on feature value
		for si := 0; si < len(node.States); si++ {
			state := node.States[si]
			if state == "high" && fval > 0.5 {
				node.Belief[state] = fval
			} else if state == "low" && fval <= 0.5 {
				node.Belief[state] = 1.0 - fval
			} else {
				node.Belief[state] = 0.1 // small residual
			}
		}
	}

	// Step 2: Iterative belief propagation (forward pass along topological order)
	// No recursion — iterate through pre-computed topological order
	proofTrace := make([]string, 0, len(net.Order)+2)
	proofTrace = append(proofTrace, fmt.Sprintf("evidence: %d features observed", len(datum.Features)))

	for oi := 0; oi < len(net.Order); oi++ {
		nodeName := net.Order[oi]
		node := net.Nodes[nodeName]

		if len(node.Parents) == 0 {
			// Root node: use prior or evidence
			hasCPT := false
			for _, s := range node.States {
				key := "|" + s
				if p, ok := node.CPT[key]; ok {
					// Combine prior with evidence using Bayesian update
					node.Belief[s] = node.Belief[s] * p
					hasCPT = true
				}
			}
			if hasCPT {
				normalize(node)
				proofTrace = append(proofTrace, fmt.Sprintf("prior(%s): beliefs updated", nodeName))
			}
			continue
		}

		// Non-root: propagate beliefs from parents iteratively
		for si := 0; si < len(node.States); si++ {
			state := node.States[si]
			totalProb := 0.0

			// Sum over all parent state combinations (iterative)
			for pi := 0; pi < len(node.Parents); pi++ {
				parentName := node.Parents[pi]
				parent, ok := net.Nodes[parentName]
				if !ok {
					continue
				}

				for _, pState := range parent.States {
					key := pState + "|" + state
					cpt, ok := node.CPT[key]
					if !ok {
						cpt = 1.0 / float64(len(node.States)) // uniform default
					}
					totalProb += cpt * parent.Belief[pState]
				}
			}

			if totalProb > 0 {
				node.Belief[state] = totalProb
			}
		}

		normalize(node)
		proofTrace = append(proofTrace,
			fmt.Sprintf("propagate(%s): P(high)=%.3f P(low)=%.3f",
				nodeName, node.Belief["high"], node.Belief["low"]))
	}

	// Step 3: Extract prediction from output node (last in topological order)
	prediction := "unknown"
	confidence := 0.0

	if len(net.Order) > 0 {
		outputName := net.Order[len(net.Order)-1]
		outputNode := net.Nodes[outputName]

		bestState := ""
		bestProb := 0.0
		for _, s := range outputNode.States {
			if outputNode.Belief[s] > bestProb {
				bestProb = outputNode.Belief[s]
				bestState = s
			}
		}

		if label, ok := net.Labels[bestState]; ok {
			prediction = label
		} else {
			prediction = bestState
		}
		confidence = bestProb
		proofTrace = append(proofTrace,
			fmt.Sprintf("conclusion: %s=P(%.3f) via %s", prediction, confidence, outputName))
	}

	return kinds.ModelResult{
		Prediction: prediction,
		Confidence: clamp(confidence, 0.0, 1.0),
		Kind:       kinds.KindBayesNets,
		ProofTrace: proofTrace,
	}, nil
}

// normalize ensures beliefs in a node sum to 1.
func normalize(node *Node) {
	total := 0.0
	for _, s := range node.States {
		total += node.Belief[s]
	}
	if total > 0 && !math.IsNaN(total) && !math.IsInf(total, 0) {
		for _, s := range node.States {
			node.Belief[s] /= total
		}
	}
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

// ResetBeliefs resets all node beliefs to uniform for a new inference.
func (e *Engine) ResetBeliefs() {
	for _, node := range e.Net.Nodes {
		for _, s := range node.States {
			node.Belief[s] = 1.0 / float64(len(node.States))
		}
	}
}
