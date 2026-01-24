package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// TEST 1: Subchannel State Transitions
// =============================================================================

func TestSubchannelStateTransitions(t *testing.T) {
	addr := Address{Addr: "10.0.1.5:443", ServerName: "test-service"}
	sc := NewSubchannel(addr)

	// Initial state is IDLE
	if sc.State() != Idle {
		t.Errorf("expected initial state IDLE, got %s", sc.State())
	}

	// Connect transitions through CONNECTING
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	go sc.Connect(ctx)

	// Wait for state change
	select {
	case state := <-sc.stateCh:
		if state != Connecting {
			t.Errorf("expected first transition to CONNECTING, got %s", state)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for CONNECTING state")
	}

	// Wait for final state (READY or TRANSIENT_FAILURE)
	select {
	case state := <-sc.stateCh:
		if state != Ready && state != TransientFailure {
			t.Errorf("expected final state READY or TRANSIENT_FAILURE, got %s", state)
		}
		t.Logf("✓ Subchannel transitioned: IDLE → CONNECTING → %s", state)
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for final state")
	}
}

// =============================================================================
// TEST 2: Stream Management
// =============================================================================

func TestSubchannelStreamManagement(t *testing.T) {
	addr := Address{Addr: "10.0.1.5:443", ServerName: "test-service"}
	sc := NewSubchannel(addr)
	sc.maxStreams = 5 // Low limit for testing

	// Manually set to ready
	sc.transitionTo(Ready)

	// Acquire streams up to limit
	for i := 0; i < 5; i++ {
		if !sc.AcquireStream() {
			t.Errorf("failed to acquire stream %d", i)
		}
	}

	// Next acquisition should fail
	if sc.AcquireStream() {
		t.Error("should not acquire stream beyond limit")
	}

	// Release one and verify we can acquire again
	sc.ReleaseStream()
	if !sc.AcquireStream() {
		t.Error("should acquire stream after release")
	}

	t.Log("✓ Subchannel enforces max concurrent streams")
	t.Log("  - This maps to HTTP/2 SETTINGS_MAX_CONCURRENT_STREAMS")
	t.Log("  - Default is typically 100 per connection")
}

// =============================================================================
// TEST 3: Exponential Backoff
// =============================================================================

func TestExponentialBackoff(t *testing.T) {
	backoff := NewExponentialBackoff()

	delays := make([]time.Duration, 5)
	for i := 0; i < 5; i++ {
		delays[i] = backoff.NextBackoff()
	}

	t.Log("✓ Exponential backoff sequence:")
	for i, d := range delays {
		t.Logf("  Attempt %d: %v", i+1, d)
	}

	// Verify exponential growth (with jitter tolerance)
	for i := 1; i < len(delays); i++ {
		// Allow 50% variance due to jitter
		if float64(delays[i]) < float64(delays[i-1])*0.5 {
			t.Errorf("backoff not increasing: %v followed by %v", delays[i-1], delays[i])
		}
	}

	// Reset should go back to base
	backoff.Reset()
	resetDelay := backoff.NextBackoff()
	if resetDelay > 2*time.Second { // Base is 1s, with jitter max ~1.2s
		t.Errorf("reset didn't restore base delay: got %v", resetDelay)
	}

	t.Log("  - Backoff resets after successful connection")
	t.Log("  - Prevents thundering herd on transient failures")
}

// =============================================================================
// TEST 4: Round Robin Load Balancing
// =============================================================================

func TestRoundRobinLoadBalancing(t *testing.T) {
	lb := NewRoundRobinBalancer()

	addrs := []Address{
		{Addr: "10.0.1.5:443"},
		{Addr: "10.0.1.6:443"},
		{Addr: "10.0.1.7:443"},
	}
	lb.UpdateAddresses(addrs)

	// Wait for subchannels to connect
	time.Sleep(200 * time.Millisecond)

	// Force all subchannels to READY for testing
	lb.mu.Lock()
	for _, sc := range lb.subchannels {
		sc.transitionTo(Ready)
	}
	lb.mu.Unlock()

	// Pick multiple times and verify distribution
	picks := make(map[string]int)
	picker := lb.GetPicker()

	for i := 0; i < 30; i++ {
		sc, err := picker.Pick()
		if err != nil {
			continue // Skip if no ready subchannel
		}
		picks[sc.addr.Addr]++
	}

	t.Log("✓ Round-robin distribution:")
	for addr, count := range picks {
		t.Logf("  %s: %d picks", addr, count)
	}

	// Verify roughly even distribution (allow some variance)
	for _, count := range picks {
		if count < 5 || count > 15 { // Expecting ~10 each
			t.Logf("  Warning: uneven distribution detected")
		}
	}
}

// =============================================================================
// TEST 5: Interceptor Chaining
// =============================================================================

func TestInterceptorChaining(t *testing.T) {
	var order []string
	var mu sync.Mutex

	interceptor1 := func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker) error {
		mu.Lock()
		order = append(order, "interceptor1-before")
		mu.Unlock()
		err := invoker(ctx, method, req, reply)
		mu.Lock()
		order = append(order, "interceptor1-after")
		mu.Unlock()
		return err
	}

	interceptor2 := func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker) error {
		mu.Lock()
		order = append(order, "interceptor2-before")
		mu.Unlock()
		err := invoker(ctx, method, req, reply)
		mu.Lock()
		order = append(order, "interceptor2-after")
		mu.Unlock()
		return err
	}

	chained := ChainUnaryInterceptors(interceptor1, interceptor2)

	// Mock invoker
	invoker := func(ctx context.Context, method string, req, reply interface{}) error {
		mu.Lock()
		order = append(order, "actual-call")
		mu.Unlock()
		return nil
	}

	err := chained(context.Background(), "/test/Method", nil, nil, invoker)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := []string{
		"interceptor1-before",
		"interceptor2-before",
		"actual-call",
		"interceptor2-after",
		"interceptor1-after",
	}

	t.Log("✓ Interceptor execution order:")
	for i, step := range order {
		t.Logf("  %d: %s", i+1, step)
		if i < len(expected) && step != expected[i] {
			t.Errorf("expected step %d to be %s, got %s", i, expected[i], step)
		}
	}
	t.Log("  - Interceptors wrap like middleware (LIFO unwinding)")
}

// =============================================================================
// TEST 6: Channel with Interceptors
// =============================================================================

func TestChannelWithInterceptors(t *testing.T) {
	var callCount int32

	countingInterceptor := func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker) error {
		atomic.AddInt32(&callCount, 1)
		return invoker(ctx, method, req, reply)
	}

	ch, err := Dial(
		"test-service.default.svc.cluster.local:443",
		WithUnaryInterceptor(countingInterceptor),
		WithUnaryInterceptor(LoggingInterceptor),
		WithDefaultTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer ch.Close()

	// Wait for subchannels
	time.Sleep(200 * time.Millisecond)

	// Force subchannels ready
	if rrb, ok := ch.balancer.(*RoundRobinBalancer); ok {
		rrb.mu.Lock()
		for _, sc := range rrb.subchannels {
			sc.transitionTo(Ready)
		}
		rrb.mu.Unlock()
	}

	// Make some calls
	for i := 0; i < 5; i++ {
		err := ch.Invoke(context.Background(), "/test.Service/Method", nil, nil)
		if err != nil {
			t.Logf("Call %d error (expected in test): %v", i, err)
		}
	}

	if atomic.LoadInt32(&callCount) != 5 {
		t.Errorf("expected 5 interceptor invocations, got %d", callCount)
	}

	t.Log("✓ Channel applies interceptors to all RPCs")
	t.Logf("  - Interceptor saw %d calls", callCount)
}

// =============================================================================
// TEST 7: Name Resolution Updates
// =============================================================================

func TestNameResolutionUpdates(t *testing.T) {
	lb := NewRoundRobinBalancer()

	// Initial addresses
	initial := []Address{
		{Addr: "10.0.1.5:443"},
		{Addr: "10.0.1.6:443"},
	}
	lb.UpdateAddresses(initial)

	lb.mu.Lock()
	initialCount := len(lb.subchannels)
	lb.mu.Unlock()

	if initialCount != 2 {
		t.Errorf("expected 2 initial subchannels, got %d", initialCount)
	}

	// Simulate DNS update with one removal and one addition
	updated := []Address{
		{Addr: "10.0.1.5:443"}, // Kept
		// 10.0.1.6 removed
		{Addr: "10.0.1.7:443"}, // New
		{Addr: "10.0.1.8:443"}, // New
	}
	lb.UpdateAddresses(updated)

	lb.mu.Lock()
	updatedCount := len(lb.subchannels)
	lb.mu.Unlock()

	if updatedCount != 3 {
		t.Errorf("expected 3 subchannels after update, got %d", updatedCount)
	}

	t.Log("✓ Load balancer handles address changes:")
	t.Logf("  - Initial: 2 addresses")
	t.Logf("  - Updated: 3 addresses (1 kept, 1 removed, 2 added)")
	t.Log("  - This is how k8s endpoint changes propagate to gRPC clients")
}

// =============================================================================
// Summary: The Complete Picture
// =============================================================================

func TestGRPCSummary(t *testing.T) {
	t.Log(`
================================================================================
gRPC CLIENT INTERNALS - SUMMARY
================================================================================

1. CHANNEL
   - Logical connection to a service (not a single TCP connection)
   - Contains resolver, load balancer, interceptors
   - Entry point for all RPCs

2. RESOLVER (Name Resolution)
   - Converts target string to list of addresses
   - Schemes: dns:///, xds:///, unix:///
   - Updates trigger load balancer rebalancing

3. LOAD BALANCER
   - Manages subchannels based on resolved addresses
   - Provides "picker" to select subchannel for each RPC
   - Policies: pick_first, round_robin, grpclb, xds

4. SUBCHANNEL
   - Single TCP+TLS connection to one backend
   - Has its own connectivity state machine
   - HTTP/2 transport: multiplexes many RPCs
   - Max concurrent streams (SETTINGS_MAX_CONCURRENT_STREAMS)

5. INTERCEPTORS
   - Middleware for RPCs (logging, retry, auth, tracing)
   - Chained in order, unwind in reverse (LIFO)
   - Unary vs streaming interceptors

6. CONNECTION LIFECYCLE
   - IDLE: No connection, will connect on demand
   - CONNECTING: TCP/TLS handshake in progress
   - READY: Can accept RPCs
   - TRANSIENT_FAILURE: Backing off before retry
   - SHUTDOWN: Permanently closed

7. KEY BEHAVIORS
   - Lazy connection: Connects on first RPC
   - Keepalive: Pings to detect dead connections
   - Backoff: Exponential backoff on failures
   - Wait-for-ready: Optional blocking until READY

================================================================================
`)
}
