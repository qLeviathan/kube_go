# Module 02: Envoy and xDS Protocol Deep-Dive

## The Question They'll Ask

> "Can you tell us how Envoy scales and manages state?"

This module gives you a complete working xDS control plane and explains Envoy's architecture.

## Envoy Architecture Overview

Envoy is a **single-process, multi-threaded** proxy with a unique threading model:

```
┌────────────────────────────────────────────────────────────────────────────┐
│                            Envoy Process                                   │
│                                                                            │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                         Main Thread                                   │  │
│  │  - Configuration management (xDS)                                    │  │
│  │  - Stats flushing                                                    │  │
│  │  - Admin interface                                                   │  │
│  │  - Cluster manager (connection pool management)                      │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                            │
│  ┌────────────────┐  ┌────────────────┐       ┌────────────────┐          │
│  │  Worker Thread │  │  Worker Thread │  ...  │  Worker Thread │          │
│  │                │  │                │       │                │          │
│  │  - Listener    │  │  - Listener    │       │  - Listener    │          │
│  │  - Connections │  │  - Connections │       │  - Connections │          │
│  │  - Filters     │  │  - Filters     │       │  - Filters     │          │
│  │                │  │                │       │                │          │
│  │  (event loop)  │  │  (event loop)  │       │  (event loop)  │          │
│  └────────────────┘  └────────────────┘       └────────────────┘          │
│                                                                            │
│  Key Insight: Each worker is nearly independent.                           │
│  Connections are assigned to workers via SO_REUSEPORT.                     │
│  Workers share nothing except read-only config snapshots.                  │
└────────────────────────────────────────────────────────────────────────────┘
```

## How Envoy Manages State

### 1. Configuration State (xDS)

Envoy receives configuration from a control plane via xDS protocols:

| Protocol | Full Name | What It Configures |
|----------|-----------|-------------------|
| LDS | Listener Discovery Service | Listeners (ports, protocols) |
| RDS | Route Discovery Service | Routes (URL → cluster mapping) |
| CDS | Cluster Discovery Service | Clusters (upstream services) |
| EDS | Endpoint Discovery Service | Endpoints (IPs/ports) |
| SDS | Secret Discovery Service | TLS certificates |

Configuration updates are **atomic** at the resource level and **eventually consistent** across workers.

### 2. Connection State

Each worker thread independently manages its connections:

```
Worker Thread N
├── Listener Socket (shared via SO_REUSEPORT)
├── Connection Pool (per-cluster, per-worker)
│   ├── Active connections to upstream
│   └── Pending requests queue
├── Filter Chains (instantiated per-connection)
│   ├── Network filters (TCP-level)
│   └── HTTP filters (L7-level)
└── Local Stats (aggregated periodically)
```

### 3. Cluster Manager & Connection Pooling

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Cluster Manager                               │
│                                                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                    Cluster: service-a                          │  │
│  │                                                                │  │
│  │  Load Balancer: round_robin | least_request | ring_hash        │  │
│  │                                                                │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │  │
│  │  │ Endpoint 1  │  │ Endpoint 2  │  │ Endpoint 3  │            │  │
│  │  │ 10.0.1.5:80 │  │ 10.0.1.6:80 │  │ 10.0.1.7:80 │            │  │
│  │  │             │  │             │  │             │            │  │
│  │  │ Health: OK  │  │ Health: OK  │  │ Health: FAIL│            │  │
│  │  │ Weight: 1   │  │ Weight: 1   │  │ (ejected)   │            │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘            │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  Circuit Breaker State:                                             │
│  - max_connections: 1024                                            │
│  - max_pending_requests: 1024                                       │
│  - max_requests: 1024 (HTTP/2)                                      │
│  - max_retries: 3                                                   │
└─────────────────────────────────────────────────────────────────────┘
```

## xDS Protocol Mechanics

### State of the World (SotW) vs Delta

**SotW**: Control plane sends complete resource set on each update.
- Simpler to implement
- Higher bandwidth for large configurations

**Delta (Incremental)**: Only changed resources are sent.
- More efficient for large deployments
- Requires more complex state tracking

### The xDS Subscription Model

```go
// Client sends DiscoveryRequest
DiscoveryRequest {
    version_info: "42"           // Last version ACKed
    resource_names: ["cluster-a"] // Which resources to subscribe to
    type_url: "type.googleapis.com/envoy.config.cluster.v3.Cluster"
    response_nonce: "abc123"     // Correlates with last response
}

// Server responds with DiscoveryResponse
DiscoveryResponse {
    version_info: "43"           // New version
    resources: [...]             // Actual cluster configs
    nonce: "def456"              // For ACK/NACK correlation
    type_url: "type.googleapis.com/envoy.config.cluster.v3.Cluster"
}
```

## Files in This Module

- `xds_control_plane.go` - Complete working xDS server
- `xds_control_plane_test.go` - Tests demonstrating xDS mechanics
- `envoy.yaml` - Static Envoy config pointing to our control plane
- `docker-compose.yaml` - Run Envoy + control plane locally
