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

// ModelResult holds the output from a single ML or AR inference.
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

// ComposedResult holds the combined ML+AR output.
type ComposedResult struct {
	MLResult  ModelResult
	ARResult  ModelResult
	Final     string
	Verified  bool
	AUROC     float64
	Explained bool
}

// Summary returns a one-line summary string.
func (cr ComposedResult) Summary() string {
	return fmt.Sprintf("Final=%s Verified=%v AUROC=%.4f ML=%s(%.2f) AR=%s(%.2f)",
		cr.Final, cr.Verified, cr.AUROC,
		cr.MLResult.Prediction, cr.MLResult.Confidence,
		cr.ARResult.Prediction, cr.ARResult.Confidence)
}

// Metric captures a named Phase 1 metric evaluation.
type Metric struct {
	Name   string
	Value  float64
	Target float64
	Pass   bool
	Detail string
}
