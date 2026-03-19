package agents

import (
	"testing"
	"time"

	"github.com/clara-phase1/kinds"
)

func TestVerifierAgent(t *testing.T) {
	v := NewVerifierAgent("test-verifier")

	if v.ID() != "test-verifier" {
		t.Errorf("expected ID 'test-verifier', got %q", v.ID())
	}
	if v.Role() != RoleVerifier {
		t.Errorf("expected role Verifier, got %q", v.Role())
	}

	cr := kinds.ComposedResult{
		MLResult: kinds.ModelResult{
			Prediction: "A",
			Confidence: 0.8,
			ProofTrace: []string{"premise: feature_x", "rule: x => A"},
		},
		ARResult: kinds.ModelResult{
			Prediction: "A",
			Confidence: 0.9,
			ProofTrace: []string{"fact: has_feature(x, high)", "rule[r1]: x ∧ y => A"},
		},
		Final:     "A",
		Verified:  false,
		AUROC:     0.85,
		Explained: true,
	}

	msg := Message{From: "test", To: v.ID(), Type: "verify", Payload: cr, Timestamp: time.Now()}
	resp, err := v.Process(msg)
	if err != nil {
		t.Fatalf("verification error: %v", err)
	}

	vr, ok := resp.Payload.(VerificationResult)
	if !ok {
		t.Fatal("expected VerificationResult payload")
	}
	if !vr.Pass {
		t.Errorf("expected verification to pass, got issues: %v", vr.Issues)
	}
	if !vr.Sound {
		t.Error("expected sound")
	}
	if !vr.Complete {
		t.Error("expected complete")
	}
}

func TestPhDAgent(t *testing.T) {
	p := NewPhDAgent("test-phd", "bayesian-lp")

	msg := Message{
		From: "test", To: p.ID(), Type: "request",
		Payload:   DomainRequest{Domain: "medical", Constraints: []string{"verifiable"}},
		Timestamp: time.Now(),
	}

	resp, err := p.Process(msg)
	if err != nil {
		t.Fatalf("PhD process error: %v", err)
	}

	advice, ok := resp.Payload.(DomainAdvice)
	if !ok {
		t.Fatal("expected DomainAdvice payload")
	}
	if len(advice.RecommendedKinds) == 0 {
		t.Error("expected recommended kinds")
	}
	if advice.TractabilityNote == "" {
		t.Error("expected tractability note")
	}
}

func TestModelAgent(t *testing.T) {
	// Create a dummy engine
	engine := &dummyEngine{}
	m := NewModelAgent("test-model", kinds.KindBayesNets, engine)

	datum := kinds.Datum{
		Features: map[string]float64{"x": 0.7},
		Label:    "positive",
	}

	msg := Message{From: "test", To: m.ID(), Type: "request", Payload: datum, Timestamp: time.Now()}
	resp, err := m.Process(msg)
	if err != nil {
		t.Fatalf("model process error: %v", err)
	}

	result, ok := resp.Payload.(kinds.ModelResult)
	if !ok {
		t.Fatal("expected ModelResult payload")
	}
	if result.Prediction == "" {
		t.Error("expected non-empty prediction")
	}
}

func TestOrchestrator(t *testing.T) {
	orch := NewOrchestrator()
	orch.AddVerifier(NewVerifierAgent("v1"))
	orch.AddPhD(NewPhDAgent("phd1", "test"))
	orch.AddModel(NewModelAgent("ml1", kinds.KindBayesNets, &dummyEngine{cat: kinds.CategoryML}))
	orch.AddModel(NewModelAgent("ar1", kinds.KindLogicPrograms, &dummyEngine{cat: kinds.CategoryAR}))

	dataset := kinds.DataSet{
		Name:  "test",
		Split: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"x": 0.8}, Label: "A"},
			{Features: map[string]float64{"x": 0.2}, Label: "B"},
		},
	}

	results := orch.RunBatch(dataset)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.ComposedResult.Final == "" {
			t.Errorf("result %d: empty final prediction", i)
		}
	}
}

// dummyEngine implements InferenceEngine for testing.
type dummyEngine struct {
	cat kinds.Category
}

func (d *dummyEngine) Name() string { return "dummy" }

func (d *dummyEngine) Infer(datum kinds.Datum) (kinds.ModelResult, error) {
	k := kinds.KindBayesNets
	if d.cat == kinds.CategoryAR {
		k = kinds.KindLogicPrograms
	}
	return kinds.ModelResult{
		Prediction: datum.Label,
		Confidence: 0.75,
		Kind:       k,
		ProofTrace: []string{"premise: input", "rule: test => output"},
	}, nil
}
