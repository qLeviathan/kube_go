// Package futures implements predictive future chaining for CARLA.
// It looks at current facts and predicts what rules will fire,
// pre-computes likely inference paths, and caches predictions.
// Learns prediction accuracy over time via the memory store.
// Future chaining happens at the swarm level: predictors warm caches
// ahead of actual inference, enabling autoscaled pre-computation.
package futures

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/clara-phase1/ar"
	"github.com/clara-phase1/memory"
)

// Prediction represents a predicted inference outcome.
type Prediction struct {
	LikelyRules []string `json:"likely_rules"`
	LikelyLabel string   `json:"likely_label"`
	Confidence  float64  `json:"confidence"`
	ChainDepth  int      `json:"chain_depth"`
	CacheHit    bool     `json:"cache_hit"`
}

// FutureStep represents one step in a predicted inference chain.
type FutureStep struct {
	Depth       int      `json:"depth"`
	RuleID      string   `json:"rule_id"`
	WouldDerive string   `json:"would_derive"`
	Probability float64  `json:"probability"`
	DependsOn   []string `json:"depends_on"`
}

// FutureChain is a sequence of predicted rule firings.
type FutureChain struct {
	Steps []FutureStep `json:"steps"`
}

// PredictorStats tracks predictor performance.
type PredictorStats struct {
	TotalPredictions int     `json:"total_predictions"`
	CacheHits        int     `json:"cache_hits"`
	CorrectPredictions int   `json:"correct_predictions"`
	Accuracy         float64 `json:"accuracy"`
}

// Predictor predicts inference outcomes before running the full engine.
// It uses memory patterns and rule structure to anticipate results.
type Predictor struct {
	Memory    *memory.Store
	Rules     ar.RuleSet
	RuleIndex map[string]ar.RuleSpec
	Labels    map[string]string // head predicate -> label
	Cache     map[string]Prediction
	MaxDepth  int
	Stats     PredictorStats
	mu        sync.Mutex
}

// NewPredictor creates a future predictor from a rule set and memory store.
func NewPredictor(mem *memory.Store, rules ar.RuleSet, maxDepth int) *Predictor {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	p := &Predictor{
		Memory:    mem,
		Rules:     rules,
		RuleIndex: make(map[string]ar.RuleSpec),
		Labels:    make(map[string]string),
		Cache:     make(map[string]Prediction),
		MaxDepth:  maxDepth,
	}

	for _, r := range rules.Rules {
		p.RuleIndex[r.ID] = r
	}
	for _, l := range rules.Labels {
		p.Labels[l.Atom] = l.Label
	}

	return p
}

// Predict predicts the inference outcome for a set of feature facts.
// Checks cache first, then builds a future chain.
func (p *Predictor) Predict(featureFacts []string) Prediction {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Stats.TotalPredictions++

	key := cacheKey(featureFacts)

	// Check cache
	if cached, ok := p.Cache[key]; ok {
		cached.CacheHit = true
		p.Stats.CacheHits++
		return cached
	}

	// Check memory for historical pattern
	featureNames := extractFeatureNames(featureFacts)
	if label, conf, found := p.Memory.PredictFromHistory(featureNames); found {
		pred := Prediction{
			LikelyLabel: label,
			Confidence:  conf,
			ChainDepth:  0,
			CacheHit:    false,
		}
		p.Cache[key] = pred
		return pred
	}

	// Build future chain from rules
	chain := p.buildChain(featureFacts)
	pred := p.chainToPrediction(chain)
	p.Cache[key] = pred
	return pred
}

// BuildChain traces the full predicted chain of rule firings.
func (p *Predictor) BuildChain(featureFacts []string) FutureChain {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buildChain(featureFacts)
}

// buildChain does the actual chain building (caller holds lock).
func (p *Predictor) buildChain(featureFacts []string) FutureChain {
	chain := FutureChain{}

	// Start with known facts
	derived := make(map[string]bool)
	for _, f := range featureFacts {
		derived[f] = true
	}

	// Iteratively predict rule firings up to max depth
	for depth := 0; depth < p.MaxDepth; depth++ {
		newDerived := false

		for _, rule := range p.Rules.Rules {
			headKey := atomSpecToString(rule.Head)
			if derived[headKey] {
				continue
			}

			// Count satisfied body atoms
			satisfied := 0
			total := len(rule.Body)
			dependsOn := make([]string, 0)

			for _, bodyAtom := range rule.Body {
				bodyKey := atomSpecToString(bodyAtom)
				if derived[bodyKey] {
					satisfied++
				} else {
					dependsOn = append(dependsOn, bodyKey)
				}
			}

			if total == 0 {
				continue
			}

			probability := float64(satisfied) / float64(total)

			// If all body atoms satisfied, rule WILL fire
			if satisfied == total {
				derived[headKey] = true
				newDerived = true
				chain.Steps = append(chain.Steps, FutureStep{
					Depth:       depth,
					RuleID:      rule.ID,
					WouldDerive: headKey,
					Probability: 1.0,
					DependsOn:   nil,
				})
			} else if probability >= 0.5 {
				// Partially satisfied — predict likelihood
				chain.Steps = append(chain.Steps, FutureStep{
					Depth:       depth,
					RuleID:      rule.ID,
					WouldDerive: headKey,
					Probability: probability,
					DependsOn:   dependsOn,
				})
			}
		}

		if !newDerived {
			break
		}
	}

	return chain
}

// chainToPrediction converts a future chain into a prediction.
func (p *Predictor) chainToPrediction(chain FutureChain) Prediction {
	pred := Prediction{
		LikelyRules: make([]string, 0),
		ChainDepth:  0,
	}

	// Find the deepest certain rule firing that maps to a label
	bestConf := 0.0

	for _, step := range chain.Steps {
		pred.LikelyRules = append(pred.LikelyRules, step.RuleID)
		if step.Depth+1 > pred.ChainDepth {
			pred.ChainDepth = step.Depth + 1
		}

		// Check if this derivation maps to a label
		if label, ok := p.Labels[step.WouldDerive]; ok {
			if step.Probability > bestConf {
				bestConf = step.Probability
				pred.LikelyLabel = label
				pred.Confidence = step.Probability
			}
		}
	}

	return pred
}

// WarmCache pre-computes predictions for known data patterns from memory.
func (p *Predictor) WarmCache(topN int) int {
	patterns := p.Memory.GetTopPatterns(topN)
	warmed := 0

	for _, pattern := range patterns {
		features := strings.Split(pattern.FeatureKey, ",")
		// Convert feature names to fact strings
		facts := make([]string, 0, len(features))
		for _, f := range features {
			if f != "" {
				facts = append(facts, fmt.Sprintf("has_feature(%s, high)", f))
			}
		}

		if len(facts) > 0 {
			p.Predict(facts)
			warmed++
		}
	}

	return warmed
}

// RecordOutcome updates prediction accuracy based on actual results.
func (p *Predictor) RecordOutcome(featureFacts []string, actualLabel string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := cacheKey(featureFacts)
	if cached, ok := p.Cache[key]; ok {
		if cached.LikelyLabel == actualLabel {
			p.Stats.CorrectPredictions++
		}
	}

	if p.Stats.TotalPredictions > 0 {
		p.Stats.Accuracy = float64(p.Stats.CorrectPredictions) / float64(p.Stats.TotalPredictions)
	}
}

// ClearCache empties the prediction cache.
func (p *Predictor) ClearCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Cache = make(map[string]Prediction)
}

// Summary returns a human-readable summary.
func (p *Predictor) Summary() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return fmt.Sprintf("Futures: %d cached, %d predictions, %d hits (%.1f%% accuracy)",
		len(p.Cache), p.Stats.TotalPredictions, p.Stats.CacheHits, p.Stats.Accuracy*100)
}

// cacheKey creates a deterministic key from feature facts.
func cacheKey(facts []string) string {
	sorted := make([]string, len(facts))
	copy(sorted, facts)
	sort.Strings(sorted)
	return strings.Join(sorted, "|")
}

// extractFeatureNames pulls feature names from fact strings like "has_feature(X, high)".
func extractFeatureNames(facts []string) []string {
	names := make([]string, 0, len(facts))
	for _, f := range facts {
		// Parse "has_feature(name, value)"
		if strings.HasPrefix(f, "has_feature(") && strings.HasSuffix(f, ")") {
			inner := f[len("has_feature(") : len(f)-1]
			parts := strings.SplitN(inner, ", ", 2)
			if len(parts) >= 1 {
				names = append(names, parts[0])
			}
		}
	}
	return names
}

// atomSpecToString converts an AtomSpec to its string representation.
func atomSpecToString(a ar.AtomSpec) string {
	if len(a.Args) == 0 {
		return a.Predicate
	}
	return fmt.Sprintf("%s(%s)", a.Predicate, strings.Join(a.Args, ", "))
}
