# Module 01: Kubernetes Fundamentals - Cached Clients & Controllers

## The Question They'll Ask

> "Can you tell us how k8s cached clients work?"

This module gives you the complete answer through runnable code.

## Architectural Overview

Kubernetes client-go implements a sophisticated caching layer that transforms the naive "poll-the-API-server" pattern into an efficient event-driven system. The architecture:

```
┌──────────────────────────────────────────────────────────────────────┐
│                         API Server                                    │
│    ┌─────────────┐                                                   │
│    │  etcd watch │ ←── persistent watch connection                   │
│    └──────┬──────┘                                                   │
└───────────┼──────────────────────────────────────────────────────────┘
            │ HTTP/2 streaming (chunked transfer-encoding)
            ▼
┌──────────────────────────────────────────────────────────────────────┐
│                       Reflector (ListWatch)                          │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ 1. Initial LIST: Get all objects + resourceVersion              │  │
│  │ 2. WATCH from resourceVersion: Stream deltas                    │  │
│  │ 3. On desync: Re-LIST and restart watch                         │  │
│  └────────────────────────────────────────────────────────────────┘  │
└──────────────────────────┬───────────────────────────────────────────┘
                           │ Deltas (Added/Modified/Deleted)
                           ▼
┌──────────────────────────────────────────────────────────────────────┐
│                         DeltaFIFO                                    │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │ Queue of (object, DeltaType) pairs                              │  │
│  │ Deduplication: Multiple updates to same object → single entry   │  │
│  │ Ordering: FIFO within object, but parallel across objects       │  │
│  └────────────────────────────────────────────────────────────────┘  │
└──────────────────────────┬───────────────────────────────────────────┘
                           │ Pop()
                           ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    SharedIndexInformer                               │
│  ┌─────────────────────┐    ┌──────────────────────────────────┐    │
│  │       Indexer       │    │      Event Handlers               │    │
│  │   (thread-safe      │    │  OnAdd(obj)                       │    │
│  │    in-memory cache) │    │  OnUpdate(oldObj, newObj)         │    │
│  │                     │    │  OnDelete(obj)                    │    │
│  │  Index by:          │    │                                   │    │
│  │  - namespace        │    │  → Enqueue to workqueue           │    │
│  │  - labels           │    └──────────────────────────────────┘    │
│  │  - custom indexers  │                                            │
│  └─────────────────────┘                                            │
└──────────────────────────────────────────────────────────────────────┘
```

## Key Invariants

1. **resourceVersion Consistency**: The watch starts from the LIST's resourceVersion, ensuring no events are missed.

2. **Local Cache Consistency**: The Indexer reflects the state at the most recent resourceVersion seen. Reads from cache are slightly stale but consistent.

3. **Event Ordering**: Events for a single object arrive in order. Events across objects may be reordered.

4. **Resync Period**: Periodic re-LIST operations catch any missed deltas (defense against bugs/network issues).

## Run the Code

```bash
go mod init infra-lab/kubernetes-fundamentals
go get k8s.io/client-go@latest
go test -v ./...
```

## Files in This Module

- `informer_mechanics.go` - Core informer implementation walkthrough
- `informer_mechanics_test.go` - Runnable tests demonstrating behavior
- `controller_pattern.go` - Full controller implementation
- `custom_indexer.go` - Building custom cache indexes
