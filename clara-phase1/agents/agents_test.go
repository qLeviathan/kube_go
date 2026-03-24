package agents

import (
	"fmt"
	"testing"
	"time"

	"github.com/clara-phase1/kinds"
)

// --- SuperClaude Boss Tests ---

func TestSuperClaudeDirective(t *testing.T) {
	boss := NewSuperClaudeAgent("boss")
	msg := Message{From: "test", To: boss.ID(), Type: "directive",
		Payload: "run evaluation", Timestamp: time.Now()}

	resp, err := boss.Process(msg)
	if err != nil {
		t.Fatalf("directive error: %v", err)
	}
	if resp.Type != "result" {
		t.Errorf("expected result type, got %s", resp.Type)
	}
	if len(boss.Directives) != 1 {
		t.Errorf("expected 1 directive, got %d", len(boss.Directives))
	}
}

func TestSuperClaudeFullRun(t *testing.T) {
	boss := NewSuperClaudeAgent("boss")
	boss.AddVerifier(NewVerifierAgent("v1"))
	boss.AddPhD(NewPhDAgent("phd1", "logic-programs"))
	boss.AddModel(NewModelAgent("ar1", kinds.KindLogicPrograms, &dummyEngine{}))

	dataset := kinds.DataSet{
		Name: "test", Split: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"x": 0.8}, Label: "A"},
			{Features: map[string]float64{"x": 0.2}, Label: "B"},
		},
	}

	msg := Message{From: "test", To: boss.ID(), Type: "request",
		Payload: dataset, Timestamp: time.Now()}
	resp, err := boss.Process(msg)
	if err != nil {
		t.Fatalf("boss run error: %v", err)
	}

	results, ok := resp.Payload.([]OrchestratorResult)
	if !ok {
		t.Fatal("expected []OrchestratorResult payload")
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if r.InferenceResult.Final == "" {
			t.Errorf("result %d: empty final prediction", i)
		}
	}
	if len(boss.Log) == 0 {
		t.Error("expected boss to have log entries")
	}
}

func TestSuperClaudeRole(t *testing.T) {
	boss := NewSuperClaudeAgent("boss")
	if boss.Role() != RoleSuperClaude {
		t.Errorf("expected SuperClaude role, got %s", boss.Role())
	}
}

// --- Verifier Tests ---

func TestVerifierAgent(t *testing.T) {
	v := NewVerifierAgent("v1")
	ir := kinds.InferenceResult{
		Result: kinds.ModelResult{
			Prediction: "A", Confidence: 0.9,
			ProofTrace: []string{"fact: y", "rule[r1]: y => A"},
		},
		Final: "A", Verified: false, Confidence: 0.9, Explained: true,
	}

	msg := Message{From: "test", To: v.ID(), Type: "verify",
		Payload: ir, Timestamp: time.Now()}
	resp, err := v.Process(msg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	vr, ok := resp.Payload.(VerificationResult)
	if !ok {
		t.Fatal("expected VerificationResult")
	}
	if !vr.Pass {
		t.Errorf("expected pass, issues: %v", vr.Issues)
	}
}

func TestVerifierRejectsEmptyTrace(t *testing.T) {
	v := NewVerifierAgent("v1")
	ir := kinds.InferenceResult{
		Result: kinds.ModelResult{Prediction: "A", Confidence: 0.8, ProofTrace: []string{}},
		Final:  "A",
	}
	msg := Message{From: "test", To: v.ID(), Type: "verify",
		Payload: ir, Timestamp: time.Now()}
	resp, _ := v.Process(msg)
	vr := resp.Payload.(VerificationResult)
	if vr.Pass {
		t.Error("should fail: empty proof trace")
	}
}

// --- PhD Tests ---

func TestPhDAgent(t *testing.T) {
	p := NewPhDAgent("phd1", "logic-programs")

	msg := Message{From: "test", To: p.ID(), Type: "request",
		Payload:   DomainRequest{Domain: "medical", Constraints: []string{"verifiable"}},
		Timestamp: time.Now()}
	resp, err := p.Process(msg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	advice := resp.Payload.(DomainAdvice)
	if len(advice.RecommendedKinds) == 0 {
		t.Error("expected recommendations")
	}
	if advice.TractabilityNote == "" {
		t.Error("expected tractability note")
	}
}

// --- Model Tests ---

func TestModelAgent(t *testing.T) {
	m := NewModelAgent("m1", kinds.KindLogicPrograms, &dummyEngine{})
	datum := kinds.Datum{Features: map[string]float64{"x": 0.7}, Label: "A"}

	msg := Message{From: "test", To: m.ID(), Type: "request",
		Payload: datum, Timestamp: time.Now()}
	resp, err := m.Process(msg)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	result := resp.Payload.(kinds.ModelResult)
	if result.Prediction == "" {
		t.Error("expected prediction")
	}
}

// --- Datum Isolation via Boss ---

func TestBossDatumIsolation(t *testing.T) {
	boss := NewSuperClaudeAgent("boss")
	boss.AddVerifier(NewVerifierAgent("v1"))
	boss.AddPhD(NewPhDAgent("phd1", "test"))
	boss.AddModel(NewModelAgent("ar1", kinds.KindLogicPrograms, &isolationEngine{}))

	dataset := kinds.DataSet{
		Name: "isolation-test", Split: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"x": 0.9}, Label: "high"},
			{Features: map[string]float64{"x": 0.1}, Label: "low"},
			{Features: map[string]float64{"x": 0.9}, Label: "high"}, // same as first
		},
	}

	msg := Message{From: "test", To: boss.ID(), Type: "request",
		Payload: dataset, Timestamp: time.Now()}
	resp, _ := boss.Process(msg)
	results := resp.Payload.([]OrchestratorResult)

	// Items 0 and 2 have same input, should produce same output
	if results[0].InferenceResult.Final != results[2].InferenceResult.Final {
		t.Errorf("isolation failure: item0=%s item2=%s (should match)",
			results[0].InferenceResult.Final, results[2].InferenceResult.Final)
	}
	// Item 1 has different input, should differ
	if results[0].InferenceResult.Final == results[1].InferenceResult.Final {
		t.Error("different inputs produced same output — possible state leak")
	}
}

// --- Test helpers ---

type dummyEngine struct{}

func (d *dummyEngine) Name() string { return "dummy" }
func (d *dummyEngine) Infer(datum kinds.Datum) (kinds.ModelResult, error) {
	return kinds.NewModelResult(datum.Label, 0.75, kinds.KindLogicPrograms,
		[]string{"premise: input", "rule: test => output"}), nil
}

type isolationEngine struct{}

func (e *isolationEngine) Name() string { return "isolation-test" }
func (e *isolationEngine) Infer(datum kinds.Datum) (kinds.ModelResult, error) {
	pred := "low"
	conf := 0.3
	if datum.Features["x"] > 0.5 {
		pred = "high"
		conf = 0.8
	}
	return kinds.NewModelResult(pred, conf, kinds.KindLogicPrograms,
		[]string{"premise: x=" + fmt.Sprintf("%.1f", datum.Features["x"]),
			"rule: threshold => " + pred}), nil
}
