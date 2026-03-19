package dataagents

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/clara-phase1/kinds"
)

// LoaderAgent loads datasets from files into kinds.DataSet.
type LoaderAgent struct {
	AgentID          string
	FeatureThreshold float64 // for validation; actual discretization is in engines
	Log              []string
}

func NewLoaderAgent(id string, threshold float64) *LoaderAgent {
	return &LoaderAgent{AgentID: id, FeatureThreshold: threshold}
}

func (l *LoaderAgent) ID() string { return l.AgentID }

// Load reads a dataset from a file based on its metadata.
func (l *LoaderAgent) Load(meta DataMeta) (kinds.DataSet, error) {
	l.log("loading dataset: %s from %s", meta.Name, meta.FilePath)

	var items []kinds.Datum
	var err error

	switch meta.Format {
	case "csv":
		items, err = l.loadCSV(meta)
	case "json":
		items, err = l.loadJSON(meta)
	default:
		return kinds.DataSet{}, fmt.Errorf("unsupported format: %s", meta.Format)
	}
	if err != nil {
		return kinds.DataSet{}, err
	}

	ds := kinds.DataSet{
		Name:  meta.Name,
		Items: items,
		Split: "test",
	}

	l.log("loaded %d items with %d features from %s", len(items), meta.NumFeatures, meta.Name)
	return ds, nil
}

func (l *LoaderAgent) loadCSV(meta DataMeta) ([]kinds.Datum, error) {
	f, err := os.Open(meta.FilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("csv needs header + data")
	}

	headers := records[0]
	labelIdx := len(headers) - 1
	featureNames := headers[:labelIdx]

	items := make([]kinds.Datum, 0, len(records)-1)
	for ri := 1; ri < len(records); ri++ {
		row := records[ri]
		if len(row) < len(headers) {
			l.log("skipping row %d: incomplete (%d cols, expected %d)", ri, len(row), len(headers))
			continue
		}

		features := make(map[string]float64)
		for fi := 0; fi < len(featureNames); fi++ {
			v, err := strconv.ParseFloat(row[fi], 64)
			if err != nil {
				l.log("row %d col %s: non-numeric value %q, using 0.0", ri, featureNames[fi], row[fi])
				v = 0.0
			}
			features[featureNames[fi]] = v
		}

		items = append(items, kinds.Datum{
			Features: features,
			Label:    row[labelIdx],
		})
	}

	return items, nil
}

func (l *LoaderAgent) loadJSON(meta DataMeta) ([]kinds.Datum, error) {
	data, err := os.ReadFile(meta.FilePath)
	if err != nil {
		return nil, err
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	items := make([]kinds.Datum, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		entry := raw[i]
		features := make(map[string]float64)
		label := ""

		for key, val := range entry {
			if key == "label" {
				label = fmt.Sprintf("%v", val)
				continue
			}
			switch v := val.(type) {
			case float64:
				features[key] = v
			case string:
				fv, err := strconv.ParseFloat(v, 64)
				if err == nil {
					features[key] = fv
				}
			}
		}

		items = append(items, kinds.Datum{
			Features: features,
			Label:    label,
		})
	}

	return items, nil
}

func (l *LoaderAgent) log(format string, args ...interface{}) {
	l.Log = append(l.Log, fmt.Sprintf(format, args...))
}
