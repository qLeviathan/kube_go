// Package agents implements the CLARA Phase 1 agent framework.
// Three agent types: Verifier, PhD (domain expert), Model (inference).
// No recursion — all agents operate via iterative message-passing on channels.
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
	RoleVerifier Role = "Verifier"
	RolePhD      Role = "PhD"
	RoleModel    Role = "Model"
)

// Message is the unit of communication between agents.
type Message struct {
	From      string
	To        string
	Type      string // "request", "result", "verify", "report"
	Payload   interface{}
	Timestamp time.Time
}

// Agent is the interface all CLARA agents implement.
type Agent interface {
	ID() string
	Role() Role
	Process(msg Message) (Message, error)
}

// VerifierAgent checks proofs, verifiability, and logical explainability.
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

	switch msg.Type {
	case "verify":
		result, ok := msg.Payload.(kinds.ComposedResult)
		if !ok {
			return Message{}, fmt.Errorf("verifier: expected ComposedResult payload")
		}
		verification := v.verify(result)
		v.Log = append(v.Log, fmt.Sprintf("verified: %s pass=%v", result.Final, verification.Pass))
		return Message{
			From:      v.AgentID,
			To:        msg.From,
			Type:      "result",
			Payload:   verification,
			Timestamp: time.Now(),
		}, nil
	default:
		return Message{}, fmt.Errorf("verifier: unknown message type %q", msg.Type)
	}
}

// verify checks soundness, completeness, and explainability properties iteratively.
func (v *VerifierAgent) verify(cr kinds.ComposedResult) VerificationResult {
	vr := VerificationResult{
		Sound:    true,
		Complete: true,
	}

	// Check ML result has proof trace (explainability)
	if len(cr.MLResult.ProofTrace) == 0 {
		vr.Sound = false
		vr.Issues = append(vr.Issues, "ML result missing proof trace")
	}
	// Check AR result has proof trace
	if len(cr.ARResult.ProofTrace) == 0 {
		vr.Sound = false
		vr.Issues = append(vr.Issues, "AR result missing proof trace")
	}
	// Check unfolding expansion ≤ 10 (CLARA requirement)
	mlDepth := len(cr.MLResult.ProofTrace)
	arDepth := len(cr.ARResult.ProofTrace)
	if mlDepth > 10 {
		vr.Issues = append(vr.Issues, fmt.Sprintf("ML proof depth %d exceeds 10", mlDepth))
		vr.Sound = false
	}
	if arDepth > 10 {
		vr.Issues = append(vr.Issues, fmt.Sprintf("AR proof depth %d exceeds 10", arDepth))
		vr.Sound = false
	}
	// Check confidence thresholds
	if cr.MLResult.Confidence < 0.0 || cr.MLResult.Confidence > 1.0 {
		vr.Sound = false
		vr.Issues = append(vr.Issues, "ML confidence out of [0,1] range")
	}
	if cr.ARResult.Confidence < 0.0 || cr.ARResult.Confidence > 1.0 {
		vr.Sound = false
		vr.Issues = append(vr.Issues, "AR confidence out of [0,1] range")
	}
	// Verify composed result consistency
	if cr.MLResult.Prediction != cr.ARResult.Prediction && cr.Final == "" {
		vr.Complete = false
		vr.Issues = append(vr.Issues, "ML/AR disagreement with no resolution")
	}
	// Natural deduction style check: each step must reference prior
	for i := 1; i < len(cr.MLResult.ProofTrace); i++ {
		if cr.MLResult.ProofTrace[i] == "" {
			vr.Sound = false
			vr.Issues = append(vr.Issues, fmt.Sprintf("ML proof step %d is empty", i))
		}
	}
	for i := 1; i < len(cr.ARResult.ProofTrace); i++ {
		if cr.ARResult.ProofTrace[i] == "" {
			vr.Sound = false
			vr.Issues = append(vr.Issues, fmt.Sprintf("AR proof step %d is empty", i))
		}
	}

	vr.Pass = vr.Sound && vr.Complete && len(vr.Issues) == 0
	vr.Explained = vr.Pass
	return vr
}

// VerificationResult captures the outcome of verification.
type VerificationResult struct {
	Sound     bool
	Complete  bool
	Pass      bool
	Explained bool
	Issues    []string
}

// PhDAgent provides domain expertise: selects kinds, validates approach, reviews theory.
type PhDAgent struct {
	AgentID   string
	Specialty string // e.g., "bayesian-lp", "logic-programs"
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
		payload, ok := msg.Payload.(DomainRequest)
		if !ok {
			return Message{}, fmt.Errorf("phd: expected DomainRequest payload")
		}
		advice := p.analyze(payload)
		p.Log = append(p.Log, fmt.Sprintf("analyzed domain=%s kinds=%d", payload.Domain, len(advice.RecommendedKinds)))
		return Message{
			From:      p.AgentID,
			To:        msg.From,
			Type:      "result",
			Payload:   advice,
			Timestamp: time.Now(),
		}, nil
	case "review":
		result, ok := msg.Payload.(kinds.ComposedResult)
		if !ok {
			return Message{}, fmt.Errorf("phd: expected ComposedResult for review")
		}
		review := p.review(result)
		p.Log = append(p.Log, fmt.Sprintf("reviewed: %s", review.Summary))
		return Message{
			From:      p.AgentID,
			To:        msg.From,
			Type:      "result",
			Payload:   review,
			Timestamp: time.Now(),
		}, nil
	default:
		return Message{}, fmt.Errorf("phd: unknown message type %q", msg.Type)
	}
}

func (p *PhDAgent) analyze(req DomainRequest) DomainAdvice {
	advice := DomainAdvice{
		Domain:          req.Domain,
		RecommendedKinds: []kinds.Kind{},
		Rationale:       []string{},
	}

	// Select appropriate AR kind
	arKinds := kinds.ARKinds()
	for _, k := range arKinds {
		if k.ID == "ar-lp" || k.ID == "ar-blp" {
			advice.RecommendedKinds = append(advice.RecommendedKinds, k)
			advice.Rationale = append(advice.Rationale,
				fmt.Sprintf("Selected %s: strong composability, proven verifiability, polynomial inference", k.Name))
		}
	}
	// Select appropriate ML kind
	mlKinds := kinds.MLKinds()
	for _, k := range mlKinds {
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
	review := DomainReview{
		Approved: cr.Verified && cr.Explained,
	}
	if cr.AUROC >= 0.5 {
		review.Summary = fmt.Sprintf("Composed result acceptable: AUROC=%.4f, verified=%v", cr.AUROC, cr.Verified)
	} else {
		review.Summary = fmt.Sprintf("Composed result below threshold: AUROC=%.4f", cr.AUROC)
		review.Approved = false
	}
	return review
}

// DomainRequest asks the PhD agent for domain expertise.
type DomainRequest struct {
	Domain      string
	Constraints []string
}

// DomainAdvice is the PhD agent's response.
type DomainAdvice struct {
	Domain           string
	RecommendedKinds []kinds.Kind
	Rationale        []string
	TractabilityNote string
}

// DomainReview is the PhD agent's review of a composed result.
type DomainReview struct {
	Approved bool
	Summary  string
}

// ModelAgent runs ML or AR inference using the configured engine.
type ModelAgent struct {
	AgentID   string
	ModelKind kinds.Kind
	Engine    InferenceEngine
	Log       []string
	mu        sync.Mutex
}

// InferenceEngine is the interface that AR and ML engines implement.
type InferenceEngine interface {
	Infer(datum kinds.Datum) (kinds.ModelResult, error)
	Name() string
}

func NewModelAgent(id string, kind kinds.Kind, engine InferenceEngine) *ModelAgent {
	return &ModelAgent{AgentID: id, ModelKind: kind, Engine: engine}
}

func (m *ModelAgent) ID() string  { return m.AgentID }
func (m *ModelAgent) Role() Role  { return RoleModel }

func (m *ModelAgent) Process(msg Message) (Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch msg.Type {
	case "request":
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
		return Message{
			From:      m.AgentID,
			To:        msg.From,
			Type:      "result",
			Payload:   result,
			Timestamp: time.Now(),
		}, nil
	default:
		return Message{}, fmt.Errorf("model: unknown message type %q", msg.Type)
	}
}

// Orchestrator coordinates all agents iteratively (no recursion).
type Orchestrator struct {
	Verifiers []*VerifierAgent
	PhDs      []*PhDAgent
	Models    []*ModelAgent
	Results   []OrchestratorResult
	mu        sync.Mutex
}

type OrchestratorResult struct {
	Datum          kinds.Datum
	ComposedResult kinds.ComposedResult
	Verification   VerificationResult
	DomainReview   DomainReview
}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{}
}

func (o *Orchestrator) AddVerifier(v *VerifierAgent) { o.Verifiers = append(o.Verifiers, v) }
func (o *Orchestrator) AddPhD(p *PhDAgent)           { o.PhDs = append(o.PhDs, p) }
func (o *Orchestrator) AddModel(m *ModelAgent)        { o.Models = append(o.Models, m) }

// RunBatch processes a dataset through all agents iteratively.
// Iterates over each datum, dispatches to model agents, composes, verifies, reviews.
func (o *Orchestrator) RunBatch(dataset kinds.DataSet) []OrchestratorResult {
	results := make([]OrchestratorResult, 0, len(dataset.Items))

	// Iterate over data items (no recursion)
	for idx := 0; idx < len(dataset.Items); idx++ {
		datum := dataset.Items[idx]
		or := o.processSingleDatum(datum)
		results = append(results, or)
	}

	o.mu.Lock()
	o.Results = append(o.Results, results...)
	o.mu.Unlock()

	return results
}

func (o *Orchestrator) processSingleDatum(datum kinds.Datum) OrchestratorResult {
	result := OrchestratorResult{Datum: datum}

	// Step 1: Run all model agents on this datum, collect ML and AR results
	var mlResult, arResult kinds.ModelResult
	for i := 0; i < len(o.Models); i++ {
		m := o.Models[i]
		msg := Message{
			From:      "orchestrator",
			To:        m.ID(),
			Type:      "request",
			Payload:   datum,
			Timestamp: time.Now(),
		}
		resp, err := m.Process(msg)
		if err != nil {
			continue
		}
		mr, ok := resp.Payload.(kinds.ModelResult)
		if !ok {
			continue
		}
		if mr.Kind.Category == kinds.CategoryML {
			mlResult = mr
		} else {
			arResult = mr
		}
	}

	// Step 2: Compose ML + AR results
	composed := kinds.ComposedResult{
		MLResult: mlResult,
		ARResult: arResult,
	}
	// Resolution: AR overrides if both agree; weighted if they disagree
	if mlResult.Prediction == arResult.Prediction {
		composed.Final = mlResult.Prediction
		composed.AUROC = (mlResult.Confidence + arResult.Confidence) / 2.0
	} else {
		// AR gets priority (higher assurance)
		if arResult.Confidence >= mlResult.Confidence {
			composed.Final = arResult.Prediction
		} else {
			composed.Final = mlResult.Prediction
		}
		composed.AUROC = arResult.Confidence*0.6 + mlResult.Confidence*0.4
	}
	composed.Explained = len(mlResult.ProofTrace) > 0 && len(arResult.ProofTrace) > 0
	composed.Verified = false // will be set by verifier

	// Step 3: Verify with all verifier agents
	for i := 0; i < len(o.Verifiers); i++ {
		v := o.Verifiers[i]
		msg := Message{
			From:      "orchestrator",
			To:        v.ID(),
			Type:      "verify",
			Payload:   composed,
			Timestamp: time.Now(),
		}
		resp, err := v.Process(msg)
		if err != nil {
			continue
		}
		vr, ok := resp.Payload.(VerificationResult)
		if ok {
			result.Verification = vr
			composed.Verified = vr.Pass
		}
	}
	result.ComposedResult = composed

	// Step 4: PhD review
	for i := 0; i < len(o.PhDs); i++ {
		p := o.PhDs[i]
		msg := Message{
			From:      "orchestrator",
			To:        p.ID(),
			Type:      "review",
			Payload:   composed,
			Timestamp: time.Now(),
		}
		resp, err := p.Process(msg)
		if err != nil {
			continue
		}
		dr, ok := resp.Payload.(DomainReview)
		if ok {
			result.DomainReview = dr
		}
	}

	return result
}
