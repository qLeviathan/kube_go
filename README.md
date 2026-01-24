# xAI Infrastructure Engineering Self-Learning Lab

A hands-on, runnable learning package for mastering the infrastructure stack: Kubernetes internals, Envoy/xDS, gRPC, and L3-L7 network debugging.

## Learning Architecture

This package is structured around the core competencies from production traffic engineering:

```
┌─────────────────────────────────────────────────────────────────┐
│                    05-service-discovery                         │
│                 (DNS, xDS Control Planes)                       │
├─────────────────────────────────────────────────────────────────┤
│        02-envoy-xds           │        03-grpc-internals        │
│    (L4/L7 Proxy, xDS API)     │    (Channels, Interceptors)     │
├─────────────────────────────────────────────────────────────────┤
│                  01-kubernetes-fundamentals                     │
│        (Controllers, Informers, Cached Clients, CRDs)           │
├─────────────────────────────────────────────────────────────────┤
│                    04-network-debugging                         │
│           (L3→L7 Path Analysis, iptables, eBPF)                 │
└─────────────────────────────────────────────────────────────────┘
```

## Prerequisites

- Go 1.21+ (`brew install go` or download from golang.org)
- Docker Desktop (for local containers)
- `kind` for local Kubernetes (`go install sigs.k8s.io/kind@latest`)
- `kubectl` (`brew install kubectl`)
- `grpcurl` for gRPC debugging (`brew install grpcurl`)

## Quick Start

```bash
# 1. Install dependencies
./scripts/setup.sh

# 2. Start local Kubernetes cluster
kind create cluster --name infra-lab

# 3. Run module tests
cd 01-kubernetes-fundamentals && go test -v ./...
```

## Module Deep-Dives

### Module 01: Kubernetes Fundamentals
**Objective**: Understand how k8s cached clients (informers) work internally.

Key concepts:
- SharedInformer architecture: ListWatch → DeltaFIFO → Indexer
- Controller pattern: Reconcile loop with work queue
- Client-go cache mechanics: How watches maintain local state

### Module 02: Envoy/xDS
**Objective**: Build an xDS control plane from scratch.

Key concepts:
- xDS protocol variants (ADS, SotW, Delta)
- Listener Discovery Service (LDS)
- Cluster Discovery Service (CDS)
- Endpoint Discovery Service (EDS)
- Route Discovery Service (RDS)

### Module 03: gRPC Internals
**Objective**: Understand channel management, connection pooling, load balancing.

Key concepts:
- Channel lifecycle and subchannel management
- Name resolution and load balancing policies
- Interceptors (unary and streaming)
- Health checking and keepalive

### Module 04: Network Debugging
**Objective**: Trace full L3→L7 path for cross-environment calls.

Key concepts:
- iptables/nftables packet flow
- TCP connection establishment and troubleshooting
- TLS handshake debugging
- gRPC-over-HTTP/2 specifics

### Module 05: Service Discovery
**Objective**: Build DNS and xDS-based discovery systems.

Key concepts:
- CoreDNS architecture and plugins
- xDS as universal discovery protocol
- Kubernetes DNS resolution mechanics

## How to Use This Package

Each module contains:
1. `README.md` - Conceptual deep-dive
2. `main.go` or `cmd/` - Runnable code
3. `*_test.go` - Tests that demonstrate behavior
4. `manifests/` - Kubernetes/Docker configs where applicable

Run tests to see internals in action:
```bash
cd 01-kubernetes-fundamentals
go test -v -run TestInformerMechanics
```
