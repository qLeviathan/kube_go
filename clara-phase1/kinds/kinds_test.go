package kinds

import "testing"

func TestRegistry(t *testing.T) {
	all := Registry()
	if len(all) != 6 {
		t.Fatalf("expected 6 kinds, got %d", len(all))
	}
}

func TestMLKinds(t *testing.T) {
	ml := MLKinds()
	if len(ml) < 3 {
		t.Fatalf("expected ≥3 ML kinds, got %d", len(ml))
	}
	for _, k := range ml {
		if k.Category != CategoryML {
			t.Errorf("non-ML kind %q in MLKinds()", k.ID)
		}
	}
}

func TestARKinds(t *testing.T) {
	ar := ARKinds()
	if len(ar) < 3 {
		t.Fatalf("expected ≥3 AR kinds, got %d", len(ar))
	}
	for _, k := range ar {
		if k.Category != CategoryAR {
			t.Errorf("non-AR kind %q in ARKinds()", k.ID)
		}
	}
}

func TestComposedResultSummary(t *testing.T) {
	cr := ComposedResult{
		MLResult:  ModelResult{Prediction: "A", Confidence: 0.8, Kind: KindBayesNets},
		ARResult:  ModelResult{Prediction: "A", Confidence: 0.9, Kind: KindLogicPrograms},
		Final:     "A",
		Verified:  true,
		AUROC:     0.85,
		Explained: true,
	}
	s := cr.Summary()
	if s == "" {
		t.Error("expected non-empty summary")
	}
}
