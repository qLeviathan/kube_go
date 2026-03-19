// Package dataagents implements autonomous data filing agents for CLARA Phase 1.
// Agents: Discovery, Loader, Validator, Quality, Registry.
// No recursion. All agents operate iteratively.
package dataagents

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/clara-phase1/kinds"
)

// DataMeta describes a discovered dataset.
type DataMeta struct {
	Name         string            `json:"name"`
	FilePath     string            `json:"file_path"`
	Format       string            `json:"format"` // "csv", "json", "builtin"
	Domain       string            `json:"domain"`
	FeatureNames []string          `json:"feature_names"`
	LabelName    string            `json:"label_name"`
	NumItems     int               `json:"num_items"`
	NumFeatures  int               `json:"num_features"`
	FeatureStats map[string]FeatureStat `json:"feature_stats"`
	DiscoveredAt time.Time         `json:"discovered_at"`
}

// FeatureStat holds statistics for a single feature column.
type FeatureStat struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
}

// --- Discovery Agent ---

// DiscoveryAgent scans directories to find and catalog datasets.
type DiscoveryAgent struct {
	AgentID string
	Log     []string
}

func NewDiscoveryAgent(id string) *DiscoveryAgent {
	return &DiscoveryAgent{AgentID: id}
}

func (d *DiscoveryAgent) ID() string { return d.AgentID }

// Discover scans a directory for CSV and JSON dataset files.
// Returns metadata for each discovered file. Iterative, no recursion.
func (d *DiscoveryAgent) Discover(dir string) ([]DataMeta, error) {
	d.log("scanning directory: %s", dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		d.log("directory not found or unreadable: %s", dir)
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var metas []DataMeta
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))

		if ext != ".csv" && ext != ".json" {
			continue
		}

		fullPath := filepath.Join(dir, name)
		format := strings.TrimPrefix(ext, ".")

		meta, err := d.inspectFile(fullPath, format)
		if err != nil {
			d.log("skipping %s: %v", name, err)
			continue
		}
		meta.DiscoveredAt = time.Now()
		metas = append(metas, meta)
		d.log("discovered: %s (%s, %d items, %d features)",
			meta.Name, meta.Format, meta.NumItems, meta.NumFeatures)
	}

	d.log("discovery complete: %d datasets found", len(metas))
	return metas, nil
}

func (d *DiscoveryAgent) inspectFile(path, format string) (DataMeta, error) {
	meta := DataMeta{
		Name:     strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		FilePath: path,
		Format:   format,
	}

	switch format {
	case "csv":
		return d.inspectCSV(meta)
	case "json":
		return d.inspectJSON(meta)
	default:
		return meta, fmt.Errorf("unsupported format: %s", format)
	}
}

func (d *DiscoveryAgent) inspectCSV(meta DataMeta) (DataMeta, error) {
	f, err := os.Open(meta.FilePath)
	if err != nil {
		return meta, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return meta, fmt.Errorf("parse csv: %w", err)
	}
	if len(records) < 2 {
		return meta, fmt.Errorf("csv needs header + at least 1 row")
	}

	headers := records[0]
	meta.LabelName = headers[len(headers)-1] // last column is label
	meta.FeatureNames = headers[:len(headers)-1]
	meta.NumFeatures = len(meta.FeatureNames)
	meta.NumItems = len(records) - 1

	// Compute feature stats
	meta.FeatureStats = make(map[string]FeatureStat)
	for fi := 0; fi < meta.NumFeatures; fi++ {
		fname := meta.FeatureNames[fi]
		var vals []float64
		for ri := 1; ri < len(records); ri++ {
			if fi < len(records[ri]) {
				v, err := strconv.ParseFloat(records[ri][fi], 64)
				if err == nil {
					vals = append(vals, v)
				}
			}
		}
		if len(vals) > 0 {
			meta.FeatureStats[fname] = computeStats(vals)
		}
	}

	// Infer domain from feature names
	meta.Domain = inferDomain(meta.FeatureNames)
	return meta, nil
}

func (d *DiscoveryAgent) inspectJSON(meta DataMeta) (DataMeta, error) {
	data, err := os.ReadFile(meta.FilePath)
	if err != nil {
		return meta, err
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(data, &items); err != nil {
		return meta, fmt.Errorf("parse json: %w", err)
	}
	if len(items) == 0 {
		return meta, fmt.Errorf("json dataset is empty")
	}

	// Extract feature names from first item
	featureSet := make(map[string]bool)
	for key := range items[0] {
		if key != "label" {
			featureSet[key] = true
		}
	}
	for key := range featureSet {
		meta.FeatureNames = append(meta.FeatureNames, key)
	}
	sort.Strings(meta.FeatureNames)
	meta.LabelName = "label"
	meta.NumFeatures = len(meta.FeatureNames)
	meta.NumItems = len(items)
	meta.Domain = inferDomain(meta.FeatureNames)

	return meta, nil
}

// InferDomainFromFeatures infers domain from a dataset's feature names.
func InferDomainFromFeatures(ds kinds.DataSet) string {
	if len(ds.Items) == 0 {
		return "general"
	}
	var features []string
	for fname := range ds.Items[0].Features {
		features = append(features, fname)
	}
	return inferDomain(features)
}

func inferDomain(features []string) string {
	joined := strings.Join(features, " ")
	switch {
	case strings.Contains(joined, "blood") || strings.Contains(joined, "glucose") || strings.Contains(joined, "heart"):
		return "medical"
	case strings.Contains(joined, "threat") || strings.Contains(joined, "supply") || strings.Contains(joined, "terrain"):
		return "coa"
	case strings.Contains(joined, "equipment") || strings.Contains(joined, "usage") || strings.Contains(joined, "failure"):
		return "supply-chain"
	default:
		return "general"
	}
}

func computeStats(vals []float64) FeatureStat {
	n := float64(len(vals))
	if n == 0 {
		return FeatureStat{}
	}
	min, max, sum := vals[0], vals[0], 0.0
	for i := 0; i < len(vals); i++ {
		if vals[i] < min {
			min = vals[i]
		}
		if vals[i] > max {
			max = vals[i]
		}
		sum += vals[i]
	}
	mean := sum / n
	variance := 0.0
	for i := 0; i < len(vals); i++ {
		diff := vals[i] - mean
		variance += diff * diff
	}
	variance /= n
	stddev := 0.0
	if variance > 0 {
		// iterative sqrt (Newton's method, no math import needed for this)
		stddev = variance
		for j := 0; j < 20; j++ {
			stddev = (stddev + variance/stddev) / 2.0
		}
	}
	return FeatureStat{Min: min, Max: max, Mean: mean, StdDev: stddev}
}

func (d *DiscoveryAgent) log(format string, args ...interface{}) {
	d.Log = append(d.Log, fmt.Sprintf(format, args...))
}
