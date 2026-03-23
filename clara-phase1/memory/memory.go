// Package memory implements a persistent learning store for CLARA.
// Facts, rule performance, inference patterns, and run history survive across runs.
// All data is JSON-backed and loaded/saved to a single file.
// Agents query memory during inference to leverage past experience.
package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store is the persistent memory for the CLARA system.
// It tracks learned facts, rule performance, inference patterns, and run history.
// Thread-safe for concurrent agent access.
type Store struct {
	Path       string              `json:"-"`
	Facts      []MemoryFact        `json:"facts"`
	RuleStats  []RulePerformance   `json:"rule_stats"`
	RunHistory []RunSummary        `json:"run_history"`
	Patterns   []InferencePattern  `json:"patterns"`
	mu         sync.Mutex
}

// MemoryFact is a learned fact from a past run.
type MemoryFact struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Domain    string    `json:"domain"`
	LearnedAt time.Time `json:"learned_at"`
	Source    string    `json:"source"`
	Uses     int       `json:"uses"`
}

// RulePerformance tracks how well a specific AR rule performs across runs.
type RulePerformance struct {
	RuleID       string    `json:"rule_id"`
	FireCount    int       `json:"fire_count"`
	CorrectCount int       `json:"correct_count"`
	Accuracy     float64   `json:"accuracy"`
	LastUsed     time.Time `json:"last_used"`
	Domains      []string  `json:"domains"`
}

// RunSummary captures the outcome of a complete pipeline run.
type RunSummary struct {
	Timestamp  time.Time     `json:"timestamp"`
	Datasets   int           `json:"datasets"`
	Accuracy   float64       `json:"accuracy"`
	RulesUsed  int           `json:"rules_used"`
	RulesFired int           `json:"rules_fired"`
	DurationMs int64         `json:"duration_ms"`
	Rewrites   int           `json:"rewrites"`
	SwarmPeak  int           `json:"swarm_peak"`
}

// InferencePattern tracks recurring feature→prediction associations.
type InferencePattern struct {
	FeatureKey string  `json:"feature_key"` // sorted, comma-joined feature names
	Prediction string  `json:"prediction"`
	Confidence float64 `json:"confidence"`
	Frequency  int     `json:"frequency"`
}

// NewStore loads a memory store from file, or creates an empty one.
func NewStore(path string) *Store {
	s := &Store{
		Path:       path,
		Facts:      []MemoryFact{},
		RuleStats:  []RulePerformance{},
		RunHistory: []RunSummary{},
		Patterns:   []InferencePattern{},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}

	loaded := &Store{}
	if err := json.Unmarshal(data, loaded); err != nil {
		return s
	}

	loaded.Path = path
	if loaded.Facts == nil {
		loaded.Facts = []MemoryFact{}
	}
	if loaded.RuleStats == nil {
		loaded.RuleStats = []RulePerformance{}
	}
	if loaded.RunHistory == nil {
		loaded.RunHistory = []RunSummary{}
	}
	if loaded.Patterns == nil {
		loaded.Patterns = []InferencePattern{}
	}
	return loaded
}

// Save persists the store to its JSON file.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("memory: marshal error: %w", err)
	}
	return os.WriteFile(s.Path, data, 0644)
}

// RecordRuleFire tracks a rule firing and whether it produced a correct result.
func (s *Store) RecordRuleFire(ruleID string, correct bool, domain string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := 0; i < len(s.RuleStats); i++ {
		if s.RuleStats[i].RuleID == ruleID {
			s.RuleStats[i].FireCount++
			if correct {
				s.RuleStats[i].CorrectCount++
			}
			if s.RuleStats[i].FireCount > 0 {
				s.RuleStats[i].Accuracy = float64(s.RuleStats[i].CorrectCount) / float64(s.RuleStats[i].FireCount)
			}
			s.RuleStats[i].LastUsed = time.Now()
			if !containsStr(s.RuleStats[i].Domains, domain) {
				s.RuleStats[i].Domains = append(s.RuleStats[i].Domains, domain)
			}
			return
		}
	}

	// New rule
	acc := 0.0
	cc := 0
	if correct {
		acc = 1.0
		cc = 1
	}
	s.RuleStats = append(s.RuleStats, RulePerformance{
		RuleID:       ruleID,
		FireCount:    1,
		CorrectCount: cc,
		Accuracy:     acc,
		LastUsed:     time.Now(),
		Domains:      []string{domain},
	})
}

// RecordRun saves a run summary.
func (s *Store) RecordRun(summary RunSummary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RunHistory = append(s.RunHistory, summary)
}

// LearnFact stores a new fact, or increments usage if it already exists.
func (s *Store) LearnFact(key, value, domain, source string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := 0; i < len(s.Facts); i++ {
		if s.Facts[i].Key == key && s.Facts[i].Value == value && s.Facts[i].Domain == domain {
			s.Facts[i].Uses++
			return
		}
	}

	s.Facts = append(s.Facts, MemoryFact{
		Key:       key,
		Value:     value,
		Domain:    domain,
		LearnedAt: time.Now(),
		Source:    source,
		Uses:     1,
	})
}

// RecordPattern tracks a feature→prediction association.
func (s *Store) RecordPattern(features []string, prediction string, confidence float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := featureKey(features)

	for i := 0; i < len(s.Patterns); i++ {
		if s.Patterns[i].FeatureKey == key && s.Patterns[i].Prediction == prediction {
			s.Patterns[i].Frequency++
			// Running average of confidence
			s.Patterns[i].Confidence = (s.Patterns[i].Confidence*float64(s.Patterns[i].Frequency-1) + confidence) / float64(s.Patterns[i].Frequency)
			return
		}
	}

	s.Patterns = append(s.Patterns, InferencePattern{
		FeatureKey: key,
		Prediction: prediction,
		Confidence: confidence,
		Frequency:  1,
	})
}

// QueryFacts returns all facts for a given domain.
func (s *Store) QueryFacts(domain string) []MemoryFact {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []MemoryFact
	for i := 0; i < len(s.Facts); i++ {
		if s.Facts[i].Domain == domain || domain == "" {
			s.Facts[i].Uses++
			result = append(result, s.Facts[i])
		}
	}
	return result
}

// GetRuleStats returns performance data for a specific rule.
func (s *Store) GetRuleStats(ruleID string) *RulePerformance {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := 0; i < len(s.RuleStats); i++ {
		if s.RuleStats[i].RuleID == ruleID {
			cp := s.RuleStats[i]
			return &cp
		}
	}
	return nil
}

// AllRuleStats returns all rule performance data.
func (s *Store) AllRuleStats() []RulePerformance {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]RulePerformance, len(s.RuleStats))
	copy(result, s.RuleStats)
	return result
}

// GetTopPatterns returns the N most frequent inference patterns.
func (s *Store) GetTopPatterns(n int) []InferencePattern {
	s.mu.Lock()
	defer s.mu.Unlock()

	sorted := make([]InferencePattern, len(s.Patterns))
	copy(sorted, s.Patterns)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Frequency > sorted[j].Frequency
	})

	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

// PredictFromHistory tries to predict a label from feature patterns seen before.
// Returns prediction, confidence, and whether a match was found.
func (s *Store) PredictFromHistory(features []string) (string, float64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := featureKey(features)

	var best *InferencePattern
	for i := 0; i < len(s.Patterns); i++ {
		if s.Patterns[i].FeatureKey == key {
			if best == nil || s.Patterns[i].Frequency > best.Frequency {
				p := s.Patterns[i]
				best = &p
			}
		}
	}

	if best != nil && best.Frequency >= 3 {
		return best.Prediction, best.Confidence, true
	}
	return "", 0, false
}

// TotalRuns returns how many runs have been recorded.
func (s *Store) TotalRuns() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.RunHistory)
}

// Summary returns a human-readable summary of the memory store.
func (s *Store) Summary() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return fmt.Sprintf("Memory: %d facts, %d rule stats, %d patterns, %d past runs",
		len(s.Facts), len(s.RuleStats), len(s.Patterns), len(s.RunHistory))
}

// featureKey creates a deterministic key from a list of features.
func featureKey(features []string) string {
	sorted := make([]string, len(features))
	copy(sorted, features)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
