// Package agents implements the CLARA Phase 1 agent framework.
// Agent types: SuperClaude (boss), Verifier, PhD (domain expert), Model (inference).
// No recursion — all agents operate via iterative message-passing.
// All inference state is isolated per-datum — no state bleeds between calls.
package agents

import (
	"fmt"
	"sync"
	"time"

	"github.com/clara-phase1/kinds"
)

// Role identifies the agent type.
type Role string

const (
	RoleSuperClaude Role = "SuperClaude"
	RoleVerifier    Role = "Verifier"
	RolePhD         Role = "PhD"
	RoleModel       Role = "Model"
)

// Message is the unit of communication between agents.
type Message struct {
	From      string
	To        string
	Type      string // "request", "result", "verify", "review", "directive", "report"
	Payload   interface{}
	Timestamp time.Time
}

// Agent is the interface all CLARA agents implement.
type Agent interface {
	ID() string
	Role() Role
	Process(msg Message) (Message, error)
}

// InferenceEngine is the interface that AR and ML engines implement.
type InferenceEngine interface {
	Infer(datum kinds.Datum) (kinds.ModelResult, error)
	Name() string
}

// --- SuperClaudeAgent: the boss agent that orchestrates everything ---

// SuperClaudeAgent is the top-level orchestrator. It dispatches work to
// PhD agents for domain analysis, Model agents for inference, and Verifier
// agents for proof checking. It makes final decisions and produces reports.
type SuperClaudeAgent struct {
	AgentID    string
	Verifiers  []*VerifierAgent
	PhDs       []*PhDAgent
	Models     []*ModelAgent
	Log        []string
	Directives []string
	Results    []OrchestratorResult
	mu         sync.Mutex
}

func NewSuperClaudeAgent(id string) *SuperClaudeAgent {
	return &SuperClaudeAgent{
		AgentID:    id,
		Directives: []string{},
	}
}

func (s *SuperClaudeAgent) ID() string  { return s.AgentID }
func (s *SuperClaudeAgent) Role() Role  { return RoleSuperClaude }

func (s *SuperClaudeAgent) AddVerifier(v *VerifierAgent) { s.Verifiers = append(s.Verifiers, v) }
func (s *SuperClaudeAgent) AddPhD(p *PhDAgent)           { s.PhDs = append(s.PhDs, p) }
func (s *SuperClaudeAgent) AddModel(m *ModelAgent)        { s.Models = append(s.Models, m) }

func (s *SuperClaudeAgent) Process(msg Message) (Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch msg.Type {
	case "directive":
		directive, ok := msg.Payload.(string)
		if !ok {
			return Message{}, fmt.Errorf("superclaude: expected string directive")
		}
		s.Directives = append(s.Directives, directive)
		s.log("received directive: %s", directive)
		return Message{From: s.AgentID, To: msg.From, Type: "result",
			Payload: fmt.Sprintf("ACK: directive received — %s", directive), Timestamp: time.Now()}, nil

	case "request":
		dataset, ok := msg.Payload.(kinds.DataSet)
		if !ok {
			return Message{}, fmt.Errorf("superclaude: expected DataSet payload for request")
		}
		results := s.runFullEvaluation(dataset)
		return Message{From: s.AgentID, To: msg.From, Type: "report",
			Payload: results, Timestamp: time.Now()}, nil

	default:
		return Message{}, fmt.Errorf("superclaude: unknown message type %q", msg.Type)
	}
}

// runFullEvaluation is the boss's main loop: iterates datums, dispatches to agents.
func (s *SuperClaudeAgent) runFullEvaluation(dataset kinds.DataSet) []OrchestratorResult {
	s.log("=== SuperClaude starting evaluation: %s (%d items) ===", dataset.Name, len(dataset.Items))

	// Step 1: Ask PhD agents for domain analysis
	s.log("Phase: Domain Analysis")
	for i := 0; i < len(s.PhDs); i++ {
		p := s.PhDs[i]
		msg := Message{
			From: s.AgentID, To: p.ID(), Type: "request",
			Payload:   DomainRequest{Domain: dataset.Name, Constraints: []string{"verifiable", "polynomial"}},
			Timestamp: time.Now(),
		}
		resp, err := p.Process(msg)
		if err != nil {
			s.log("PhD %s error: %v", p.ID(), err)
			continue
		}
		if advice, ok := resp.Payload.(DomainAdvice); ok {
			s.log("PhD %s recommends %d kinds: %s", p.ID(), len(advice.RecommendedKinds), advice.TractabilityNote)
		}
	}

	// Step 2: Process each datum — fully isolated per iteration
	results := make([]OrchestratorResult, 0, len(dataset.Items))
	for idx := 0; idx < len(dataset.Items); idx++ {
		datum := dataset.Items[idx]
		or := s.processSingleDatum(datum, idx)
		results = append(results, or)
	}

	s.log("=== SuperClaude completed: %d results ===", len(results))

	s.Results = append(s.Results, results...)
	return results
}

// processSingleDatum handles one datum through all agents.
func (s *SuperClaudeAgent) processSingleDatum(datum kinds.Datum, idx int) OrchestratorResult {
	result := OrchestratorResult{Datum: datum}

	// Step A: Run all model agents (each Infer call has isolated state)
	var mlResult, arResult kinds.ModelResult
	mlFound, arFound := false, false

	for i := 0; i < len(s.Models); i++ {
		m := s.Models[i]
		msg := Message{From: s.AgentID, To: m.ID(), Type: "request",
			Payload: datum, Timestamp: time.Now()}
		resp, err := m.Process(msg)
		if err != nil {
			s.log("Model %s error on item %d: %v", m.ID(), idx, err)
			continue
		}
		mr, ok := resp.Payload.(kinds.ModelResult)
		if !ok {
			continue
		}
		if mr.Kind.Category == kinds.CategoryML {
			mlResult = mr
			mlFound = true
		} else if mr.Kind.Category == kinds.CategoryAR {
			arResult = mr
			arFound = true
		}
	}

	if !mlFound {
		mlResult = kinds.NewModelResult("no_ml", 0, kinds.KindBayesNets, []string{"no ML model ran"})
	}
	if !arFound {
		arResult = kinds.NewModelResult("no_ar", 0, kinds.KindLogicPrograms, []string{"no AR model ran"})
	}

	// Step B: Compose ML + AR results
	composed := kinds.ComposedResult{
		MLResult:  mlResult,
		ARResult:  arResult,
		Explained: len(mlResult.ProofTrace) > 0 && len(arResult.ProofTrace) > 0,
	}

	// Resolution: AR priority (higher assurance)
	if mlResult.Prediction == arResult.Prediction {
		composed.Final = mlResult.Prediction
		composed.AUROC = (mlResult.Confidence + arResult.Confidence) / 2.0
	} else if arResult.Confidence >= mlResult.Confidence {
		composed.Final = arResult.Prediction
		composed.AUROC = arResult.Confidence*0.6 + mlResult.Confidence*0.4
	} else {
		composed.Final = mlResult.Prediction
		composed.AUROC = mlResult.Confidence*0.6 + arResult.Confidence*0.4
	}

	// Step C: Verify with all verifier agents
	for i := 0; i < len(s.Verifiers); i++ {
		v := s.Verifiers[i]
		msg := Message{From: s.AgentID, To: v.ID(), Type: "verify",
			Payload: composed, Timestamp: time.Now()}
		resp, err := v.Process(msg)
		if err != nil {
			continue
		}
		if vr, ok := resp.Payload.(VerificationResult); ok {
			result.Verification = vr
			composed.Verified = vr.Pass
		}
	}
	result.ComposedResult = composed

	// Step D: PhD review
	for i := 0; i < len(s.PhDs); i++ {
		p := s.PhDs[i]
		msg := Message{From: s.AgentID, To: p.ID(), Type: "review",
			Payload: composed, Timestamp: time.Now()}
		resp, err := p.Process(msg)
		if err != nil {
			continue
		}
		if dr, ok := resp.Payload.(DomainReview); ok {
			result.DomainReview = dr
		}
	}

	return result
}

func (s *SuperClaudeAgent) log(format string, args ...interface{}) {
	s.Log = append(s.Log, fmt.Sprintf(format, args...))
}

// --- OrchestratorResult ---

type OrchestratorResult struct {
	Datum          kinds.Datum
	ComposedResult kinds.ComposedResult
	Verification   VerificationResult
	DomainReview   DomainReview
}

// --- VerifierAgent ---

type VerifierAgent struct {
	AgentID string
	Log     []string
	mu      sync.Mutex
}

func NewVerifierAgent(id string) *VerifierAgent {
	return &VerifierAgent{AgentID: id}
}

func (v *VerifierAgent) ID() string  { return v.AgentID }
func (v *VerifierAgent) Role() Role  { return RoleVerifier }

func (v *VerifierAgent) Process(msg Message) (Message, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if msg.Type != "verify" {
		return Message{}, fmt.Errorf("verifier: unknown message type %q", msg.Type)
	}

	cr, ok := msg.Payload.(kinds.ComposedResult)
	if !ok {
		return Message{}, fmt.Errorf("verifier: expected ComposedResult payload")
	}

	vr := v.verify(cr)
	v.Log = append(v.Log, fmt.Sprintf("verified: %s pass=%v issues=%d", cr.Final, vr.Pass, len(vr.Issues)))

	return Message{From: v.AgentID, To: msg.From, Type: "result",
		Payload: vr, Timestamp: time.Now()}, nil
}

func (v *VerifierAgent) verify(cr kinds.ComposedResult) VerificationResult {
	vr := VerificationResult{Sound: true, Complete: true}

	// Check proof traces exist (non-nil, non-empty)
	if len(cr.MLResult.ProofTrace) == 0 {
		vr.Sound = false
		vr.Issues = append(vr.Issues, "ML result missing proof trace")
	}
	if len(cr.ARResult.ProofTrace) == 0 {
		vr.Sound = false
		vr.Issues = append(vr.Issues, "AR result missing proof trace")
	}

	// Check unfolding ≤ 10
	if len(cr.MLResult.ProofTrace) > 10 {
		vr.Issues = append(vr.Issues, fmt.Sprintf("ML proof depth %d exceeds 10", len(cr.MLResult.ProofTrace)))
		vr.Sound = false
	}
	if len(cr.ARResult.ProofTrace) > 10 {
		vr.Issues = append(vr.Issues, fmt.Sprintf("AR proof depth %d exceeds 10", len(cr.ARResult.ProofTrace)))
		vr.Sound = false
	}

	// Check confidence in [0,1]
	if cr.MLResult.Confidence < 0 || cr.MLResult.Confidence > 1 {
		vr.Sound = false
		vr.Issues = append(vr.Issues, "ML confidence out of [0,1]")
	}
	if cr.ARResult.Confidence < 0 || cr.ARResult.Confidence > 1 {
		vr.Sound = false
		vr.Issues = append(vr.Issues, "AR confidence out of [0,1]")
	}

	// Check ML/AR disagreement is resolved
	if cr.MLResult.Prediction != cr.ARResult.Prediction && cr.Final == "" {
		vr.Complete = false
		vr.Issues = append(vr.Issues, "ML/AR disagreement with no resolution")
	}

	// Check no empty proof steps
	for i := 0; i < len(cr.MLResult.ProofTrace); i++ {
		if cr.MLResult.ProofTrace[i] == "" {
			vr.Sound = false
			vr.Issues = append(vr.Issues, fmt.Sprintf("ML proof step %d is empty", i))
		}
	}
	for i := 0; i < len(cr.ARResult.ProofTrace); i++ {
		if cr.ARResult.ProofTrace[i] == "" {
			vr.Sound = false
			vr.Issues = append(vr.Issues, fmt.Sprintf("AR proof step %d is empty", i))
		}
	}

	vr.Pass = vr.Sound && vr.Complete && len(vr.Issues) == 0
	vr.Explained = vr.Pass
	return vr
}

type VerificationResult struct {
	Sound     bool
	Complete  bool
	Pass      bool
	Explained bool
	Issues    []string
}

// --- PhDAgent ---

type PhDAgent struct {
	AgentID   string
	Specialty string
	Log       []string
	mu        sync.Mutex
}

func NewPhDAgent(id, specialty string) *PhDAgent {
	return &PhDAgent{AgentID: id, Specialty: specialty}
}

func (p *PhDAgent) ID() string  { return p.AgentID }
func (p *PhDAgent) Role() Role  { return RolePhD }

func (p *PhDAgent) Process(msg Message) (Message, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch msg.Type {
	case "request":
		req, ok := msg.Payload.(DomainRequest)
		if !ok {
			return Message{}, fmt.Errorf("phd: expected DomainRequest payload")
		}
		advice := p.analyze(req)
		p.Log = append(p.Log, fmt.Sprintf("analyzed domain=%s kinds=%d", req.Domain, len(advice.RecommendedKinds)))
		return Message{From: p.AgentID, To: msg.From, Type: "result",
			Payload: advice, Timestamp: time.Now()}, nil

	case "review":
		cr, ok := msg.Payload.(kinds.ComposedResult)
		if !ok {
			return Message{}, fmt.Errorf("phd: expected ComposedResult for review")
		}
		review := p.review(cr)
		p.Log = append(p.Log, fmt.Sprintf("reviewed: %s", review.Summary))
		return Message{From: p.AgentID, To: msg.From, Type: "result",
			Payload: review, Timestamp: time.Now()}, nil

	default:
		return Message{}, fmt.Errorf("phd: unknown message type %q", msg.Type)
	}
}

func (p *PhDAgent) analyze(req DomainRequest) DomainAdvice {
	advice := DomainAdvice{Domain: req.Domain}

	for _, k := range kinds.ARKinds() {
		if k.ID == "ar-lp" || k.ID == "ar-blp" {
			advice.RecommendedKinds = append(advice.RecommendedKinds, k)
			advice.Rationale = append(advice.Rationale,
				fmt.Sprintf("Selected %s: strong composability, proven verifiability, polynomial inference", k.Name))
		}
	}
	for _, k := range kinds.MLKinds() {
		if k.ID == "ml-bn" || k.ID == "ml-bayes" {
			advice.RecommendedKinds = append(advice.RecommendedKinds, k)
			advice.Rationale = append(advice.Rationale,
				fmt.Sprintf("Selected %s: probabilistic inference, composable with LP", k.Name))
		}
	}
	advice.TractabilityNote = "Bayesian-LP with restraint achieves worst-case polynomial time for inference"
	return advice
}

func (p *PhDAgent) review(cr kinds.ComposedResult) DomainReview {
	review := DomainReview{Approved: cr.Verified && cr.Explained}
	if cr.AUROC >= 0.5 {
		review.Summary = fmt.Sprintf("acceptable: AUROC=%.4f verified=%v", cr.AUROC, cr.Verified)
	} else {
		review.Summary = fmt.Sprintf("below threshold: AUROC=%.4f", cr.AUROC)
		review.Approved = false
	}
	return review
}

type DomainRequest struct {
	Domain      string
	Constraints []string
}

type DomainAdvice struct {
	Domain           string
	RecommendedKinds []kinds.Kind
	Rationale        []string
	TractabilityNote string
}

type DomainReview struct {
	Approved bool
	Summary  string
}

// --- ModelAgent ---

type ModelAgent struct {
	AgentID   string
	ModelKind kinds.Kind
	Engine    InferenceEngine
	Log       []string
	mu        sync.Mutex
}

func NewModelAgent(id string, kind kinds.Kind, engine InferenceEngine) *ModelAgent {
	return &ModelAgent{AgentID: id, ModelKind: kind, Engine: engine}
}

func (m *ModelAgent) ID() string  { return m.AgentID }
func (m *ModelAgent) Role() Role  { return RoleModel }

func (m *ModelAgent) Process(msg Message) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if msg.Type != "request" {
		return Message{}, fmt.Errorf("model: unknown message type %q", msg.Type)
	}

	datum, ok := msg.Payload.(kinds.Datum)
	if !ok {
		return Message{}, fmt.Errorf("model: expected Datum payload")
	}

	result, err := m.Engine.Infer(datum)
	if err != nil {
		return Message{}, fmt.Errorf("model: inference error: %w", err)
	}
	result.Kind = m.ModelKind
	m.Log = append(m.Log, fmt.Sprintf("inferred: %s conf=%.2f", result.Prediction, result.Confidence))

	return Message{From: m.AgentID, To: msg.From, Type: "result",
		Payload: result, Timestamp: time.Now()}, nil
}
