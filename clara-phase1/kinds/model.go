package kinds

import "fmt"

// Datum represents a single input-output pair for training or inference.
type Datum struct {
	Features map[string]float64
	Label    string
	Score    float64 // confidence or probability
}

// DataSet is a named collection of data for training or evaluation.
type DataSet struct {
	Name   string
	Items  []Datum
	Split  string // "train", "test", "val"
}

// ModelResult holds the output from ML or AR inference on a single datum.
type ModelResult struct {
	Prediction string
	Confidence float64
	Kind       Kind
	ProofTrace []string // logical explanation steps
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

func (cr ComposedResult) Summary() string {
	return fmt.Sprintf("Final=%s Verified=%v AUROC=%.4f ML=%s(%.2f) AR=%s(%.2f)",
		cr.Final, cr.Verified, cr.AUROC,
		cr.MLResult.Prediction, cr.MLResult.Confidence,
		cr.ARResult.Prediction, cr.ARResult.Confidence)
}

// Metric captures a named Phase 1 metric evaluation.
type Metric struct {
	Name     string
	Value    float64
	Target   float64
	Pass     bool
	Detail   string
}
