package swarm

import (
	"fmt"
	"sync"
	"time"

	"github.com/clara-phase1/agents"
	"github.com/clara-phase1/kinds"
	"github.com/clara-phase1/memory"
)

// TaskStatus represents the lifecycle of a task.
type TaskStatus string

const (
	TaskPending  TaskStatus = "pending"
	TaskRunning  TaskStatus = "running"
	TaskDone     TaskStatus = "done"
	TaskFailed   TaskStatus = "failed"
)

// Task represents a unit of work dispatched to the swarm.
type Task struct {
	ID         string
	Type       string // "infer", "verify", "review", "compose"
	Priority   int
	Payload    interface{}
	AssignedTo string
	Status     TaskStatus
	Result     interface{}
	SpawnedBy  string   // parent task ID for hierarchical decomposition
	Children   []string // child task IDs
	CreatedAt  time.Time
	StartedAt  time.Time
	DoneAt     time.Time
}

// SwarmStatus is a snapshot of the swarm's current state.
type SwarmStatus struct {
	TotalAgents    int            `json:"total_agents"`
	AgentsByRole   map[string]int `json:"agents_by_role"`
	PendingTasks   int            `json:"pending_tasks"`
	RunningTasks   int            `json:"running_tasks"`
	CompletedTasks int            `json:"completed_tasks"`
	PeakAgents     int            `json:"peak_agents"`
}

// Controller manages the agent swarm lifecycle: spawning, retiring,
// task assignment, and autoscaling. It replaces the flat SuperClaude
// boss pattern with a dynamic mesh-coordinated swarm.
type Controller struct {
	Agents     map[string]agents.Agent
	AgentRoles map[string]agents.Role
	Mesh       *MeshNetwork
	Scaler     *AutoScaler
	Memory     *memory.Store
	Tasks      map[string]*Task
	Boss       *agents.SuperClaudeAgent
	Log        []string
	taskSeq    int
	peakAgents int
	mu         sync.Mutex
}

// NewController creates a swarm controller with initial agents.
func NewController(cfg SwarmConfig, mem *memory.Store) *Controller {
	c := &Controller{
		Agents:     make(map[string]agents.Agent),
		AgentRoles: make(map[string]agents.Role),
		Mesh:       NewMeshNetwork(64),
		Scaler:     NewAutoScaler(cfg),
		Memory:     mem,
		Tasks:      make(map[string]*Task),
	}

	// Create the boss agent
	c.Boss = agents.NewSuperClaudeAgent("swarm-boss")
	c.registerAgent(c.Boss, agents.RoleSuperClaude)

	return c
}

// SpawnVerifier creates and registers a new verifier agent.
func (c *Controller) SpawnVerifier(id string) *agents.VerifierAgent {
	c.mu.Lock()
	defer c.mu.Unlock()

	v := agents.NewVerifierAgent(id)
	c.registerAgentLocked(v, agents.RoleVerifier)
	c.Boss.AddVerifier(v)
	c.log("spawned verifier: %s", id)
	return v
}

// SpawnPhD creates and registers a new PhD agent.
func (c *Controller) SpawnPhD(id, specialty string) *agents.PhDAgent {
	c.mu.Lock()
	defer c.mu.Unlock()

	p := agents.NewPhDAgent(id, specialty)
	c.registerAgentLocked(p, agents.RolePhD)
	c.Boss.AddPhD(p)
	c.log("spawned PhD: %s (specialty: %s)", id, specialty)
	return p
}

// SpawnModel creates and registers a new model agent.
func (c *Controller) SpawnModel(id string, kind kinds.Kind, engine agents.InferenceEngine) *agents.ModelAgent {
	c.mu.Lock()
	defer c.mu.Unlock()

	m := agents.NewModelAgent(id, kind, engine)
	c.registerAgentLocked(m, agents.RoleModel)
	c.Boss.AddModel(m)
	c.log("spawned model: %s (kind: %s)", id, kind.Name)
	return m
}

// RetireAgent removes an agent from the swarm.
func (c *Controller) RetireAgent(agentID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.Agents[agentID]; !exists {
		return
	}

	c.Mesh.Unregister(agentID)
	role := c.AgentRoles[agentID]
	delete(c.Agents, agentID)
	delete(c.AgentRoles, agentID)

	// Update scaler metrics
	count := 0
	for _, r := range c.AgentRoles {
		if r == role {
			count++
		}
	}
	c.Scaler.RecordAgentCount(role, count)

	c.log("retired agent: %s", agentID)
}

// Submit adds a task to the swarm queue.
func (c *Controller) Submit(taskType string, priority int, payload interface{}, parentID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.taskSeq++
	id := fmt.Sprintf("task-%d", c.taskSeq)

	task := &Task{
		ID:        id,
		Type:      taskType,
		Priority:  priority,
		Payload:   payload,
		Status:    TaskPending,
		SpawnedBy: parentID,
		CreatedAt: time.Now(),
	}

	c.Tasks[id] = task

	// Link to parent
	if parentID != "" {
		if parent, ok := c.Tasks[parentID]; ok {
			parent.Children = append(parent.Children, id)
		}
	}

	return id
}

// RunDataset executes the full swarm pipeline on a dataset.
// This is the main entry point: replaces the old sequential boss.runFullEvaluation.
func (c *Controller) RunDataset(dataset kinds.DataSet, strategy string) []agents.OrchestratorResult {
	c.mu.Lock()
	// Send directive
	c.Boss.Process(agents.Message{
		From: "controller", To: c.Boss.ID(), Type: "directive",
		Payload:   fmt.Sprintf("Swarm evaluation: dataset=%s, strategy=%s", dataset.Name, strategy),
		Timestamp: time.Now(),
	})
	c.mu.Unlock()

	// Run evaluation through the boss (which coordinates its sub-agents)
	resp, err := c.Boss.Process(agents.Message{
		From: "controller", To: c.Boss.ID(), Type: "request",
		Payload: dataset, Timestamp: time.Now(),
	})

	if err != nil {
		c.mu.Lock()
		c.log("dataset %s error: %v", dataset.Name, err)
		c.mu.Unlock()
		return nil
	}

	results, ok := resp.Payload.([]agents.OrchestratorResult)
	if !ok {
		return nil
	}

	// Record results in memory
	for _, or := range results {
		c.Memory.RecordPattern(
			featureKeys(or.Datum),
			or.InferenceResult.Final,
			or.InferenceResult.Confidence,
		)
	}

	return results
}

// CheckScale evaluates autoscaling and applies decisions.
func (c *Controller) CheckScale() []ScaleDecision {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update metrics
	pending := 0
	for _, t := range c.Tasks {
		if t.Status == TaskPending {
			pending++
		}
	}
	c.Scaler.RecordPendingTasks(pending)

	decisions := c.Scaler.Evaluate()

	for _, d := range decisions {
		switch d.Action {
		case "scale_up":
			c.log("autoscale: spawning %s", d.Role)
		case "scale_down":
			c.log("autoscale: would retire %s", d.Role)
		}
	}

	return decisions
}

// Status returns a snapshot of the swarm state.
func (c *Controller) Status() SwarmStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	byRole := make(map[string]int)
	for _, role := range c.AgentRoles {
		byRole[string(role)]++
	}

	pending, running, completed := 0, 0, 0
	for _, t := range c.Tasks {
		switch t.Status {
		case TaskPending:
			pending++
		case TaskRunning:
			running++
		case TaskDone:
			completed++
		}
	}

	return SwarmStatus{
		TotalAgents:    len(c.Agents),
		AgentsByRole:   byRole,
		PendingTasks:   pending,
		RunningTasks:   running,
		CompletedTasks: completed,
		PeakAgents:     c.peakAgents,
	}
}

// Summary returns a human-readable summary of the swarm.
func (c *Controller) Summary() string {
	status := c.Status()
	return fmt.Sprintf("Swarm: %d agents (peak %d), %d tasks completed",
		status.TotalAgents, status.PeakAgents, status.CompletedTasks)
}

// registerAgent registers an agent (acquires lock).
func (c *Controller) registerAgent(a agents.Agent, role agents.Role) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registerAgentLocked(a, role)
}

// registerAgentLocked registers an agent (caller holds lock).
func (c *Controller) registerAgentLocked(a agents.Agent, role agents.Role) {
	c.Agents[a.ID()] = a
	c.AgentRoles[a.ID()] = role
	c.Mesh.Register(a.ID())

	// Connect to boss
	c.Mesh.AddRoute(a.ID(), c.Boss.ID())

	// Connect to all agents of same role (mesh, not star)
	for id, r := range c.AgentRoles {
		if r == role && id != a.ID() {
			c.Mesh.AddRoute(a.ID(), id)
		}
	}

	// Update peak
	if len(c.Agents) > c.peakAgents {
		c.peakAgents = len(c.Agents)
	}

	// Update scaler
	count := 0
	for _, r := range c.AgentRoles {
		if r == role {
			count++
		}
	}
	c.Scaler.RecordAgentCount(role, count)
}

func (c *Controller) log(format string, args ...interface{}) {
	c.Log = append(c.Log, fmt.Sprintf("[Swarm] "+format, args...))
}

// featureKeys extracts sorted feature names from a datum.
func featureKeys(d kinds.Datum) []string {
	keys := make([]string, 0, len(d.Features))
	for k := range d.Features {
		keys = append(keys, k)
	}
	return keys
}
