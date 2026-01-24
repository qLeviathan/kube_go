# Module 04: Network Debugging - L3 to L7 Path Analysis

## The Interview Question

> "How would a gRPC client in a cloud environment call a gRPC server in an on-prem server? Describe the entire network path and any issues to watch out for, including Service Discovery / DNS, gRPC channel management, egress proxies, VPC routing, peering/PNI, edge caching / CDN, L4 loadbalancing devices, host networking + virtualization, k8s networking, L7 routing, TLS / authnz, TCP/IP"

This module gives you the complete answer with debugging commands at each layer.

## The Full Network Path

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     CLOUD ENVIRONMENT (GCP/AWS)                              │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │ Kubernetes Cluster                                                       ││
│  │                                                                          ││
│  │  ┌──────────────────────────────────────────────────────────────────┐   ││
│  │  │ Pod: grpc-client                                                  │   ││
│  │  │                                                                   │   ││
│  │  │  1. DNS: service.onprem.company.com → [IP via ExternalDNS/split]│   ││
│  │  │  2. gRPC Channel: dial, resolve, pick subchannel                 │   ││
│  │  │  3. Application → gRPC stub → HTTP/2 frames                      │   ││
│  │  │                                                                   │   ││
│  │  └───────────────────────────┬──────────────────────────────────────┘   ││
│  │                              │ (eth0 via veth pair)                      ││
│  │                              ▼                                           ││
│  │  ┌──────────────────────────────────────────────────────────────────┐   ││
│  │  │ CNI: Pod Network (10.244.x.x)                                     │   ││
│  │  │  4. iptables MASQUERADE / Cilium eBPF                            │   ││
│  │  │  5. SNAT to node IP for external traffic                         │   ││
│  │  └───────────────────────────┬──────────────────────────────────────┘   ││
│  │                              │                                           ││
│  └──────────────────────────────┼───────────────────────────────────────────┘│
│                                 │ (Node's eth0)                              │
│                                 ▼                                            │
│  ┌──────────────────────────────────────────────────────────────────────────┐│
│  │ VPC Routing                                                              ││
│  │  6. Route table: 10.1.0.0/16 (on-prem) → VPN Gateway / Interconnect    ││
│  │  7. Security Groups / Firewall rules                                    ││
│  └───────────────────────────────┬──────────────────────────────────────────┘│
│                                  │                                           │
│  ┌───────────────────────────────▼──────────────────────────────────────────┐│
│  │ Egress Path (NAT / Proxy)                                                ││
│  │  8. Cloud NAT (if needed for public IPs)                                ││
│  │  9. OR: Egress Proxy (Envoy) for policy/logging                         ││
│  └───────────────────────────────┬──────────────────────────────────────────┘│
└──────────────────────────────────┼───────────────────────────────────────────┘
                                   │
                    ┌──────────────▼──────────────┐
                    │  INTERCONNECT / VPN TUNNEL  │
                    │  10. IPsec / MACsec         │
                    │  11. MTU considerations     │
                    │  12. Latency: +2-10ms       │
                    └──────────────┬──────────────┘
                                   │
┌──────────────────────────────────┼───────────────────────────────────────────┐
│                    ON-PREM ENVIRONMENT                                       │
│                                  │                                           │
│  ┌───────────────────────────────▼──────────────────────────────────────────┐│
│  │ Edge Router / Firewall                                                   ││
│  │  13. ACLs, stateful inspection                                          ││
│  │  14. NAT (if cloud IPs not routable internally)                         ││
│  └───────────────────────────────┬──────────────────────────────────────────┘│
│                                  │                                           │
│  ┌───────────────────────────────▼──────────────────────────────────────────┐│
│  │ L4 Load Balancer (F5, HAProxy, etc.)                                     ││
│  │  15. TCP connection termination OR passthrough                           ││
│  │  16. Health checks against backends                                      ││
│  │  17. Connection limits, timeouts                                         ││
│  └───────────────────────────────┬──────────────────────────────────────────┘│
│                                  │                                           │
│  ┌───────────────────────────────▼──────────────────────────────────────────┐│
│  │ Kubernetes Cluster (On-Prem)                                             ││
│  │                                                                          ││
│  │  ┌──────────────────────────────────────────────────────────────────┐   ││
│  │  │ Ingress / Gateway                                                 │   ││
│  │  │  18. TLS termination (certificate validation)                    │   ││
│  │  │  19. L7 routing (:authority header → service)                    │   ││
│  │  │  20. Authentication (mTLS, JWT)                                  │   ││
│  │  └───────────────────────────┬──────────────────────────────────────┘   ││
│  │                              │                                           ││
│  │  ┌───────────────────────────▼──────────────────────────────────────┐   ││
│  │  │ Service Mesh (Istio/Linkerd sidecar) - Optional                   │   ││
│  │  │  21. mTLS between pods                                           │   ││
│  │  │  22. Observability (traces, metrics)                             │   ││
│  │  └───────────────────────────┬──────────────────────────────────────┘   ││
│  │                              │                                           ││
│  │  ┌───────────────────────────▼──────────────────────────────────────┐   ││
│  │  │ Pod: grpc-server                                                  │   ││
│  │  │  23. gRPC server receives HTTP/2 request                         │   ││
│  │  │  24. Deserialize protobuf, execute handler                       │   ││
│  │  │  25. Response follows reverse path                               │   ││
│  │  └──────────────────────────────────────────────────────────────────┘   ││
│  │                                                                          ││
│  └──────────────────────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────────────────────┘
```

## Issues to Watch Out For

### Layer 3/4: Network and Transport

| Issue | Symptoms | Debug Command |
|-------|----------|---------------|
| MTU mismatch | Connections hang, large requests fail | `ping -M do -s 1472 <target>` |
| PMTU black hole | Works for small requests, fails for large | `tracepath <target>` |
| Asymmetric routing | Intermittent drops, RSTs | Check firewall state tables |
| NAT timeout | Long-idle connections drop | `cat /proc/sys/net/netfilter/nf_conntrack_tcp_timeout_established` |
| TCP window scaling | Slow throughput | `ss -ti` to check window sizes |

### Layer 7: Application

| Issue | Symptoms | Debug Command |
|-------|----------|---------------|
| gRPC GOAWAY | Connections reset periodically | Check server max_connection_age |
| HTTP/2 SETTINGS mismatch | Stream errors | `grpcurl -vv` to see SETTINGS |
| Deadline exceeded | Timeouts despite server responding | Check all proxy timeouts |
| TLS handshake failure | Connection refused at TLS | `openssl s_client -connect` |
| ALPN mismatch | TLS works, HTTP/2 fails | Check `h2` in ALPN |

### Kubernetes-Specific

| Issue | Symptoms | Debug Command |
|-------|----------|---------------|
| DNS resolution | Can't resolve service names | `nslookup` from pod |
| kube-proxy IPVS | LoadBalancer not working | `ipvsadm -Ln` |
| CNI issues | Pod-to-pod fails | `kubectl exec -- ping` |
| NetworkPolicy | Blocked connections | Check NetworkPolicy objects |
| Endpoint not ready | Service returns no endpoints | `kubectl get endpoints` |

## Files in This Module

- `network_debugging.go` - Debugging utilities and examples
- `network_debugging_test.go` - Tests demonstrating each layer
- `tcpdump_recipes.md` - Essential tcpdump commands
- `troubleshooting_tree.md` - Decision tree for debugging
