package kinds

import "testing"

func TestRegistry(t *testing.T) {
	all := Registry()
	if len(all) != 3 {
		t.Fatalf("expected 3 kinds, got %d", len(all))
	}
}

func TestARKinds(t *testing.T) {
	ar := ARKinds()
	if len(ar) != 3 {
		t.Fatalf("expected 3 AR kinds, got %d", len(ar))
	}
}

func TestKindValidate(t *testing.T) {
	good := Kind{ID: "test", Name: "Test"}
	if err := good.Validate(); err != nil {
		t.Errorf("valid kind failed: %v", err)
	}

	empty := Kind{Name: "Test"}
	if err := empty.Validate(); err == nil {
		t.Error("empty ID should fail validation")
	}
}

func TestInferenceResultSummary(t *testing.T) {
	ir := InferenceResult{
		Result: ModelResult{Prediction: "A", Confidence: 0.9, Kind: KindLogicPrograms},
		Final: "A", Verified: true, Confidence: 0.9, Explained: true,
	}
	s := ir.Summary()
	if s == "" {
		t.Error("expected non-empty summary")
	}
}

func TestNewModelResultNilTrace(t *testing.T) {
	mr := NewModelResult("X", 0.5, KindLogicPrograms, nil)
	if mr.ProofTrace == nil {
		t.Error("NewModelResult should initialize nil trace to empty slice")
	}
	if len(mr.ProofTrace) != 0 {
		t.Error("initialized trace should be empty")
	}
}
