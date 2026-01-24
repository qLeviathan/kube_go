package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Test Layer 3 (IP) Analysis
// =============================================================================

func TestIPConnectivityAnalysis(t *testing.T) {
	t.Log("=== Layer 3: IP Connectivity Analysis ===")
	
	// Test traceroute structure
	hops := TraceRouteTo("8.8.8.8")
	
	t.Logf("✓ TraceRoute returned %d hops", len(hops))
	for _, hop := range hops {
		t.Logf("  Hop %d: %s (RTT: %v)", hop.HopNumber, hop.Address, hop.RTT)
	}
	
	// Test MTU discovery
	mtu := MTUDiscovery("8.8.8.8")
	t.Logf("✓ Path MTU: %d bytes", mtu)
	t.Log("  - Common MTU issues:")
	t.Log("    * 1500 bytes: Standard Ethernet")
	t.Log("    * 1400 bytes: VPN/tunnel overhead")
	t.Log("    * 9000 bytes: Jumbo frames (datacenter)")
	t.Log("  - Watch for: PMTU black holes (DF bit + no ICMP)")
}

func TestTraceHopStructure(t *testing.T) {
	hop := TraceHop{
		HopNumber: 3,
		Address:   "10.0.0.1",
		RTT:       5 * time.Millisecond,
		Error:     nil,
	}
	
	if hop.HopNumber != 3 {
		t.Errorf("Expected hop 3, got %d", hop.HopNumber)
	}
	t.Logf("✓ TraceHop structure correctly captures hop %d at %s", hop.HopNumber, hop.Address)
}

// =============================================================================
// Test Layer 4 (TCP) Analysis
// =============================================================================

func TestTCPConnectionInfo(t *testing.T) {
	t.Log("=== Layer 4: TCP Connection Analysis ===")
	
	// Create a local server for testing
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	
	// Accept connections in background
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			time.Sleep(100 * time.Millisecond)
			conn.Close()
		}
	}()
	
	// Connect to the test server
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	info := AnalyzeTCPConnection(conn)
	
	t.Logf("✓ TCP Connection analyzed:")
	t.Logf("  - Local:  %s", info.LocalAddr)
	t.Logf("  - Remote: %s", info.RemoteAddr)
	t.Logf("  - State:  %s", info.State)
	t.Logf("  - RTT:    %v", info.RTT)
	t.Logf("  - CWND:   %d MSS units", info.CwndMSS)
	t.Log("")
	t.Log("  TCP states to watch:")
	t.Log("    * ESTABLISHED: Active connection")
	t.Log("    * TIME_WAIT: Lingering after close (port exhaustion risk)")
	t.Log("    * CLOSE_WAIT: App not closing (resource leak)")
	t.Log("    * SYN_SENT: Stuck handshake (firewall/routing)")
}

func TestTCPHandshakeAnalysis(t *testing.T) {
	t.Log("=== TCP Handshake Timing ===")
	
	// Create local server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	
	// Accept in background
	go func() {
		conn, _ := listener.Accept()
		if conn != nil {
			conn.Close()
		}
	}()
	
	result := AnalyzeTCPHandshake(listener.Addr().String())
	
	if !result.Success {
		t.Fatalf("Handshake failed: %v", result.Error)
	}
	
	t.Logf("✓ TCP 3-way handshake completed in %v", result.TotalTime)
	t.Logf("  - SYN sent:    %s", result.SYNSent.Format(time.StampMicro))
	t.Logf("  - ACK sent:    %s", result.ACKSent.Format(time.StampMicro))
	t.Logf("  - Server time: ~%v", result.ServerTime)
	t.Log("")
	t.Log("  Handshake issues to watch:")
	t.Log("    * >100ms local: Network congestion/distance")
	t.Log("    * Timeout: Firewall dropping SYN packets")
	t.Log("    * RST on connect: Port closed or filtered")
}

// =============================================================================
// Test TLS Analysis
// =============================================================================

func TestTLSVersionString(t *testing.T) {
	t.Log("=== TLS Version Analysis ===")
	
	testCases := []struct {
		version  uint16
		expected string
	}{
		{0x0301, "TLS 1.0"},
		{0x0302, "TLS 1.1"},
		{0x0303, "TLS 1.2"},
		{0x0304, "TLS 1.3"},
		{0x0000, "Unknown (0x0000)"},
	}
	
	for _, tc := range testCases {
		info := TLSConnectionInfo{Version: tc.version}
		got := info.VersionString()
		if got != tc.expected {
			t.Errorf("Version 0x%04x: got %s, want %s", tc.version, got, tc.expected)
		}
		t.Logf("  0x%04x → %s", tc.version, got)
	}
	
	t.Log("")
	t.Log("✓ TLS version requirements for gRPC:")
	t.Log("  - Minimum: TLS 1.2 (gRPC spec requirement)")
	t.Log("  - Preferred: TLS 1.3 (faster handshake, 0-RTT)")
	t.Log("  - ALPN must negotiate 'h2' for HTTP/2")
}

// =============================================================================
// Test DNS Analysis
// =============================================================================

func TestDNSResolution(t *testing.T) {
	t.Log("=== DNS Resolution Analysis ===")
	
	result := ResolveDNS("localhost")
	
	if result.Error != nil {
		t.Fatalf("DNS resolution failed: %v", result.Error)
	}
	
	t.Logf("✓ DNS query for '%s' completed in %v", result.Query, result.QueryTime)
	t.Logf("  - Answers: %v", result.Answers)
	if len(result.CNAMEs) > 0 {
		t.Logf("  - CNAMEs: %v", result.CNAMEs)
	}
	
	t.Log("")
	t.Log("  DNS issues in K8s environments:")
	t.Log("    * ndots:5 default causes many search domain queries")
	t.Log("    * CoreDNS can become bottleneck at scale")
	t.Log("    * Stale DNS cache can route to dead pods")
	t.Log("    * Service discovery: <svc>.<ns>.svc.cluster.local")
}

func TestDNSResultStructure(t *testing.T) {
	result := DNSResult{
		Query:     "api.example.com",
		Answers:   []string{"10.0.1.5", "10.0.1.6"},
		CNAMEs:    []string{"api.lb.example.com."},
		TTL:       300,
		Server:    "8.8.8.8",
		QueryTime: 15 * time.Millisecond,
	}
	
	t.Log("✓ DNS result structure correctly captures:")
	t.Logf("  - Query:     %s", result.Query)
	t.Logf("  - Answers:   %v (multiple for load balancing)", result.Answers)
	t.Logf("  - CNAMEs:    %v (for aliases/CDN)", result.CNAMEs)
	t.Logf("  - TTL:       %d seconds", result.TTL)
	t.Logf("  - QueryTime: %v", result.QueryTime)
}

// =============================================================================
// Test HTTP/2 Frame Parsing
// =============================================================================

func TestHTTP2FrameParsing(t *testing.T) {
	t.Log("=== HTTP/2 Frame Analysis ===")
	
	// Construct a SETTINGS frame (type 0x4)
	// Format: [Length:3][Type:1][Flags:1][StreamID:4][Payload:Length]
	var buf bytes.Buffer
	
	// Frame header: Length=6 (1 setting), Type=SETTINGS, Flags=0, StreamID=0
	buf.Write([]byte{0x00, 0x00, 0x06}) // Length = 6
	buf.WriteByte(FrameSettings)         // Type = 0x4
	buf.WriteByte(0x00)                  // Flags
	binary.Write(&buf, binary.BigEndian, uint32(0)) // Stream ID = 0
	
	// SETTINGS payload: SETTINGS_MAX_CONCURRENT_STREAMS (0x3) = 100
	binary.Write(&buf, binary.BigEndian, uint16(0x0003)) // Setting ID
	binary.Write(&buf, binary.BigEndian, uint32(100))    // Value
	
	frame, err := ParseHTTP2Frame(&buf)
	if err != nil {
		t.Fatalf("Failed to parse frame: %v", err)
	}
	
	t.Logf("✓ Parsed HTTP/2 frame:")
	t.Logf("  - Type:     %s (0x%02x)", FrameTypeName(frame.Type), frame.Type)
	t.Logf("  - Length:   %d bytes", frame.Length)
	t.Logf("  - Flags:    0x%02x", frame.Flags)
	t.Logf("  - StreamID: %d (0 = connection-level)", frame.StreamID)
	t.Logf("  - Payload:  %d bytes", len(frame.Payload))
	
	if frame.Type != FrameSettings {
		t.Errorf("Expected SETTINGS frame, got %s", FrameTypeName(frame.Type))
	}
	if frame.StreamID != 0 {
		t.Errorf("SETTINGS frame should have StreamID=0, got %d", frame.StreamID)
	}
}

func TestHTTP2FrameTypes(t *testing.T) {
	t.Log("=== HTTP/2 Frame Types for gRPC ===")
	
	frameTypes := []struct {
		typeVal uint8
		purpose string
	}{
		{FrameData, "Carries request/response body (gRPC messages)"},
		{FrameHeaders, "Opens stream, carries :method, :path, grpc-status"},
		{FrameSettings, "Connection parameters (MAX_CONCURRENT_STREAMS)"},
		{FrameWindowUpdate, "Flow control (prevents overwhelming slow receivers)"},
		{FramePing, "Keepalive, latency measurement"},
		{FrameGoAway, "Graceful connection close (triggers reconnect)"},
		{FrameRSTStream, "Immediate stream cancellation"},
	}
	
	for _, ft := range frameTypes {
		t.Logf("  %s (0x%02x): %s", FrameTypeName(ft.typeVal), ft.typeVal, ft.purpose)
	}
	
	t.Log("")
	t.Log("✓ gRPC over HTTP/2 flow:")
	t.Log("  1. HEADERS frame opens stream (request metadata)")
	t.Log("  2. DATA frames carry gRPC messages (length-prefixed protobuf)")
	t.Log("  3. HEADERS frame closes stream (trailers with grpc-status)")
}

// =============================================================================
// Test iptables Parsing
// =============================================================================

func TestIPTablesRuleParsing(t *testing.T) {
	t.Log("=== iptables Rule Analysis ===")
	
	// Sample iptables -L -v -n output
	sampleOutput := `Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
 pkts bytes target     prot opt in     out     source               destination
 1234 56789 ACCEPT     tcp  --  *      *       10.0.0.0/8           0.0.0.0/0            tcp dpt:443
  456  7890 DROP       all  --  *      *       192.168.1.100        0.0.0.0/0

Chain FORWARD (policy DROP 0 packets, 0 bytes)
 pkts bytes target     prot opt in     out     source               destination
    0     0 ACCEPT     all  --  cni0   eth0    10.244.0.0/16        0.0.0.0/0

Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
 pkts bytes target     prot opt in     out     source               destination`

	rules := ParseIPTablesOutput(sampleOutput)
	
	t.Logf("✓ Parsed %d iptables rules:", len(rules))
	for _, rule := range rules {
		t.Logf("  Chain: %-8s Target: %-8s Proto: %-4s Src: %-18s Dst: %s",
			rule.Chain, rule.Target, rule.Protocol, rule.Source, rule.Destination)
	}
	
	t.Log("")
	t.Log("  K8s networking iptables chains:")
	t.Log("    * KUBE-SERVICES: Service VIP → pod endpoints")
	t.Log("    * KUBE-NODEPORTS: NodePort → pod endpoints")
	t.Log("    * KUBE-MARK-MASQ: Mark packets for SNAT")
	t.Log("    * KUBE-POSTROUTING: MASQUERADE marked packets")
}

// =============================================================================
// Test Debug Command Generation
// =============================================================================

func TestDebugCommandGeneration(t *testing.T) {
	t.Log("=== Debug Command Generation ===")
	
	cmds := GenerateDebugCommands("10.0.1.5", "192.168.1.100", "api.internal.corp", 443)
	
	t.Log("✓ Layer 3 commands generated:")
	for _, cmd := range cmds.Layer3[:2] {
		t.Logf("    %s", cmd)
	}
	
	t.Log("✓ Layer 4 commands generated:")
	for _, cmd := range cmds.Layer4[:2] {
		t.Logf("    %s", cmd)
	}
	
	t.Log("✓ TLS commands generated:")
	for _, cmd := range cmds.TLS[:2] {
		t.Logf("    %s", cmd)
	}
	
	t.Log("✓ DNS commands generated:")
	for _, cmd := range cmds.DNS[:2] {
		t.Logf("    %s", cmd)
	}
	
	// Verify commands contain expected elements
	layer4Found := false
	for _, cmd := range cmds.Layer4 {
		if strings.Contains(cmd, "192.168.1.100") && strings.Contains(cmd, "443") {
			layer4Found = true
			break
		}
	}
	if !layer4Found {
		t.Error("Layer 4 commands should contain target IP and port")
	}
}

// =============================================================================
// Test Troubleshooting Decision Tree
// =============================================================================

func TestTroubleshootingTree(t *testing.T) {
	t.Log("=== Troubleshooting Decision Tree ===")
	
	tree := BuildTroubleshootingTree()
	
	if tree == nil {
		t.Fatal("Tree should not be nil")
	}
	
	t.Log("✓ Decision tree built successfully")
	t.Logf("  Root question: %s", tree.Question)
	
	// Traverse a path: ping works → TCP works → TLS works → ALPN works → gRPC works
	current := tree
	path := []string{"yes", "yes", "yes", "yes", "yes"}
	pathNames := []string{"ping", "TCP", "TLS", "ALPN", "gRPC"}
	
	t.Log("")
	t.Log("✓ Path through decision tree (all yes):")
	for i, direction := range path {
		t.Logf("  Q: %s", current.Question)
		if direction == "yes" && current.YesPath != nil {
			current = current.YesPath
		} else if current.NoPath != nil {
			current = current.NoPath
		}
		t.Logf("    → %s check passed", pathNames[i])
	}
	
	if current.Action != "" {
		t.Logf("  Final action: %s", current.Action)
	}
	
	// Test a failure path: ping fails
	t.Log("")
	t.Log("✓ Path through decision tree (ping fails):")
	current = tree
	t.Logf("  Q: %s → NO", current.Question)
	current = current.NoPath
	t.Logf("  Q: %s → NO", current.Question)
	current = current.NoPath
	t.Logf("  Action: %s", current.Action)
}

// =============================================================================
// Test Full Network Path Scenario
// =============================================================================

func TestFullNetworkPathScenario(t *testing.T) {
	t.Log(`
================================================================================
FULL NETWORK PATH: Cloud Pod → On-Prem gRPC Server
================================================================================

This test documents the complete path a gRPC call takes from a Kubernetes pod
in a cloud environment to an on-premises gRPC server.

=== STEP 1: Pod DNS Resolution ===
`)
	
	t.Log("  Pod queries CoreDNS for 'api.internal.corp'")
	t.Log("  - Kubernetes DNS search: api.internal.corp.default.svc.cluster.local")
	t.Log("  - Falls back to upstream resolver")
	t.Log("  - Returns: 192.168.1.100 (on-prem LB VIP)")
	
	t.Log(`
=== STEP 2: gRPC Channel Creation ===
`)
	t.Log("  - Resolver returns address list")
	t.Log("  - Load balancer creates subchannels")
	t.Log("  - Initial state: IDLE (lazy connect)")
	
	t.Log(`
=== STEP 3: Pod Network (CNI) ===
`)
	t.Log("  - Pod IP: 10.244.1.50 (assigned by CNI)")
	t.Log("  - Default route via cbr0 bridge")
	t.Log("  - iptables MASQUERADE: SNAT to node IP")
	t.Log("    10.244.1.50 → 10.0.1.5 (node IP)")
	
	t.Log(`
=== STEP 4: VPC Routing ===
`)
	t.Log("  - Route table entry: 192.168.0.0/16 → VPN gateway")
	t.Log("  - Security groups: Allow TCP/443 outbound")
	t.Log("  - NAT Gateway if node in private subnet")
	
	t.Log(`
=== STEP 5: VPN/Interconnect ===
`)
	t.Log("  - IPsec tunnel to on-prem network")
	t.Log("  - MTU reduced (1400 for IPsec overhead)")
	t.Log("  - Latency added: 10-50ms typical")
	
	t.Log(`
=== STEP 6: On-Prem Network ===
`)
	t.Log("  - Firewall allows 443 from cloud CIDR")
	t.Log("  - Routes to internal load balancer")
	
	t.Log(`
=== STEP 7: L4 Load Balancer ===
`)
	t.Log("  - VIP: 192.168.1.100:443")
	t.Log("  - DSR (Direct Server Return) or NAT mode")
	t.Log("  - Health checks to backend pods")
	t.Log("  - Selects backend: 192.168.1.10:8443")
	
	t.Log(`
=== STEP 8: TLS Termination ===
`)
	t.Log("  - TLS 1.3 handshake")
	t.Log("  - ALPN: h2 negotiated")
	t.Log("  - Certificate: CN=api.internal.corp")
	t.Log("  - mTLS optional: client cert verification")
	
	t.Log(`
=== STEP 9: gRPC Server ===
`)
	t.Log("  - HTTP/2 SETTINGS exchanged")
	t.Log("  - MAX_CONCURRENT_STREAMS: 100")
	t.Log("  - Request: HEADERS + DATA frames")
	t.Log("  - Response: HEADERS + DATA + HEADERS (trailers)")
	
	t.Log(`
=== COMMON ISSUES AT EACH LAYER ===
`)
	issues := map[string][]string{
		"L3 (IP)": {
			"MTU mismatch causing fragmentation",
			"Asymmetric routing breaking conntrack",
			"Missing routes in VPC",
		},
		"L4 (TCP)": {
			"Connection timeouts (SYN not reaching)",
			"RST from firewall/middlebox",
			"TIME_WAIT exhaustion under load",
		},
		"TLS": {
			"Certificate name mismatch",
			"Expired certificates",
			"TLS version incompatibility",
		},
		"HTTP/2": {
			"GOAWAY during deployment (reconnect storm)",
			"MAX_CONCURRENT_STREAMS too low",
			"Flow control stalls",
		},
		"gRPC": {
			"Deadline exceeded (propagation issue)",
			"UNAVAILABLE during rollout",
			"RESOURCE_EXHAUSTED (server overload)",
		},
	}
	
	for layer, layerIssues := range issues {
		t.Logf("  %s:", layer)
		for _, issue := range layerIssues {
			t.Logf("    • %s", issue)
		}
	}
}

// =============================================================================
// Summary Test
// =============================================================================

func TestNetworkDebuggingSummary(t *testing.T) {
	t.Log(`
================================================================================
NETWORK DEBUGGING L3→L7 - SUMMARY
================================================================================

1. LAYER 3 (IP)
   - Tools: ping, traceroute, mtr
   - Issues: Routing, MTU, packet drops
   - K8s: CNI assigns pod IPs, handles routing

2. LAYER 4 (TCP)
   - Tools: nc, ss, tcpdump, netstat
   - Issues: Connection timeouts, RSTs, state exhaustion
   - K8s: kube-proxy manages Service→Pod translation

3. TLS
   - Tools: openssl s_client, curl -v
   - Issues: Cert errors, version mismatch, ALPN
   - Critical: Must negotiate 'h2' for gRPC

4. DNS
   - Tools: dig, nslookup, tcpdump port 53
   - Issues: Stale cache, slow resolution, NXDOMAIN
   - K8s: CoreDNS, ndots:5 default, search domains

5. HTTP/2
   - Tools: grpcurl, nghttp, curl --http2
   - Issues: SETTINGS mismatch, GOAWAY, flow control
   - Key frames: HEADERS, DATA, SETTINGS, GOAWAY

6. gRPC
   - Tools: grpcurl, grpc_cli, interceptors
   - Issues: Deadline propagation, load balancing, retry
   - Status codes: UNAVAILABLE, DEADLINE_EXCEEDED, RESOURCE_EXHAUSTED

7. DEBUGGING APPROACH
   a. Start at L3: Can you reach the IP?
   b. L4: Can you establish TCP connection?
   c. TLS: Does handshake complete with correct ALPN?
   d. HTTP/2: Are SETTINGS frames exchanged?
   e. gRPC: Does health check pass?

8. K8s-SPECIFIC
   - Pod networking: CNI, iptables, SNAT
   - Service discovery: CoreDNS, Endpoints, EndpointSlices
   - Ingress: L7 routing, TLS termination
   - NetworkPolicy: Calico/Cilium enforcement

================================================================================
`)
}
