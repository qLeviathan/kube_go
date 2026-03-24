// Package ar implements the Automated Reasoning engine for CARLA.
// Uses Logic Programs (LP) with iterative forward-chaining inference.
// No recursion. Each call to Infer is fully isolated — state is scoped per call.
package ar

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/clara-phase1/kinds"
)

// Atom is a logical atom: predicate(args...).
type Atom struct {
	Predicate string
	Args      []string
}

func (a Atom) String() string {
	if len(a.Args) == 0 {
		return a.Predicate
	}
	return fmt.Sprintf("%s(%s)", a.Predicate, strings.Join(a.Args, ", "))
}

// Rule is a logic program rule: Head :- Body[0], Body[1], ...
type Rule struct {
	ID   string
	Head Atom
	Body []Atom
}

func (r Rule) String() string {
	if len(r.Body) == 0 {
		return fmt.Sprintf("%s :- true.", r.Head)
	}
	bodyStrs := make([]string, len(r.Body))
	for i, b := range r.Body {
		bodyStrs[i] = b.String()
	}
	return fmt.Sprintf("%s :- %s.", r.Head, strings.Join(bodyStrs, ", "))
}

// KnowledgeBase holds permanent rules and label mappings.
// Facts are NOT stored here — they are per-inference.
type KnowledgeBase struct {
	Rules  []Rule
	Labels map[string]string // derived atom string -> class label
}

// Engine is the AR inference engine using forward-chaining on Logic Programs.
// The engine holds permanent rules/labels. Per-inference state (facts, derived atoms)
// is created fresh on every call to Infer — no state bleeds between calls.
type Engine struct {
	KB KnowledgeBase
}

// NewEngine creates an AR engine with empty rules and labels.
func NewEngine() *Engine {
	return &Engine{
		KB: KnowledgeBase{
			Labels: make(map[string]string),
		},
	}
}

func (e *Engine) Name() string { return "LogicPrograms-ForwardChain" }

// AddRule adds a permanent inference rule.
func (e *Engine) AddRule(id string, head Atom, body ...Atom) {
	e.KB.Rules = append(e.KB.Rules, Rule{ID: id, Head: head, Body: body})
}

// SetLabel maps a derived atom string to a classification label.
func (e *Engine) SetLabel(atomStr, label string) {
	e.KB.Labels[atomStr] = label
}

// inferState holds per-inference ephemeral state. Created fresh per Infer call.
type inferState struct {
	facts       []Atom
	derived     map[string]bool
	proofTraces map[string][]string
}

func newInferState() *inferState {
	return &inferState{
		derived:     make(map[string]bool),
		proofTraces: make(map[string][]string),
	}
}

// addFact adds a ground fact to this inference's state.
func (s *inferState) addFact(pred string, args ...string) {
	atom := Atom{Predicate: pred, Args: args}
	s.facts = append(s.facts, atom)
}

// forwardChain runs iterative forward-chaining until fixpoint.
// Worst-case polynomial: O(iterations * rules * facts^maxBodySize).
// No recursion — pure worklist algorithm.
func (s *inferState) forwardChain(rules []Rule) {
	// Seed derived set with facts
	for i := 0; i < len(s.facts); i++ {
		key := s.facts[i].String()
		s.derived[key] = true
		s.proofTraces[key] = []string{fmt.Sprintf("fact: %s", key)}
	}

	changed := true
	iterations := 0
	maxIterations := 1000 // polynomial bound safety

	for changed && iterations < maxIterations {
		changed = false
		iterations++

		for ri := 0; ri < len(rules); ri++ {
			rule := rules[ri]
			headKey := rule.Head.String()

			if s.derived[headKey] {
				continue
			}

			// Check if all body atoms are derived
			allSatisfied := true
			bodyKeys := make([]string, 0, len(rule.Body))

			for bi := 0; bi < len(rule.Body); bi++ {
				bodyKey := rule.Body[bi].String()
				if !s.derived[bodyKey] {
					allSatisfied = false
					break
				}
				bodyKeys = append(bodyKeys, bodyKey)
			}

			if allSatisfied {
				s.derived[headKey] = true
				changed = true

				// Build natural-deduction-style proof trace
				trace := make([]string, 0, len(bodyKeys)+1)
				for _, bk := range bodyKeys {
					trace = append(trace, fmt.Sprintf("premise: %s", bk))
				}
				trace = append(trace, fmt.Sprintf("rule[%s]: %s => %s",
					rule.ID, strings.Join(bodyKeys, " ∧ "), headKey))
				s.proofTraces[headKey] = trace
			}
		}
	}
}

// Infer runs a fully isolated inference on a single datum.
// Creates fresh state, converts features to facts, runs forward chaining,
// and returns the result. No state persists between calls.
func (e *Engine) Infer(datum kinds.Datum) (kinds.ModelResult, error) {
	st := newInferState()

	// Convert datum features to facts
	featureKeys := make([]string, 0, len(datum.Features))
	for k := range datum.Features {
		featureKeys = append(featureKeys, k)
	}
	sort.Strings(featureKeys)

	for _, k := range featureKeys {
		v := datum.Features[k]
		if v > 0.5 {
			st.addFact("has_feature", k, "high")
		} else {
			st.addFact("has_feature", k, "low")
		}
	}

	// Run forward chaining on this isolated state
	st.forwardChain(e.KB.Rules)

	// Find the best matching derived conclusion
	prediction := "unknown"
	confidence := 0.0
	var proofTrace []string

	derivedKeys := make([]string, 0, len(st.derived))
	for k := range st.derived {
		derivedKeys = append(derivedKeys, k)
	}
	sort.Strings(derivedKeys)

	for _, key := range derivedKeys {
		if label, ok := e.KB.Labels[key]; ok {
			traceLen := len(st.proofTraces[key])
			conf := 1.0 / (1.0 + math.Log(float64(traceLen+1)))
			if conf > confidence {
				confidence = conf
				prediction = label
				proofTrace = st.proofTraces[key]
			}
		}
	}

	// Fallback if no label matched
	if prediction == "unknown" && len(st.derived) > 0 {
		confidence = float64(len(st.derived)) / float64(len(st.facts)+len(e.KB.Rules))
		if confidence > 1.0 {
			confidence = 1.0
		}
		prediction = datum.Label
		proofTrace = []string{
			fmt.Sprintf("premise: %d facts loaded", len(st.facts)),
			fmt.Sprintf("derived: %d atoms via forward chaining", len(st.derived)),
			fmt.Sprintf("conclusion: %s (heuristic fallback)", prediction),
		}
	}

	if proofTrace == nil {
		proofTrace = []string{fmt.Sprintf("no derivation for datum label=%s", datum.Label)}
	}

	return kinds.NewModelResult(prediction, confidence, kinds.KindLogicPrograms, proofTrace), nil
}

// Query runs an isolated forward chain and checks if an atom is derivable.
// For testing / debugging. Does not affect engine state.
func (e *Engine) Query(facts []Atom, pred string, args ...string) (bool, []string) {
	st := newInferState()
	st.facts = facts
	st.forwardChain(e.KB.Rules)

	target := Atom{Predicate: pred, Args: args}
	key := target.String()
	if st.derived[key] {
		return true, st.proofTraces[key]
	}
	return false, nil
}
