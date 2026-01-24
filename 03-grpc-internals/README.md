# Module 03: gRPC Internals - Channels, Connections, and Load Balancing

## Why This Matters

The job posting specifically asks about "Deep experience with gRPC Client libraries" and the interview question describes:

> "How would a gRPC client in a cloud environment call a gRPC server in an on-prem server? Describe the entire network path..."

This module dissects gRPC internals so you can answer that question definitively.

## gRPC Architecture Layers

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Application Code                                   │
│  stub.SayHello(ctx, &pb.HelloRequest{Name: "world"})                        │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Channel                                         │
│  - Logical connection to a service                                          │
│  - Contains Subchannels (actual TCP connections)                            │
│  - Handles name resolution, load balancing                                  │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                        Name Resolver                                     ││
│  │  dns:///myservice.default.svc.cluster.local → [10.0.1.5, 10.0.1.6]     ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                       Load Balancer                                      ││
│  │  pick_first | round_robin | grpclb | xds                                ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                    │                                         │
│                                    ▼                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                      │
│  │ Subchannel 1 │  │ Subchannel 2 │  │ Subchannel 3 │                      │
│  │ 10.0.1.5:443 │  │ 10.0.1.6:443 │  │ 10.0.1.7:443 │                      │
│  │  (READY)     │  │  (READY)     │  │  (CONNECTING)│                      │
│  └──────────────┘  └──────────────┘  └──────────────┘                      │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                          HTTP/2 Transport                                    │
│  - Single TCP connection per subchannel                                     │
│  - Multiple streams (RPCs) multiplexed                                      │
│  - Flow control per-stream and per-connection                               │
│  - HPACK header compression                                                 │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              TCP/TLS                                         │
│  - TLS 1.3 with ALPN (h2)                                                   │
│  - TCP keepalives for connection health                                     │
│  - Socket options (TCP_NODELAY, etc.)                                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Key Concepts

### 1. Channel Lifecycle

```
                    ┌─────────┐
                    │  IDLE   │◄────────────────────────┐
                    └────┬────┘                         │
                         │ RPC call                     │ idle_timeout
                         ▼                              │
                    ┌─────────┐                         │
            ┌───────│CONNECTING│───────┐                │
            │       └─────────┘       │                │
            │ success          failure │                │
            ▼                          ▼                │
       ┌─────────┐              ┌──────────────┐       │
       │  READY  │              │TRANSIENT_FAIL│───────┘
       └────┬────┘              └──────────────┘
            │ connection lost         ▲ backoff retry
            └─────────────────────────┘
```

### 2. Subchannel States

Each subchannel (connection to a single backend) has its own state machine:

- **IDLE**: No connection, will connect on demand
- **CONNECTING**: TCP/TLS handshake in progress
- **READY**: Connected, can accept RPCs
- **TRANSIENT_FAILURE**: Connection failed, backing off
- **SHUTDOWN**: Permanently closed

### 3. Load Balancing Policies

| Policy | Behavior | Use Case |
|--------|----------|----------|
| `pick_first` | Use first working address | Single backend, debugging |
| `round_robin` | Rotate through all backends | General purpose |
| `grpclb` | External load balancer service | Legacy GCP |
| `xds` | Envoy-style service mesh | Modern k8s, Istio |

### 4. Name Resolution

```
Target: dns:///myservice.default.svc.cluster.local:443

Resolver Flow:
1. Parse scheme (dns://) → use DNS resolver
2. DNS lookup → A records [10.0.1.5, 10.0.1.6, 10.0.1.7]
3. Create addresses with attributes (port, metadata)
4. Pass to load balancer for subchannel management
5. Re-resolve on:
   - TTL expiration
   - All subchannels fail
   - Explicit trigger
```

## Files in This Module

- `grpc_channel.go` - Channel and subchannel internals
- `grpc_channel_test.go` - Demonstrations of behavior
- `load_balancer.go` - Custom load balancer implementation
- `interceptors.go` - Unary and streaming interceptors
- `health_check.go` - gRPC health checking protocol
