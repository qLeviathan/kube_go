package compose

import (
	"testing"

	"github.com/clara-phase1/kinds"
)

type mockEngine struct {
	prediction string
	confidence float64
	cat        kinds.Category
}

func (m *mockEngine) Name() string { return "mock" }
func (m *mockEngine) Infer(datum kinds.Datum) (kinds.ModelResult, error) {
	k := kinds.KindBayesNets
	if m.cat == kinds.CategoryAR {
		k = kinds.KindLogicPrograms
	}
	return kinds.ModelResult{
		Prediction: m.prediction,
		Confidence: m.confidence,
		Kind:       k,
		ProofTrace: []string{"premise: mock", "rule: mock => " + m.prediction},
	}, nil
}

func TestPipelineARPriority(t *testing.T) {
	mlEng := &mockEngine{prediction: "A", confidence: 0.8, cat: kinds.CategoryML}
	arEng := &mockEngine{prediction: "A", confidence: 0.9, cat: kinds.CategoryAR}

	pipe := NewPipeline("test", mlEng, arEng, kinds.KindBayesNets, kinds.KindLogicPrograms, StrategyARPriority)

	datum := kinds.Datum{Features: map[string]float64{"x": 0.7}, Label: "A"}
	result, err := pipe.InferComposed(datum)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.Final != "A" {
		t.Errorf("expected A, got %s", result.Final)
	}
	if !result.Verified {
		t.Error("expected verified")
	}
	if !result.Explained {
		t.Error("expected explained")
	}
}

func TestPipelineDisagreement(t *testing.T) {
	mlEng := &mockEngine{prediction: "A", confidence: 0.6, cat: kinds.CategoryML}
	arEng := &mockEngine{prediction: "B", confidence: 0.8, cat: kinds.CategoryAR}

	pipe := NewPipeline("test", mlEng, arEng, kinds.KindBayesNets, kinds.KindLogicPrograms, StrategyARPriority)

	datum := kinds.Datum{Features: map[string]float64{"x": 0.5}, Label: "B"}
	result, err := pipe.InferComposed(datum)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// AR priority: should pick B
	if result.Final != "B" {
		t.Errorf("expected B (AR priority), got %s", result.Final)
	}
}

func TestBatchMetrics(t *testing.T) {
	dataset := kinds.DataSet{
		Name:  "test",
		Split: "test",
		Items: []kinds.Datum{
			{Label: "A"}, {Label: "B"}, {Label: "A"},
		},
	}
	results := []kinds.ComposedResult{
		{Final: "A", Verified: true, Explained: true, AUROC: 0.9},
		{Final: "B", Verified: true, Explained: true, AUROC: 0.8},
		{Final: "B", Verified: false, Explained: false, AUROC: 0.5}, // wrong
	}

	m := ComputeBatchMetrics(results, dataset)
	if m.TotalItems != 3 {
		t.Errorf("expected 3 items, got %d", m.TotalItems)
	}
	if m.CorrectCount != 2 {
		t.Errorf("expected 2 correct, got %d", m.CorrectCount)
	}
	if m.VerifiedCount != 2 {
		t.Errorf("expected 2 verified, got %d", m.VerifiedCount)
	}
}

func TestEvaluatePhase1Metrics(t *testing.T) {
	bm := BatchMetrics{
		TotalItems:     10,
		CorrectCount:   8,
		VerifiedCount:  10,
		ExplainedCount: 9,
		Accuracy:       0.8,
		MeanAUROC:      0.75,
		VerifyRate:     1.0,
		ExplainRate:    0.9,
	}

	metrics := EvaluatePhase1Metrics(bm, 0.70)
	if len(metrics) < 5 {
		t.Fatalf("expected ≥5 metrics, got %d", len(metrics))
	}

	for _, m := range metrics {
		t.Logf("[%v] %s: %.4f (target %.4f) — %s", m.Pass, m.Name, m.Value, m.Target, m.Detail)
	}
}

func TestComputeAUROC(t *testing.T) {
	dataset := kinds.DataSet{
		Items: []kinds.Datum{{Label: "A"}, {Label: "B"}, {Label: "A"}},
	}
	predictions := []kinds.ComposedResult{
		{Final: "A", AUROC: 0.9},
		{Final: "A", AUROC: 0.7}, // wrong
		{Final: "A", AUROC: 0.8},
	}

	auc := ComputeAUROC(predictions, dataset)
	if auc < 0 || auc > 1 {
		t.Errorf("AUROC out of range: %f", auc)
	}
	t.Logf("AUROC: %f", auc)
}
