package swarm

import (
	"fmt"
	"sync"
	"time"

	"github.com/clara-phase1/agents"
)

// SwarmConfig defines the bounds for dynamic agent scaling.
type SwarmConfig struct {
	MinVerifiers     int `json:"min_verifiers"`
	MaxVerifiers     int `json:"max_verifiers"`
	MinPhDs          int `json:"min_phds"`
	MaxPhDs          int `json:"max_phds"`
	MinModels        int `json:"min_models"`
	MaxModels        int `json:"max_models"`
	ScaleUpThreshold int `json:"scale_up_threshold"` // pending tasks to trigger scale up
	ScaleDownIdleSec int `json:"scale_down_idle_sec"` // seconds idle before retiring
	MaxSwarmSize     int `json:"max_swarm_size"`
}

// DefaultSwarmConfig returns sensible scaling defaults.
func DefaultSwarmConfig() SwarmConfig {
	return SwarmConfig{
		MinVerifiers:     1,
		MaxVerifiers:     6,
		MinPhDs:          1,
		MaxPhDs:          4,
		MinModels:        2,
		MaxModels:        8,
		ScaleUpThreshold: 5,
		ScaleDownIdleSec: 30,
		MaxSwarmSize:     20,
	}
}

// ScaleDecision is a recommendation from the autoscaler.
type ScaleDecision struct {
	Action string     `json:"action"` // "scale_up", "scale_down", "hold"
	Role   agents.Role `json:"role"`
	Count  int         `json:"count"`
	Reason string     `json:"reason"`
}

// ScaleMetrics tracks the current swarm state for scaling decisions.
type ScaleMetrics struct {
	PendingTasks    int
	ActiveAgents    map[agents.Role]int
	IdleAgents      map[string]time.Time // agentID -> last active time
	TasksCompleted  int
	TotalTaskTimeMs int64
}

// AutoScaler monitors swarm metrics and recommends scaling actions.
type AutoScaler struct {
	Config  SwarmConfig
	Metrics ScaleMetrics
	Log     []string
	mu      sync.Mutex
}

// NewAutoScaler creates an autoscaler with the given config.
func NewAutoScaler(cfg SwarmConfig) *AutoScaler {
	return &AutoScaler{
		Config: cfg,
		Metrics: ScaleMetrics{
			ActiveAgents: make(map[agents.Role]int),
			IdleAgents:   make(map[string]time.Time),
		},
	}
}

// Evaluate checks current metrics and returns scaling decisions.
// This is the core autoscaling logic.
func (as *AutoScaler) Evaluate() []ScaleDecision {
	as.mu.Lock()
	defer as.mu.Unlock()

	var decisions []ScaleDecision

	totalActive := 0
	for _, count := range as.Metrics.ActiveAgents {
		totalActive += count
	}

	// Check if we need to scale UP
	if as.Metrics.PendingTasks >= as.Config.ScaleUpThreshold && totalActive < as.Config.MaxSwarmSize {
		// Scale up models first (inference bottleneck)
		if as.Metrics.ActiveAgents[agents.RoleModel] < as.Config.MaxModels {
			decisions = append(decisions, ScaleDecision{
				Action: "scale_up",
				Role:   agents.RoleModel,
				Count:  1,
				Reason: fmt.Sprintf("%d pending tasks, %d models active (max %d)",
					as.Metrics.PendingTasks, as.Metrics.ActiveAgents[agents.RoleModel], as.Config.MaxModels),
			})
		}

		// Then verifiers
		if as.Metrics.ActiveAgents[agents.RoleVerifier] < as.Config.MaxVerifiers {
			decisions = append(decisions, ScaleDecision{
				Action: "scale_up",
				Role:   agents.RoleVerifier,
				Count:  1,
				Reason: fmt.Sprintf("%d pending tasks, %d verifiers active (max %d)",
					as.Metrics.PendingTasks, as.Metrics.ActiveAgents[agents.RoleVerifier], as.Config.MaxVerifiers),
			})
		}
	}

	// Check if we need to scale DOWN
	now := time.Now()
	idleThreshold := time.Duration(as.Config.ScaleDownIdleSec) * time.Second

	for agentID, lastActive := range as.Metrics.IdleAgents {
		if now.Sub(lastActive) > idleThreshold {
			// Determine role from agent ID prefix
			role := roleFromID(agentID)
			minForRole := as.minForRole(role)
			if as.Metrics.ActiveAgents[role] > minForRole {
				decisions = append(decisions, ScaleDecision{
					Action: "scale_down",
					Role:   role,
					Count:  1,
					Reason: fmt.Sprintf("agent %s idle for %v", agentID, now.Sub(lastActive).Round(time.Second)),
				})
			}
		}
	}

	for _, d := range decisions {
		as.Log = append(as.Log, fmt.Sprintf("[AutoScaler] %s %s x%d: %s", d.Action, d.Role, d.Count, d.Reason))
	}

	return decisions
}

// RecordTaskStart marks an agent as active.
func (as *AutoScaler) RecordTaskStart(agentID string) {
	as.mu.Lock()
	defer as.mu.Unlock()
	delete(as.Metrics.IdleAgents, agentID)
}

// RecordTaskEnd marks an agent as potentially idle.
func (as *AutoScaler) RecordTaskEnd(agentID string, durationMs int64) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.Metrics.IdleAgents[agentID] = time.Now()
	as.Metrics.TasksCompleted++
	as.Metrics.TotalTaskTimeMs += durationMs
}

// RecordPendingTasks updates the pending task count.
func (as *AutoScaler) RecordPendingTasks(count int) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.Metrics.PendingTasks = count
}

// RecordAgentCount updates the count of active agents for a role.
func (as *AutoScaler) RecordAgentCount(role agents.Role, count int) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.Metrics.ActiveAgents[role] = count
}

// minForRole returns the minimum agent count for a role.
func (as *AutoScaler) minForRole(role agents.Role) int {
	switch role {
	case agents.RoleVerifier:
		return as.Config.MinVerifiers
	case agents.RolePhD:
		return as.Config.MinPhDs
	case agents.RoleModel:
		return as.Config.MinModels
	default:
		return 1
	}
}

// roleFromID infers agent role from ID prefix.
func roleFromID(agentID string) agents.Role {
	if len(agentID) >= 8 && agentID[:8] == "verifier" {
		return agents.RoleVerifier
	}
	if len(agentID) >= 3 && agentID[:3] == "phd" {
		return agents.RolePhD
	}
	if len(agentID) >= 5 && agentID[:5] == "model" {
		return agents.RoleModel
	}
	return agents.RoleSuperClaude
}
