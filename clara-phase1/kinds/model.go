package kinds

import "fmt"

// Datum represents a single input-output pair for inference or evaluation.
type Datum struct {
	Features map[string]float64
	Label    string
	Score    float64
}

// DataSet is a named collection of data for evaluation.
type DataSet struct {
	Name  string
	Items []Datum
	Split string // "train", "test", "val"
}

// ModelResult holds the output from a single AR inference.
type ModelResult struct {
	Prediction string
	Confidence float64
	Kind       Kind
	ProofTrace []string // logical explanation steps, never nil
}

// NewModelResult creates a ModelResult with an initialized (non-nil) proof trace.
func NewModelResult(prediction string, confidence float64, kind Kind, trace []string) ModelResult {
	if trace == nil {
		trace = []string{}
	}
	return ModelResult{
		Prediction: prediction,
		Confidence: confidence,
		Kind:       kind,
		ProofTrace: trace,
	}
}

// InferenceResult holds the output of the rules engine pipeline.
// Replaces the old ComposedResult (which merged ML+AR).
type InferenceResult struct {
	Result    ModelResult
	Final     string
	Verified  bool
	Confidence float64
	Explained bool
}

// Summary returns a one-line summary string.
func (ir InferenceResult) Summary() string {
	return fmt.Sprintf("Final=%s Verified=%v Confidence=%.4f AR=%s(%.2f)",
		ir.Final, ir.Verified, ir.Confidence,
		ir.Result.Prediction, ir.Result.Confidence)
}

// Metric captures a named evaluation metric.
type Metric struct {
	Name   string
	Value  float64
	Target float64
	Pass   bool
	Detail string
}
