package dataagents

import (
	"fmt"

	"github.com/clara-phase1/kinds"
)

// RegistryEntry holds a dataset with all its metadata and quality info.
type RegistryEntry struct {
	Meta       DataMeta
	Dataset    kinds.DataSet
	Validation ValidationReport
	Quality    QualityScore
	Filed      bool
}

// RegistryAgent is the central catalog of all datasets.
// It coordinates discovery, loading, validation, and quality assessment.
type RegistryAgent struct {
	AgentID   string
	Entries   map[string]RegistryEntry
	Log       []string
	discovery *DiscoveryAgent
	loader    *LoaderAgent
	validator *ValidatorAgent
	quality   *QualityAgent
}

func NewRegistryAgent(id string, threshold float64) *RegistryAgent {
	return &RegistryAgent{
		AgentID:   id,
		Entries:   make(map[string]RegistryEntry),
		discovery: NewDiscoveryAgent(id + "-discovery"),
		loader:    NewLoaderAgent(id+"-loader", threshold),
		validator: NewValidatorAgent(id + "-validator"),
		quality:   NewQualityAgent(id + "-quality"),
	}
}

func (r *RegistryAgent) ID() string { return r.AgentID }

// DiscoverAndFile scans a directory, loads, validates, and files all datasets.
// This is the main entry point — runs all data agents in sequence.
func (r *RegistryAgent) DiscoverAndFile(dir string) error {
	r.log("=== Data Filing Pipeline: scanning %s ===", dir)

	// Step 1: Discovery
	metas, err := r.discovery.Discover(dir)
	if err != nil {
		r.log("discovery error: %v", err)
		return err
	}

	// Step 2: Load, validate, assess quality for each dataset (iterative)
	for i := 0; i < len(metas); i++ {
		meta := metas[i]
		r.log("processing: %s", meta.Name)

		// Load
		ds, err := r.loader.Load(meta)
		if err != nil {
			r.log("load error for %s: %v — skipping", meta.Name, err)
			continue
		}

		// Validate
		validation := r.validator.Validate(ds)
		if !validation.Valid {
			r.log("validation FAILED for %s — filing anyway with warnings", meta.Name)
		}

		// Quality assessment
		quality := r.quality.Assess(ds)

		// Register
		entry := RegistryEntry{
			Meta:       meta,
			Dataset:    ds,
			Validation: validation,
			Quality:    quality,
			Filed:      true,
		}
		r.Entries[meta.Name] = entry
		r.log("filed: %s (valid=%v, quality=%.2f, items=%d)",
			meta.Name, validation.Valid, quality.OverallScore, len(ds.Items))
	}

	r.log("=== Data Filing Complete: %d datasets registered ===", len(r.Entries))
	return nil
}

// RegisterBuiltin adds a pre-built dataset to the registry.
func (r *RegistryAgent) RegisterBuiltin(ds kinds.DataSet, domain string) {
	meta := DataMeta{
		Name:     ds.Name,
		Format:   "builtin",
		Domain:   domain,
		NumItems: len(ds.Items),
	}
	if len(ds.Items) > 0 {
		for fname := range ds.Items[0].Features {
			meta.FeatureNames = append(meta.FeatureNames, fname)
		}
		meta.NumFeatures = len(meta.FeatureNames)
	}

	validation := r.validator.Validate(ds)
	quality := r.quality.Assess(ds)

	r.Entries[ds.Name] = RegistryEntry{
		Meta:       meta,
		Dataset:    ds,
		Validation: validation,
		Quality:    quality,
		Filed:      true,
	}
	r.log("registered builtin: %s (domain=%s, items=%d, quality=%.2f)",
		ds.Name, domain, len(ds.Items), quality.OverallScore)
}

// GetByDomain returns all datasets matching a domain.
func (r *RegistryAgent) GetByDomain(domain string) []RegistryEntry {
	var result []RegistryEntry
	for _, entry := range r.Entries {
		if entry.Meta.Domain == domain {
			result = append(result, entry)
		}
	}
	return result
}

// GetAll returns all registered datasets.
func (r *RegistryAgent) GetAll() []RegistryEntry {
	result := make([]RegistryEntry, 0, len(r.Entries))
	for _, entry := range r.Entries {
		result = append(result, entry)
	}
	return result
}

// GetValid returns only datasets that passed validation.
func (r *RegistryAgent) GetValid() []RegistryEntry {
	var result []RegistryEntry
	for _, entry := range r.Entries {
		if entry.Validation.Valid {
			result = append(result, entry)
		}
	}
	return result
}

// Summary returns a text summary of all registered datasets.
func (r *RegistryAgent) Summary() string {
	s := fmt.Sprintf("Data Registry: %d datasets\n", len(r.Entries))
	for name, entry := range r.Entries {
		s += fmt.Sprintf("  [%s] domain=%s items=%d features=%d valid=%v quality=%.2f format=%s\n",
			name, entry.Meta.Domain, entry.Meta.NumItems, entry.Meta.NumFeatures,
			entry.Validation.Valid, entry.Quality.OverallScore, entry.Meta.Format)
	}
	return s
}

// SubAgentLogs returns all logs from sub-agents.
func (r *RegistryAgent) SubAgentLogs() map[string][]string {
	return map[string][]string{
		"discovery": r.discovery.Log,
		"loader":    r.loader.Log,
		"validator": r.validator.Log,
		"quality":   r.quality.Log,
	}
}

func (r *RegistryAgent) log(format string, args ...interface{}) {
	r.Log = append(r.Log, fmt.Sprintf(format, args...))
}
