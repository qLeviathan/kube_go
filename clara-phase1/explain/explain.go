// Package explain implements the CLARA Phase 1 explainability system.
// Produces hierarchical, fine-grained, natural-deduction-style proofs
// with unfolding expansion <= 10 between hierarchical levels.
// No recursion.
package explain

import (
	"fmt"
	"strings"

	"github.com/clara-phase1/kinds"
)

// ProofStep is a single step in a natural deduction proof.
type ProofStep struct {
	Level      int
	StepNumber int
	Type       string // "premise", "rule", "conclusion", "evidence", "propagation"
	Content    string
	Justified  bool
}

// Proof is a complete hierarchical proof for a composed result.
type Proof struct {
	ID        string
	Steps     []ProofStep
	Sound     bool
	Complete  bool
	MaxDepth  int
	Unfolding int // max steps at one hierarchical level, must be <= 10
}

// ProofBuilder builds proofs iteratively from composed results.
type ProofBuilder struct {
	proofCounter int
}

func NewProofBuilder() *ProofBuilder {
	return &ProofBuilder{}
}

// BuildProof constructs a hierarchical proof from a ComposedResult.
// Caps each component's trace at 10 steps (DARPA unfolding requirement).
func (pb *ProofBuilder) BuildProof(cr kinds.ComposedResult) Proof {
	pb.proofCounter++
	proof := Proof{ID: fmt.Sprintf("proof-%d", pb.proofCounter)}

	stepNum := 0

	// Level 0: Top-level conclusion
	stepNum++
	proof.Steps = append(proof.Steps, ProofStep{
		Level: 0, StepNumber: stepNum, Type: "conclusion",
		Content:   fmt.Sprintf("FINAL: %s (AUROC=%.4f, Verified=%v)", cr.Final, cr.AUROC, cr.Verified),
		Justified: true,
	})

	// Level 1: ML component
	stepNum++
	proof.Steps = append(proof.Steps, ProofStep{
		Level: 1, StepNumber: stepNum, Type: "evidence",
		Content:   fmt.Sprintf("ML[%s]: prediction=%s confidence=%.4f", cr.MLResult.Kind.Name, cr.MLResult.Prediction, cr.MLResult.Confidence),
		Justified: len(cr.MLResult.ProofTrace) > 0,
	})

	// Level 2: ML proof trace (capped at 10)
	mlCap := len(cr.MLResult.ProofTrace)
	if mlCap > 10 {
		mlCap = 10
	}
	for i := 0; i < mlCap; i++ {
		stepNum++
		proof.Steps = append(proof.Steps, ProofStep{
			Level: 2, StepNumber: stepNum,
			Type:      classifyTraceStep(cr.MLResult.ProofTrace[i]),
			Content:   cr.MLResult.ProofTrace[i],
			Justified: true,
		})
	}

	// Level 1: AR component
	stepNum++
	proof.Steps = append(proof.Steps, ProofStep{
		Level: 1, StepNumber: stepNum, Type: "evidence",
		Content:   fmt.Sprintf("AR[%s]: prediction=%s confidence=%.4f", cr.ARResult.Kind.Name, cr.ARResult.Prediction, cr.ARResult.Confidence),
		Justified: len(cr.ARResult.ProofTrace) > 0,
	})

	// Level 2: AR proof trace (capped at 10)
	arCap := len(cr.ARResult.ProofTrace)
	if arCap > 10 {
		arCap = 10
	}
	for i := 0; i < arCap; i++ {
		stepNum++
		proof.Steps = append(proof.Steps, ProofStep{
			Level: 2, StepNumber: stepNum,
			Type:      classifyTraceStep(cr.ARResult.ProofTrace[i]),
			Content:   cr.ARResult.ProofTrace[i],
			Justified: true,
		})
	}

	// Level 1: Composition reasoning
	stepNum++
	if cr.MLResult.Prediction == cr.ARResult.Prediction {
		proof.Steps = append(proof.Steps, ProofStep{
			Level: 1, StepNumber: stepNum, Type: "rule",
			Content:   fmt.Sprintf("COMPOSE: ML and AR agree on %q => high confidence", cr.Final),
			Justified: true,
		})
	} else {
		proof.Steps = append(proof.Steps, ProofStep{
			Level: 1, StepNumber: stepNum, Type: "rule",
			Content:   fmt.Sprintf("COMPOSE: ML=%q AR=%q disagree => final=%q", cr.MLResult.Prediction, cr.ARResult.Prediction, cr.Final),
			Justified: true,
		})
	}

	proof.MaxDepth = 2
	proof.Unfolding = computeMaxUnfolding(proof.Steps)
	proof.Sound = allJustified(proof.Steps)
	proof.Complete = len(cr.MLResult.ProofTrace) > 0 && len(cr.ARResult.ProofTrace) > 0
	return proof
}

func classifyTraceStep(s string) string {
	switch {
	case strings.HasPrefix(s, "premise:"), strings.HasPrefix(s, "fact:"):
		return "premise"
	case strings.HasPrefix(s, "rule["), strings.HasPrefix(s, "derived:"):
		return "rule"
	case strings.HasPrefix(s, "conclusion:"):
		return "conclusion"
	case strings.HasPrefix(s, "evidence:"), strings.HasPrefix(s, "prior("):
		return "evidence"
	case strings.HasPrefix(s, "propagate("):
		return "propagation"
	default:
		return "premise"
	}
}

func computeMaxUnfolding(steps []ProofStep) int {
	if len(steps) == 0 {
		return 0
	}
	maxCount := 0
	currentLevel := steps[0].Level
	count := 1
	for i := 1; i < len(steps); i++ {
		if steps[i].Level == currentLevel {
			count++
		} else {
			if count > maxCount {
				maxCount = count
			}
			currentLevel = steps[i].Level
			count = 1
		}
	}
	if count > maxCount {
		maxCount = count
	}
	return maxCount
}

func allJustified(steps []ProofStep) bool {
	for i := 0; i < len(steps); i++ {
		if !steps[i].Justified {
			return false
		}
	}
	return true
}

// FormatProof renders a proof as a human-readable string.
func FormatProof(p Proof) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== Proof %s (sound=%v complete=%v depth=%d unfolding=%d) ===\n",
		p.ID, p.Sound, p.Complete, p.MaxDepth, p.Unfolding))
	for i := 0; i < len(p.Steps); i++ {
		step := p.Steps[i]
		indent := strings.Repeat("  ", step.Level)
		marker := "+"
		if !step.Justified {
			marker = "!"
		}
		sb.WriteString(fmt.Sprintf("%s[%s] %d. [%s] %s\n", indent, marker, step.StepNumber, step.Type, step.Content))
	}
	return sb.String()
}
