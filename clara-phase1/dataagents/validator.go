package dataagents

import (
	"fmt"
	"math"

	"github.com/clara-phase1/kinds"
)

// ValidationIssue describes a data quality problem.
type ValidationIssue struct {
	Severity string // "error", "warning", "info"
	Field    string
	Message  string
}

// ValidationReport is the output of the ValidatorAgent.
type ValidationReport struct {
	DatasetName string
	Valid       bool
	Issues      []ValidationIssue
	NumItems    int
	NumFeatures int
	LabelCounts map[string]int
}

// ValidatorAgent checks datasets for quality and correctness.
type ValidatorAgent struct {
	AgentID string
	Log     []string
}

func NewValidatorAgent(id string) *ValidatorAgent {
	return &ValidatorAgent{AgentID: id}
}

func (v *ValidatorAgent) ID() string { return v.AgentID }

// Validate checks a dataset for data quality issues.
func (v *ValidatorAgent) Validate(ds kinds.DataSet) ValidationReport {
	v.log("validating dataset: %s (%d items)", ds.Name, len(ds.Items))

	report := ValidationReport{
		DatasetName: ds.Name,
		Valid:       true,
		NumItems:    len(ds.Items),
		LabelCounts: make(map[string]int),
	}

	if len(ds.Items) == 0 {
		report.Valid = false
		report.Issues = append(report.Issues, ValidationIssue{
			Severity: "error", Field: "dataset", Message: "dataset is empty",
		})
		return report
	}

	// Count features from first item
	firstFeatures := ds.Items[0].Features
	report.NumFeatures = len(firstFeatures)

	// Track per-feature stats
	featureVals := make(map[string][]float64)

	for i := 0; i < len(ds.Items); i++ {
		item := ds.Items[i]

		// Check feature count consistency
		if len(item.Features) != report.NumFeatures {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "warning",
				Field:    fmt.Sprintf("item[%d]", i),
				Message:  fmt.Sprintf("feature count %d != expected %d", len(item.Features), report.NumFeatures),
			})
		}

		// Check for empty label
		if item.Label == "" {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "error",
				Field:    fmt.Sprintf("item[%d].label", i),
				Message:  "empty label",
			})
			report.Valid = false
		}
		report.LabelCounts[item.Label]++

		// Check feature values
		for fname, fval := range item.Features {
			featureVals[fname] = append(featureVals[fname], fval)

			if math.IsNaN(fval) || math.IsInf(fval, 0) {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("item[%d].%s", i, fname),
					Message:  fmt.Sprintf("invalid value: %v", fval),
				})
				report.Valid = false
			}

			if fval < 0 || fval > 1 {
				report.Issues = append(report.Issues, ValidationIssue{
					Severity: "warning",
					Field:    fmt.Sprintf("item[%d].%s", i, fname),
					Message:  fmt.Sprintf("value %.4f outside [0,1] — may need normalization", fval),
				})
			}
		}
	}

	// Check class imbalance
	maxCount, minCount := 0, len(ds.Items)
	for _, count := range report.LabelCounts {
		if count > maxCount {
			maxCount = count
		}
		if count < minCount {
			minCount = count
		}
	}
	if len(report.LabelCounts) > 1 && minCount > 0 {
		ratio := float64(maxCount) / float64(minCount)
		if ratio > 3.0 {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "warning",
				Field:    "labels",
				Message:  fmt.Sprintf("class imbalance ratio %.1f:1 (max=%d, min=%d)", ratio, maxCount, minCount),
			})
		}
	}

	// Check for low-variance features
	for fname, vals := range featureVals {
		if len(vals) < 2 {
			continue
		}
		mean := 0.0
		for _, v := range vals {
			mean += v
		}
		mean /= float64(len(vals))
		variance := 0.0
		for _, v := range vals {
			diff := v - mean
			variance += diff * diff
		}
		variance /= float64(len(vals))
		if variance < 0.001 {
			report.Issues = append(report.Issues, ValidationIssue{
				Severity: "info",
				Field:    fname,
				Message:  fmt.Sprintf("near-zero variance (%.6f) — feature may not be informative", variance),
			})
		}
	}

	errorCount := 0
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			errorCount++
		}
	}
	report.Valid = errorCount == 0

	v.log("validation complete: valid=%v, %d issues (%d errors)",
		report.Valid, len(report.Issues), errorCount)
	return report
}

func (v *ValidatorAgent) log(format string, args ...interface{}) {
	v.Log = append(v.Log, fmt.Sprintf(format, args...))
}
