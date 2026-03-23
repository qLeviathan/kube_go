// Package swarm implements autoscaled agent swarm coordination for CLARA.
// Agents communicate via mesh topology (peer-to-peer), not star topology.
// The swarm controller manages agent lifecycle, the mesh handles communication,
// and the autoscaler dynamically spawns/retires agents based on load.
package swarm

import (
	"fmt"
	"sync"
	"time"

	"github.com/clara-phase1/agents"
)

// MeshNetwork provides agent-to-agent communication channels.
// Every agent gets an inbox. Agents can send directly to peers or broadcast.
// This replaces the star topology (all→boss) with true peer communication.
type MeshNetwork struct {
	channels map[string]chan agents.Message
	routes   map[string][]string // agentID -> list of peer IDs
	bufSize  int
	mu       sync.RWMutex
}

// NewMeshNetwork creates a mesh with the given channel buffer size.
func NewMeshNetwork(bufSize int) *MeshNetwork {
	if bufSize <= 0 {
		bufSize = 64
	}
	return &MeshNetwork{
		channels: make(map[string]chan agents.Message),
		routes:   make(map[string][]string),
		bufSize:  bufSize,
	}
}

// Register creates an inbox channel for an agent.
func (m *MeshNetwork) Register(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.channels[agentID]; !exists {
		m.channels[agentID] = make(chan agents.Message, m.bufSize)
		m.routes[agentID] = []string{}
	}
}

// Unregister closes an agent's channel and removes all routes.
func (m *MeshNetwork) Unregister(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ch, exists := m.channels[agentID]; exists {
		close(ch)
		delete(m.channels, agentID)
	}
	delete(m.routes, agentID)

	// Remove from other agents' route lists
	for id, peers := range m.routes {
		filtered := make([]string, 0, len(peers))
		for _, p := range peers {
			if p != agentID {
				filtered = append(filtered, p)
			}
		}
		m.routes[id] = filtered
	}
}

// AddRoute establishes a bidirectional communication link between two agents.
func (m *MeshNetwork) AddRoute(from, to string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !containsStr(m.routes[from], to) {
		m.routes[from] = append(m.routes[from], to)
	}
	if !containsStr(m.routes[to], from) {
		m.routes[to] = append(m.routes[to], from)
	}
}

// Send delivers a message directly to a specific agent's inbox.
func (m *MeshNetwork) Send(from, to string, msg agents.Message) error {
	m.mu.RLock()
	ch, exists := m.channels[to]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("mesh: agent %q not registered", to)
	}

	msg.From = from
	msg.To = to
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	select {
	case ch <- msg:
		return nil
	default:
		return fmt.Errorf("mesh: agent %q inbox full", to)
	}
}

// Broadcast sends a message to all peers of the sending agent.
func (m *MeshNetwork) Broadcast(from string, msg agents.Message) int {
	m.mu.RLock()
	peers := m.routes[from]
	m.mu.RUnlock()

	sent := 0
	for _, peer := range peers {
		if err := m.Send(from, peer, msg); err == nil {
			sent++
		}
	}
	return sent
}

// Receive gets the next message from an agent's inbox (non-blocking).
func (m *MeshNetwork) Receive(agentID string) (agents.Message, bool) {
	m.mu.RLock()
	ch, exists := m.channels[agentID]
	m.mu.RUnlock()

	if !exists {
		return agents.Message{}, false
	}

	select {
	case msg, ok := <-ch:
		return msg, ok
	default:
		return agents.Message{}, false
	}
}

// DrainInbox returns all pending messages for an agent.
func (m *MeshNetwork) DrainInbox(agentID string) []agents.Message {
	m.mu.RLock()
	ch, exists := m.channels[agentID]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	var msgs []agents.Message
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return msgs
			}
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

// AgentCount returns the number of registered agents.
func (m *MeshNetwork) AgentCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.channels)
}

// RegisteredAgents returns the IDs of all registered agents.
func (m *MeshNetwork) RegisteredAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.channels))
	for id := range m.channels {
		ids = append(ids, id)
	}
	return ids
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
