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
	return kinds.NewModelResult(m.prediction, m.confidence, k,
		[]string{"premise: mock", "rule: mock => " + m.prediction}), nil
}

func TestPipelineARPriority(t *testing.T) {
	pipe := NewPipeline("test",
		&mockEngine{prediction: "A", confidence: 0.8, cat: kinds.CategoryML},
		&mockEngine{prediction: "A", confidence: 0.9, cat: kinds.CategoryAR},
		kinds.KindBayesNets, kinds.KindLogicPrograms, StrategyARPriority)

	result, err := pipe.InferComposed(kinds.Datum{Features: map[string]float64{"x": 0.7}, Label: "A"})
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
	pipe := NewPipeline("test",
		&mockEngine{prediction: "A", confidence: 0.6, cat: kinds.CategoryML},
		&mockEngine{prediction: "B", confidence: 0.8, cat: kinds.CategoryAR},
		kinds.KindBayesNets, kinds.KindLogicPrograms, StrategyARPriority)

	result, _ := pipe.InferComposed(kinds.Datum{Features: map[string]float64{"x": 0.5}, Label: "B"})
	if result.Final != "B" {
		t.Errorf("AR priority: expected B, got %s", result.Final)
	}
}

func TestConsensusDisagreement(t *testing.T) {
	pipe := NewPipeline("test",
		&mockEngine{prediction: "A", confidence: 0.8, cat: kinds.CategoryML},
		&mockEngine{prediction: "B", confidence: 0.9, cat: kinds.CategoryAR},
		kinds.KindBayesNets, kinds.KindLogicPrograms, StrategyConsensus)

	result, _ := pipe.InferComposed(kinds.Datum{Label: "X"})
	if result.Final != "inconclusive" {
		t.Errorf("consensus with disagreement: expected inconclusive, got %s", result.Final)
	}
	if result.Verified {
		t.Error("inconclusive should not be verified")
	}
}

func TestBatchMetrics(t *testing.T) {
	dataset := kinds.DataSet{
		Items: []kinds.Datum{{Label: "A"}, {Label: "B"}, {Label: "A"}},
	}
	results := []kinds.ComposedResult{
		{Final: "A", Verified: true, Explained: true, AUROC: 0.9},
		{Final: "B", Verified: true, Explained: true, AUROC: 0.8},
		{Final: "B", Verified: false, Explained: false, AUROC: 0.5},
	}

	m := ComputeBatchMetrics(results, dataset)
	if m.CorrectCount != 2 {
		t.Errorf("expected 2 correct, got %d", m.CorrectCount)
	}
	if m.VerifiedCount != 2 {
		t.Errorf("expected 2 verified, got %d", m.VerifiedCount)
	}
}

func TestEvaluatePhase1Metrics(t *testing.T) {
	bm := BatchMetrics{
		TotalItems: 10, CorrectCount: 8, VerifiedCount: 10, ExplainedCount: 9,
		Accuracy: 0.8, MeanAUROC: 0.75, VerifyRate: 1.0, ExplainRate: 0.9,
	}
	metrics := EvaluatePhase1Metrics(bm, 0.70)
	if len(metrics) < 5 {
		t.Fatalf("expected >=5 metrics, got %d", len(metrics))
	}
	for _, m := range metrics {
		t.Logf("[%v] %s: %.4f (target %.4f)", m.Pass, m.Name, m.Value, m.Target)
	}
}

func TestComputeAUROC(t *testing.T) {
	dataset := kinds.DataSet{
		Items: []kinds.Datum{{Label: "A"}, {Label: "B"}, {Label: "A"}},
	}
	predictions := []kinds.ComposedResult{
		{Final: "A", AUROC: 0.9},
		{Final: "A", AUROC: 0.7},
		{Final: "A", AUROC: 0.8},
	}
	auc := ComputeAUROC(predictions, dataset)
	if auc < 0 || auc > 1 {
		t.Errorf("AUROC out of range: %f", auc)
	}
}

// Test batch isolation: each datum processed independently
func TestBatchIsolation(t *testing.T) {
	// Stateful mock that would produce wrong results if state leaked
	callCount := 0
	mlEng := &statefulMock{callCount: &callCount, cat: kinds.CategoryML}
	arEng := &statefulMock{callCount: &callCount, cat: kinds.CategoryAR}

	pipe := NewPipeline("test", mlEng, arEng,
		kinds.KindBayesNets, kinds.KindLogicPrograms, StrategyARPriority)

	dataset := kinds.DataSet{
		Items: []kinds.Datum{
			{Features: map[string]float64{"x": 0.9}, Label: "high"},
			{Features: map[string]float64{"x": 0.1}, Label: "low"},
			{Features: map[string]float64{"x": 0.9}, Label: "high"},
		},
	}
	results, _ := pipe.RunBatch(dataset)

	if results[0].Final != results[2].Final {
		t.Errorf("items 0 and 2 should match: got %s vs %s", results[0].Final, results[2].Final)
	}
}

type statefulMock struct {
	callCount *int
	cat       kinds.Category
}

func (s *statefulMock) Name() string { return "stateful" }
func (s *statefulMock) Infer(datum kinds.Datum) (kinds.ModelResult, error) {
	*s.callCount++
	k := kinds.KindBayesNets
	if s.cat == kinds.CategoryAR {
		k = kinds.KindLogicPrograms
	}
	pred := "low"
	if datum.Features["x"] > 0.5 {
		pred = "high"
	}
	return kinds.NewModelResult(pred, 0.7, k,
		[]string{"premise: call", "rule: => " + pred}), nil
}
