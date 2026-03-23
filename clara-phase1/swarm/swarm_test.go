package swarm

import (
	"testing"
	"time"

	"github.com/clara-phase1/agents"
	"github.com/clara-phase1/kinds"
	"github.com/clara-phase1/memory"
)

// --- Mesh Tests ---

func TestMeshRegisterUnregister(t *testing.T) {
	m := NewMeshNetwork(10)
	m.Register("agent-1")
	m.Register("agent-2")

	if m.AgentCount() != 2 {
		t.Errorf("expected 2 agents, got %d", m.AgentCount())
	}

	m.Unregister("agent-1")
	if m.AgentCount() != 1 {
		t.Errorf("expected 1 agent after unregister, got %d", m.AgentCount())
	}
}

func TestMeshSendReceive(t *testing.T) {
	m := NewMeshNetwork(10)
	m.Register("sender")
	m.Register("receiver")

	msg := agents.Message{Type: "test", Payload: "hello"}
	err := m.Send("sender", "receiver", msg)
	if err != nil {
		t.Fatalf("send error: %v", err)
	}

	received, ok := m.Receive("receiver")
	if !ok {
		t.Fatal("expected to receive message")
	}
	if received.From != "sender" {
		t.Errorf("expected from sender, got %s", received.From)
	}
	if received.Payload != "hello" {
		t.Errorf("expected payload hello, got %v", received.Payload)
	}
}

func TestMeshSendToUnregistered(t *testing.T) {
	m := NewMeshNetwork(10)
	m.Register("sender")

	err := m.Send("sender", "ghost", agents.Message{})
	if err == nil {
		t.Error("expected error sending to unregistered agent")
	}
}

func TestMeshReceiveEmpty(t *testing.T) {
	m := NewMeshNetwork(10)
	m.Register("agent")

	_, ok := m.Receive("agent")
	if ok {
		t.Error("expected no message from empty inbox")
	}
}

func TestMeshBroadcast(t *testing.T) {
	m := NewMeshNetwork(10)
	m.Register("boss")
	m.Register("worker-1")
	m.Register("worker-2")
	m.Register("isolated") // not routed

	m.AddRoute("boss", "worker-1")
	m.AddRoute("boss", "worker-2")

	sent := m.Broadcast("boss", agents.Message{Type: "directive", Payload: "go"})
	if sent != 2 {
		t.Errorf("expected 2 sent, got %d", sent)
	}

	// workers should have messages
	_, ok1 := m.Receive("worker-1")
	_, ok2 := m.Receive("worker-2")
	if !ok1 || !ok2 {
		t.Error("workers should have received broadcast")
	}

	// isolated should not
	_, ok3 := m.Receive("isolated")
	if ok3 {
		t.Error("isolated agent should not receive broadcast")
	}
}

func TestMeshDrainInbox(t *testing.T) {
	m := NewMeshNetwork(10)
	m.Register("agent")

	m.Send("x", "agent", agents.Message{Type: "a"})
	m.Send("y", "agent", agents.Message{Type: "b"})
	m.Send("z", "agent", agents.Message{Type: "c"})

	msgs := m.DrainInbox("agent")
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages, got %d", len(msgs))
	}

	// Should be empty now
	msgs2 := m.DrainInbox("agent")
	if len(msgs2) != 0 {
		t.Errorf("expected 0 after drain, got %d", len(msgs2))
	}
}

func TestMeshRegisteredAgents(t *testing.T) {
	m := NewMeshNetwork(10)
	m.Register("a")
	m.Register("b")

	ids := m.RegisteredAgents()
	if len(ids) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(ids))
	}
}

func TestMeshBidirectionalRoutes(t *testing.T) {
	m := NewMeshNetwork(10)
	m.Register("a")
	m.Register("b")
	m.AddRoute("a", "b")

	// Both directions should work
	m.Send("a", "b", agents.Message{Type: "test"})
	_, ok := m.Receive("b")
	if !ok {
		t.Error("b should receive from a")
	}

	m.Send("b", "a", agents.Message{Type: "test"})
	_, ok = m.Receive("a")
	if !ok {
		t.Error("a should receive from b")
	}
}

// --- AutoScaler Tests ---

func TestAutoScalerDefaults(t *testing.T) {
	cfg := DefaultSwarmConfig()
	as := NewAutoScaler(cfg)

	if as == nil {
		t.Fatal("expected non-nil autoscaler")
	}

	// With no metrics, should not scale
	decisions := as.Evaluate()
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions, got %d", len(decisions))
	}
}

func TestAutoScalerScaleUp(t *testing.T) {
	cfg := DefaultSwarmConfig()
	cfg.ScaleUpThreshold = 3
	as := NewAutoScaler(cfg)

	as.RecordPendingTasks(5) // above threshold
	as.RecordAgentCount(agents.RoleModel, 2)
	as.RecordAgentCount(agents.RoleVerifier, 1)

	decisions := as.Evaluate()
	scaleUps := 0
	for _, d := range decisions {
		if d.Action == "scale_up" {
			scaleUps++
		}
	}
	if scaleUps == 0 {
		t.Error("expected at least one scale_up decision")
	}
}

func TestAutoScalerScaleDown(t *testing.T) {
	cfg := DefaultSwarmConfig()
	cfg.ScaleDownIdleSec = 1
	as := NewAutoScaler(cfg)

	as.RecordAgentCount(agents.RoleVerifier, 3)
	as.Metrics.IdleAgents["verifier-3"] = time.Now().Add(-5 * time.Second) // idle for 5s

	decisions := as.Evaluate()
	scaleDowns := 0
	for _, d := range decisions {
		if d.Action == "scale_down" {
			scaleDowns++
		}
	}
	if scaleDowns == 0 {
		t.Error("expected scale_down decision for idle agent")
	}
}

func TestAutoScalerRecordTask(t *testing.T) {
	cfg := DefaultSwarmConfig()
	as := NewAutoScaler(cfg)

	as.RecordTaskStart("agent-1")
	if _, exists := as.Metrics.IdleAgents["agent-1"]; exists {
		t.Error("agent should not be idle after task start")
	}

	as.RecordTaskEnd("agent-1", 100)
	if _, exists := as.Metrics.IdleAgents["agent-1"]; !exists {
		t.Error("agent should be idle after task end")
	}
	if as.Metrics.TasksCompleted != 1 {
		t.Errorf("expected 1 completed, got %d", as.Metrics.TasksCompleted)
	}
}

// --- Controller Tests ---

func TestControllerCreation(t *testing.T) {
	cfg := DefaultSwarmConfig()
	mem := memory.NewStore("/tmp/no.json")
	ctrl := NewController(cfg, mem)

	if ctrl == nil {
		t.Fatal("expected non-nil controller")
	}
	if ctrl.Boss == nil {
		t.Fatal("expected non-nil boss")
	}
	// Boss should be registered
	status := ctrl.Status()
	if status.TotalAgents != 1 {
		t.Errorf("expected 1 agent (boss), got %d", status.TotalAgents)
	}
}

func TestControllerSpawnAgents(t *testing.T) {
	cfg := DefaultSwarmConfig()
	mem := memory.NewStore("/tmp/no.json")
	ctrl := NewController(cfg, mem)

	ctrl.SpawnVerifier("verifier-1")
	ctrl.SpawnPhD("phd-1", "logic")

	status := ctrl.Status()
	if status.TotalAgents != 3 { // boss + verifier + phd
		t.Errorf("expected 3 agents, got %d", status.TotalAgents)
	}
}

func TestControllerRetireAgent(t *testing.T) {
	cfg := DefaultSwarmConfig()
	mem := memory.NewStore("/tmp/no.json")
	ctrl := NewController(cfg, mem)

	ctrl.SpawnVerifier("verifier-1")
	ctrl.RetireAgent("verifier-1")

	status := ctrl.Status()
	if status.TotalAgents != 1 { // just boss
		t.Errorf("expected 1 agent after retire, got %d", status.TotalAgents)
	}
}

func TestControllerSubmitTask(t *testing.T) {
	cfg := DefaultSwarmConfig()
	mem := memory.NewStore("/tmp/no.json")
	ctrl := NewController(cfg, mem)

	id := ctrl.Submit("infer", 1, "payload", "")
	if id == "" {
		t.Error("expected non-empty task ID")
	}

	status := ctrl.Status()
	if status.PendingTasks != 1 {
		t.Errorf("expected 1 pending task, got %d", status.PendingTasks)
	}
}

func TestControllerRunDataset(t *testing.T) {
	cfg := DefaultSwarmConfig()
	mem := memory.NewStore("/tmp/no.json")
	ctrl := NewController(cfg, mem)

	// Create a dummy engine
	engine := &dummyEngine{name: "dummy"}
	ctrl.SpawnVerifier("verifier-1")
	ctrl.SpawnPhD("phd-1", "general")
	ctrl.SpawnModel("model-ml", kinds.KindBayesNets, engine)
	ctrl.SpawnModel("model-ar", kinds.KindLogicPrograms, engine)

	dataset := kinds.DataSet{
		Name: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"a": 0.8, "b": 0.3}, Label: "x"},
		},
	}

	results := ctrl.RunDataset(dataset, "ar_priority")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestControllerSummary(t *testing.T) {
	cfg := DefaultSwarmConfig()
	mem := memory.NewStore("/tmp/no.json")
	ctrl := NewController(cfg, mem)

	s := ctrl.Summary()
	if s == "" {
		t.Error("expected non-empty summary")
	}
}

func TestControllerPeakAgents(t *testing.T) {
	cfg := DefaultSwarmConfig()
	mem := memory.NewStore("/tmp/no.json")
	ctrl := NewController(cfg, mem)

	ctrl.SpawnVerifier("v1")
	ctrl.SpawnVerifier("v2")
	ctrl.SpawnVerifier("v3")
	ctrl.RetireAgent("v3")
	ctrl.RetireAgent("v2")

	status := ctrl.Status()
	if status.PeakAgents != 4 { // boss + 3 verifiers at peak
		t.Errorf("expected peak 4, got %d", status.PeakAgents)
	}
}

// dummyEngine implements agents.InferenceEngine for testing.
type dummyEngine struct {
	name string
}

func (d *dummyEngine) Infer(datum kinds.Datum) (kinds.ModelResult, error) {
	return kinds.NewModelResult("predicted", 0.8, kinds.KindBayesNets, []string{"test proof"}), nil
}

func (d *dummyEngine) Name() string {
	return d.name
}
