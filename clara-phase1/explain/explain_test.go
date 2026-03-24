package explain

import (
	"testing"

	"github.com/clara-phase1/kinds"
)

func TestBuildProof(t *testing.T) {
	pb := NewProofBuilder()
	ir := kinds.InferenceResult{
		Result: kinds.ModelResult{
			Prediction: "treat_A", Confidence: 0.9, Kind: kinds.KindLogicPrograms,
			ProofTrace: []string{
				"premise: has_feature(blood_pressure, high)",
				"premise: has_feature(heart_rate, high)",
				"rule[med-r1]: bp_high ^ hr_high => treat_A_indicated",
			},
		},
		Final: "treat_A", Verified: true, Confidence: 0.9, Explained: true,
	}

	proof := pb.BuildProof(ir)
	if !proof.Sound {
		t.Error("expected sound")
	}
	if !proof.Complete {
		t.Error("expected complete")
	}
	if proof.MaxDepth != 2 {
		t.Errorf("expected depth 2, got %d", proof.MaxDepth)
	}
	if proof.Unfolding > 10 {
		t.Errorf("unfolding %d exceeds 10", proof.Unfolding)
	}

	formatted := FormatProof(proof)
	if formatted == "" {
		t.Error("expected formatted output")
	}
	t.Log(formatted)
}

func TestProofUnfoldingCapped(t *testing.T) {
	pb := NewProofBuilder()
	longTrace := make([]string, 15)
	for i := 0; i < 15; i++ {
		longTrace[i] = "premise: step"
	}

	ir := kinds.InferenceResult{
		Result: kinds.ModelResult{
			Prediction: "X", Confidence: 0.8, Kind: kinds.KindLogicPrograms,
			ProofTrace: longTrace,
		},
		Final: "X", Confidence: 0.8,
	}

	proof := pb.BuildProof(ir)
	level2Count := 0
	for _, s := range proof.Steps {
		if s.Level == 2 {
			level2Count++
		}
	}
	// Capped at 10 level-2 steps
	if level2Count > 10 {
		t.Errorf("expected <=10 level-2 steps, got %d", level2Count)
	}
}
