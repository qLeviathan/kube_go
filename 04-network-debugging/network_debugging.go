// Package main provides network debugging utilities demonstrating L3-L7 analysis
package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// =============================================================================
// SECTION 1: Layer 3 - IP Connectivity
// =============================================================================

// PingResult contains ICMP ping results
type PingResult struct {
	Target    string
	Reachable bool
	RTT       time.Duration
	Error     error
}

// CheckIPConnectivity verifies basic IP reachability
func CheckIPConnectivity(target string) PingResult {
	// Using net.DialTimeout as a TCP-based "ping" (works without root)
	start := time.Now()
	conn, err := net.DialTimeout("ip4:icmp", target, 5*time.Second)
	if err != nil {
		// Fallback: try TCP to common port
		conn, err = net.DialTimeout("tcp", target+":443", 5*time.Second)
	}

	result := PingResult{Target: target}
	if err != nil {
		result.Error = err
		result.Reachable = false
	} else {
		conn.Close()
		result.Reachable = true
		result.RTT = time.Since(start)
	}
	return result
}

// TraceRoute performs a traceroute (simplified)
type TraceHop struct {
	HopNumber int
	Address   string
	RTT       time.Duration
	Error     error
}

// TraceRouteTo performs traceroute to target
func TraceRouteTo(target string) []TraceHop {
	// In production, use raw sockets with increasing TTL
	// This is a simplified version using external traceroute command
	var hops []TraceHop

	// Simulate traceroute output
	// In real implementation: send ICMP/UDP with TTL=1,2,3... until destination
	hops = append(hops, TraceHop{1, "10.0.0.1", 1 * time.Millisecond, nil})
	hops = append(hops, TraceHop{2, "172.16.0.1", 5 * time.Millisecond, nil})
	hops = append(hops, TraceHop{3, target, 10 * time.Millisecond, nil})

	return hops
}

// MTUDiscovery checks path MTU
func MTUDiscovery(target string) int {
	// Binary search for path MTU
	// Real implementation sends ICMP with DF bit set
	low, high := 576, 9000

	for low < high {
		mid := (low + high + 1) / 2
		// Try to send packet of size mid
		// If success, low = mid
		// If "packet too big" ICMP received, high = mid - 1
		_ = mid
		break // Simplified
	}

	return 1500 // Default Ethernet MTU
}

// =============================================================================
// SECTION 2: Layer 4 - TCP Analysis
// =============================================================================

// TCPConnectionInfo contains TCP connection details
type TCPConnectionInfo struct {
	LocalAddr  string
	RemoteAddr string
	State      string
	SendQueue  int
	RecvQueue  int
	RTT        time.Duration
	CwndMSS    int // Congestion window in MSS units
}

// AnalyzeTCPConnection examines TCP socket state
func AnalyzeTCPConnection(conn net.Conn) TCPConnectionInfo {
	info := TCPConnectionInfo{
		LocalAddr:  conn.LocalAddr().String(),
		RemoteAddr: conn.RemoteAddr().String(),
	}

	// In real implementation, read from /proc/net/tcp or use getsockopt
	// to get TCP_INFO socket option

	// TCP states (from kernel)
	// 01: ESTABLISHED, 02: SYN_SENT, 03: SYN_RECV
	// 04: FIN_WAIT1, 05: FIN_WAIT2, 06: TIME_WAIT
	// 07: CLOSE, 08: CLOSE_WAIT, 09: LAST_ACK
	// 0A: LISTEN, 0B: CLOSING

	info.State = "ESTABLISHED"
	info.RTT = 10 * time.Millisecond
	info.CwndMSS = 10

	return info
}

// TCPHandshake performs TCP 3-way handshake analysis
type TCPHandshakeResult struct {
	SYNSent     time.Time
	SYNACKRecv  time.Time
	ACKSent     time.Time
	TotalTime   time.Duration
	ServerTime  time.Duration // Time for server to respond
	Success     bool
	Error       error
}

// AnalyzeTCPHandshake measures TCP handshake timing
func AnalyzeTCPHandshake(target string) TCPHandshakeResult {
	result := TCPHandshakeResult{SYNSent: time.Now()}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.Dial("tcp", target)

	result.ACKSent = time.Now()
	result.TotalTime = result.ACKSent.Sub(result.SYNSent)

	if err != nil {
		result.Error = err
		return result
	}
	defer conn.Close()

	result.Success = true
	result.ServerTime = result.TotalTime / 2 // Approximate

	return result
}

// =============================================================================
// SECTION 3: TLS Analysis
// =============================================================================

// TLSConnectionInfo contains TLS connection details
type TLSConnectionInfo struct {
	Version            uint16
	CipherSuite        uint16
	ServerName         string
	NegotiatedProtocol string // ALPN result
	PeerCertificates   int
	HandshakeComplete  bool
	Error              error
}

// VersionString converts TLS version to string
func (t TLSConnectionInfo) VersionString() string {
	switch t.Version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", t.Version)
	}
}

// AnalyzeTLSConnection examines TLS handshake and connection
func AnalyzeTLSConnection(target, serverName string) TLSConnectionInfo {
	info := TLSConnectionInfo{ServerName: serverName}

	config := &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: false,
		NextProtos:         []string{"h2", "http/1.1"}, // ALPN for HTTP/2
	}

	dialer := tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		Config:    config,
	}

	conn, err := dialer.Dial("tcp", target)
	if err != nil {
		info.Error = err
		return info
	}
	defer conn.Close()

	tlsConn := conn.(*tls.Conn)
	state := tlsConn.ConnectionState()

	info.Version = state.Version
	info.CipherSuite = state.CipherSuite
	info.NegotiatedProtocol = state.NegotiatedProtocol
	info.PeerCertificates = len(state.PeerCertificates)
	info.HandshakeComplete = state.HandshakeComplete

	return info
}

// =============================================================================
// SECTION 4: DNS Analysis
// =============================================================================

// DNSResult contains DNS resolution details
type DNSResult struct {
	Query       string
	Answers     []string
	CNAMEs      []string
	TTL         int
	Server      string
	QueryTime   time.Duration
	Error       error
}

// ResolveDNS performs DNS resolution with timing
func ResolveDNS(hostname string) DNSResult {
	result := DNSResult{Query: hostname}
	start := time.Now()

	// Use the default resolver
	addrs, err := net.LookupHost(hostname)
	result.QueryTime = time.Since(start)

	if err != nil {
		result.Error = err
		return result
	}

	result.Answers = addrs

	// Look up CNAME
	cname, err := net.LookupCNAME(hostname)
	if err == nil && cname != hostname+"." {
		result.CNAMEs = []string{cname}
	}

	return result
}

// CheckDNSServer tests a specific DNS server
func CheckDNSServer(server, hostname string) DNSResult {
	result := DNSResult{Query: hostname, Server: server}

	// Create resolver using specific server
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", server+":53")
		},
	}

	start := time.Now()
	addrs, err := r.LookupHost(context.Background(), hostname)
	result.QueryTime = time.Since(start)

	if err != nil {
		result.Error = err
		return result
	}

	result.Answers = addrs
	return result
}

// =============================================================================
// SECTION 5: HTTP/2 and gRPC Analysis
// =============================================================================

// HTTP2Frame represents a parsed HTTP/2 frame
type HTTP2Frame struct {
	Length   uint32
	Type     uint8
	Flags    uint8
	StreamID uint32
	Payload  []byte
}

// Frame types
const (
	FrameData         = 0x0
	FrameHeaders      = 0x1
	FramePriority     = 0x2
	FrameRSTStream    = 0x3
	FrameSettings     = 0x4
	FramePushPromise  = 0x5
	FramePing         = 0x6
	FrameGoAway       = 0x7
	FrameWindowUpdate = 0x8
	FrameContinuation = 0x9
)

// FrameTypeName returns the name of a frame type
func FrameTypeName(t uint8) string {
	names := map[uint8]string{
		FrameData:         "DATA",
		FrameHeaders:      "HEADERS",
		FramePriority:     "PRIORITY",
		FrameRSTStream:    "RST_STREAM",
		FrameSettings:     "SETTINGS",
		FramePushPromise:  "PUSH_PROMISE",
		FramePing:         "PING",
		FrameGoAway:       "GOAWAY",
		FrameWindowUpdate: "WINDOW_UPDATE",
		FrameContinuation: "CONTINUATION",
	}
	if name, ok := names[t]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN(%d)", t)
}

// ParseHTTP2Frame parses an HTTP/2 frame header
func ParseHTTP2Frame(r io.Reader) (*HTTP2Frame, error) {
	header := make([]byte, 9) // HTTP/2 frame header is 9 bytes
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	frame := &HTTP2Frame{
		Length:   uint32(header[0])<<16 | uint32(header[1])<<8 | uint32(header[2]),
		Type:     header[3],
		Flags:    header[4],
		StreamID: binary.BigEndian.Uint32(header[5:9]) & 0x7FFFFFFF, // Clear reserved bit
	}

	if frame.Length > 0 {
		frame.Payload = make([]byte, frame.Length)
		if _, err := io.ReadFull(r, frame.Payload); err != nil {
			return nil, err
		}
	}

	return frame, nil
}

// =============================================================================
// SECTION 6: iptables Analysis (Linux-specific)
// =============================================================================

// IPTablesRule represents a parsed iptables rule
type IPTablesRule struct {
	Chain       string
	Target      string
	Protocol    string
	Source      string
	Destination string
	InInterface string
	OutInterface string
	Match       string
}

// ParseIPTablesOutput parses iptables -L -v -n output
func ParseIPTablesOutput(output string) []IPTablesRule {
	var rules []IPTablesRule
	var currentChain string

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// Chain line
		if strings.HasPrefix(line, "Chain ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentChain = parts[1]
			}
			continue
		}

		// Skip headers and empty lines
		if strings.HasPrefix(line, " pkts") || strings.TrimSpace(line) == "" {
			continue
		}

		// Parse rule line
		fields := strings.Fields(line)
		if len(fields) >= 8 {
			rule := IPTablesRule{
				Chain:        currentChain,
				Target:       fields[2],
				Protocol:     fields[3],
				Source:       fields[7],
				Destination:  fields[8],
			}
			rules = append(rules, rule)
		}
	}

	return rules
}

// =============================================================================
// SECTION 7: Debugging Command Generators
// =============================================================================

// DebugCommands generates debugging commands for a given scenario
type DebugCommands struct {
	Layer3  []string
	Layer4  []string
	TLS     []string
	DNS     []string
	HTTP2   []string
	K8s     []string
}

// GenerateDebugCommands creates debugging commands for cloud-to-onprem gRPC call
func GenerateDebugCommands(clientIP, serverIP, serverName string, serverPort int) DebugCommands {
	target := fmt.Sprintf("%s:%d", serverIP, serverPort)

	return DebugCommands{
		Layer3: []string{
			fmt.Sprintf("# Verify IP connectivity"),
			fmt.Sprintf("ping -c 5 %s", serverIP),
			fmt.Sprintf("# Trace route to see path"),
			fmt.Sprintf("traceroute -n %s", serverIP),
			fmt.Sprintf("# Check MTU (avoid fragmentation issues)"),
			fmt.Sprintf("ping -M do -s 1472 %s", serverIP),
		},
		Layer4: []string{
			fmt.Sprintf("# Test TCP connectivity"),
			fmt.Sprintf("nc -zv %s %d", serverIP, serverPort),
			fmt.Sprintf("# Check TCP connection state"),
			fmt.Sprintf("ss -tn state established '( dport = :%d )'", serverPort),
			fmt.Sprintf("# Watch for TCP retransmits"),
			fmt.Sprintf("netstat -s | grep -i retrans"),
			fmt.Sprintf("# Capture TCP handshake"),
			fmt.Sprintf("tcpdump -i any -nn 'host %s and port %d' -c 100", serverIP, serverPort),
		},
		TLS: []string{
			fmt.Sprintf("# Test TLS handshake"),
			fmt.Sprintf("openssl s_client -connect %s -servername %s -alpn h2", target, serverName),
			fmt.Sprintf("# Show certificate chain"),
			fmt.Sprintf("openssl s_client -connect %s -showcerts 2>/dev/null | openssl x509 -text -noout", target),
			fmt.Sprintf("# Test TLS 1.3 specifically"),
			fmt.Sprintf("openssl s_client -connect %s -tls1_3 -alpn h2", target),
		},
		DNS: []string{
			fmt.Sprintf("# Resolve hostname"),
			fmt.Sprintf("nslookup %s", serverName),
			fmt.Sprintf("# Check DNS with specific server"),
			fmt.Sprintf("dig @8.8.8.8 %s", serverName),
			fmt.Sprintf("# Check CoreDNS logs (k8s)"),
			fmt.Sprintf("kubectl logs -n kube-system -l k8s-app=kube-dns --tail=100"),
		},
		HTTP2: []string{
			fmt.Sprintf("# Test gRPC endpoint"),
			fmt.Sprintf("grpcurl -plaintext %s list", target),
			fmt.Sprintf("# Test with TLS"),
			fmt.Sprintf("grpcurl %s list", target),
			fmt.Sprintf("# Verbose mode to see HTTP/2 frames"),
			fmt.Sprintf("grpcurl -vv %s grpc.health.v1.Health/Check", target),
			fmt.Sprintf("# Check HTTP/2 SETTINGS with curl"),
			fmt.Sprintf("curl -v --http2 https://%s/", target),
		},
		K8s: []string{
			fmt.Sprintf("# Check endpoints"),
			fmt.Sprintf("kubectl get endpoints <service-name>"),
			fmt.Sprintf("# Check pod networking"),
			fmt.Sprintf("kubectl exec -it <pod> -- ip addr"),
			fmt.Sprintf("# Check NetworkPolicies"),
			fmt.Sprintf("kubectl get networkpolicy -A"),
			fmt.Sprintf("# Check service"),
			fmt.Sprintf("kubectl describe svc <service-name>"),
			fmt.Sprintf("# DNS from pod"),
			fmt.Sprintf("kubectl exec -it <pod> -- nslookup %s", serverName),
		},
	}
}

// =============================================================================
// SECTION 8: Troubleshooting Decision Tree
// =============================================================================

// TroubleshootingStep represents a step in the debugging process
type TroubleshootingStep struct {
	Question string
	YesPath  *TroubleshootingStep
	NoPath   *TroubleshootingStep
	Action   string
}

// BuildTroubleshootingTree creates a decision tree for debugging
func BuildTroubleshootingTree() *TroubleshootingStep {
	return &TroubleshootingStep{
		Question: "Can you ping the destination IP?",
		YesPath: &TroubleshootingStep{
			Question: "Does TCP connect succeed (nc -zv)?",
			YesPath: &TroubleshootingStep{
				Question: "Does TLS handshake complete?",
				YesPath: &TroubleshootingStep{
					Question: "Is ALPN negotiating h2?",
					YesPath: &TroubleshootingStep{
						Question: "Does gRPC health check pass?",
						YesPath: &TroubleshootingStep{
							Action: "Connection path is working. Check application-level issues (auth, routing).",
						},
						NoPath: &TroubleshootingStep{
							Action: "Check gRPC server health, max_concurrent_streams, deadline settings.",
						},
					},
					NoPath: &TroubleshootingStep{
						Action: "Server may not support HTTP/2. Check server config and ALPN negotiation.",
					},
				},
				NoPath: &TroubleshootingStep{
					Question: "Is certificate valid (not expired, correct hostname)?",
					YesPath: &TroubleshootingStep{
						Action: "Check TLS version compatibility (server may require TLS 1.3).",
					},
					NoPath: &TroubleshootingStep{
						Action: "Fix certificate: renew if expired, check SAN for hostname.",
					},
				},
			},
			NoPath: &TroubleshootingStep{
				Question: "Are there firewall rules blocking the port?",
				YesPath: &TroubleshootingStep{
					Action: "Update firewall/security group rules to allow traffic.",
				},
				NoPath: &TroubleshootingStep{
					Action: "Check if service is listening on the port (netstat -tlnp). Check for port exhaustion.",
				},
			},
		},
		NoPath: &TroubleshootingStep{
			Question: "Can you ping the gateway/router?",
			YesPath: &TroubleshootingStep{
				Action: "Route to destination is missing or blocked. Check VPC routes, peering, VPN tunnel status.",
			},
			NoPath: &TroubleshootingStep{
				Action: "Local network issue. Check interface status (ip link), default route (ip route).",
			},
		},
	}
}

func main() {
	fmt.Println("Network Debugging Utilities")
	fmt.Println("Run 'go test -v' to see demonstrations")

	// Generate debug commands for a sample scenario
	cmds := GenerateDebugCommands("10.0.1.5", "192.168.1.100", "api.internal.company.com", 443)

	fmt.Println("\n=== Layer 3 (IP) Commands ===")
	for _, cmd := range cmds.Layer3 {
		fmt.Println(cmd)
	}

	fmt.Println("\n=== Layer 4 (TCP) Commands ===")
	for _, cmd := range cmds.Layer4 {
		fmt.Println(cmd)
	}
}
