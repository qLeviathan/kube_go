package explain

import (
	"testing"

	"github.com/clara-phase1/kinds"
)

func TestBuildProof(t *testing.T) {
	pb := NewProofBuilder()

	cr := kinds.ComposedResult{
		MLResult: kinds.ModelResult{
			Prediction: "treat_A",
			Confidence: 0.85,
			Kind:       kinds.KindBayesNets,
			ProofTrace: []string{
				"evidence: 3 features observed",
				"prior(blood_pressure): beliefs updated",
				"propagate(decision): P(high)=0.7 P(low)=0.3",
				"conclusion: treat_A=P(0.85) via decision",
			},
		},
		ARResult: kinds.ModelResult{
			Prediction: "treat_A",
			Confidence: 0.9,
			Kind:       kinds.KindLogicPrograms,
			ProofTrace: []string{
				"premise: has_feature(blood_pressure, high)",
				"premise: has_feature(heart_rate, high)",
				"rule[med-r1]: blood_pressure_high ∧ heart_rate_high => treat_A_indicated",
			},
		},
		Final:     "treat_A",
		Verified:  true,
		AUROC:     0.875,
		Explained: true,
	}

	proof := pb.BuildProof(cr)

	if !proof.Sound {
		t.Error("expected sound proof")
	}
	if !proof.Complete {
		t.Error("expected complete proof")
	}
	if proof.MaxDepth != 2 {
		t.Errorf("expected max depth 2, got %d", proof.MaxDepth)
	}
	if proof.Unfolding > 10 {
		t.Errorf("unfolding %d exceeds CLARA limit of 10", proof.Unfolding)
	}
	if len(proof.Steps) == 0 {
		t.Fatal("expected proof steps")
	}

	formatted := FormatProof(proof)
	if formatted == "" {
		t.Error("expected non-empty formatted proof")
	}
	t.Log(formatted)
}

func TestProofUnfoldingLimit(t *testing.T) {
	pb := NewProofBuilder()

	// Create a result with many proof trace steps (should be capped at 10)
	longTrace := make([]string, 15)
	for i := 0; i < 15; i++ {
		longTrace[i] = "premise: step"
	}

	cr := kinds.ComposedResult{
		MLResult: kinds.ModelResult{
			Prediction: "X",
			Confidence: 0.7,
			Kind:       kinds.KindBayesNets,
			ProofTrace: longTrace,
		},
		ARResult: kinds.ModelResult{
			Prediction: "X",
			Confidence: 0.8,
			Kind:       kinds.KindLogicPrograms,
			ProofTrace: longTrace,
		},
		Final: "X",
		AUROC: 0.75,
	}

	proof := pb.BuildProof(cr)

	// Count level-2 steps: should be ≤10 per component (ML and AR each capped)
	level2Count := 0
	for _, s := range proof.Steps {
		if s.Level == 2 {
			level2Count++
		}
	}

	// Each component is capped at 10, so total level-2 ≤ 20
	if level2Count > 20 {
		t.Errorf("expected ≤20 level-2 steps (10 per component), got %d", level2Count)
	}

	t.Logf("Proof steps: %d, Level-2: %d, Unfolding: %d", len(proof.Steps), level2Count, proof.Unfolding)
}
