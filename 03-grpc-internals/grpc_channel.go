// Package main demonstrates gRPC client internals: channels, subchannels,
// load balancing, and interceptors.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// =============================================================================
// SECTION 1: Connectivity States
// =============================================================================

// ConnectivityState represents the state of a channel or subchannel
type ConnectivityState int

const (
	Idle ConnectivityState = iota
	Connecting
	Ready
	TransientFailure
	Shutdown
)

func (s ConnectivityState) String() string {
	return [...]string{"IDLE", "CONNECTING", "READY", "TRANSIENT_FAILURE", "SHUTDOWN"}[s]
}

// =============================================================================
// SECTION 2: Address and Resolver
// =============================================================================

// Address represents a resolved backend address
type Address struct {
	Addr       string            // host:port
	ServerName string            // For TLS verification
	Attributes map[string]string // Arbitrary metadata
}

// Resolver resolves a target to a list of addresses
type Resolver interface {
	Resolve(target string) ([]Address, error)
	// ResolveNow triggers an immediate re-resolution
	ResolveNow()
}

// DNSResolver implements DNS-based name resolution
type DNSResolver struct {
	target      string
	addresses   []Address
	updateCh    chan []Address
	mu          sync.Mutex
	lastResolve time.Time
	minInterval time.Duration
}

// NewDNSResolver creates a DNS resolver for a target
func NewDNSResolver(target string) *DNSResolver {
	return &DNSResolver{
		target:      target,
		updateCh:    make(chan []Address, 1),
		minInterval: 30 * time.Second,
	}
}

// Resolve performs DNS lookup
func (r *DNSResolver) Resolve(target string) ([]Address, error) {
	// In reality, this does net.LookupHost and parses SRV records
	// Simplified for demonstration
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastResolve = time.Now()

	// Simulated DNS response
	return []Address{
		{Addr: "10.0.1.5:443", ServerName: target},
		{Addr: "10.0.1.6:443", ServerName: target},
		{Addr: "10.0.1.7:443", ServerName: target},
	}, nil
}

func (r *DNSResolver) ResolveNow() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if time.Since(r.lastResolve) < r.minInterval {
		return // Rate limit re-resolution
	}

	// Trigger background re-resolution
	go func() {
		addrs, _ := r.Resolve(r.target)
		select {
		case r.updateCh <- addrs:
		default:
		}
	}()
}

// =============================================================================
// SECTION 3: Subchannel - A Connection to a Single Backend
// =============================================================================

// Subchannel represents a connection to a single backend address
type Subchannel struct {
	addr  Address
	state ConnectivityState

	// Connection tracking
	connectAttempts int
	lastConnectTime time.Time

	// HTTP/2 transport state (simplified)
	activeStreams int32
	maxStreams    int32

	// Backoff for reconnection
	backoff *ExponentialBackoff

	mu       sync.Mutex
	stateCh  chan ConnectivityState
	shutdown bool
}

// ExponentialBackoff implements reconnection backoff
type ExponentialBackoff struct {
	baseDelay  time.Duration
	maxDelay   time.Duration
	multiplier float64
	attempts   int
	jitter     float64
}

// NewExponentialBackoff creates a backoff calculator
func NewExponentialBackoff() *ExponentialBackoff {
	return &ExponentialBackoff{
		baseDelay:  1 * time.Second,
		maxDelay:   120 * time.Second,
		multiplier: 1.6,
		jitter:     0.2,
	}
}

// NextBackoff returns the next backoff duration
func (b *ExponentialBackoff) NextBackoff() time.Duration {
	delay := float64(b.baseDelay) * pow(b.multiplier, float64(b.attempts))
	if delay > float64(b.maxDelay) {
		delay = float64(b.maxDelay)
	}

	// Add jitter
	jitterRange := delay * b.jitter
	delay = delay + (rand.Float64()*2-1)*jitterRange

	b.attempts++
	return time.Duration(delay)
}

func (b *ExponentialBackoff) Reset() {
	b.attempts = 0
}

func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// NewSubchannel creates a subchannel for an address
func NewSubchannel(addr Address) *Subchannel {
	return &Subchannel{
		addr:       addr,
		state:      Idle,
		maxStreams: 100, // HTTP/2 default
		backoff:    NewExponentialBackoff(),
		stateCh:    make(chan ConnectivityState, 10),
	}
}

// Connect initiates connection to the backend
func (sc *Subchannel) Connect(ctx context.Context) error {
	sc.mu.Lock()
	if sc.shutdown {
		sc.mu.Unlock()
		return fmt.Errorf("subchannel shutdown")
	}
	sc.state = Connecting
	sc.stateCh <- Connecting
	sc.mu.Unlock()

	// Simulate connection attempt
	sc.connectAttempts++
	sc.lastConnectTime = time.Now()

	// Simulated connection logic
	// In reality: TCP handshake, TLS handshake, HTTP/2 SETTINGS exchange
	select {
	case <-ctx.Done():
		sc.transitionTo(TransientFailure)
		return ctx.Err()
	case <-time.After(50 * time.Millisecond): // Simulated handshake time
	}

	// 80% success rate for simulation
	if rand.Float32() < 0.8 {
		sc.transitionTo(Ready)
		sc.backoff.Reset()
		return nil
	}

	sc.transitionTo(TransientFailure)
	return fmt.Errorf("connection failed")
}

func (sc *Subchannel) transitionTo(state ConnectivityState) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.state = state
	select {
	case sc.stateCh <- state:
	default:
	}
}

// State returns current connectivity state
func (sc *Subchannel) State() ConnectivityState {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.state
}

// CanAcceptStream checks if subchannel can handle another stream
func (sc *Subchannel) CanAcceptStream() bool {
	return sc.State() == Ready &&
		atomic.LoadInt32(&sc.activeStreams) < sc.maxStreams
}

// AcquireStream reserves a stream slot
func (sc *Subchannel) AcquireStream() bool {
	for {
		current := atomic.LoadInt32(&sc.activeStreams)
		if current >= sc.maxStreams {
			return false
		}
		if atomic.CompareAndSwapInt32(&sc.activeStreams, current, current+1) {
			return true
		}
	}
}

// ReleaseStream releases a stream slot
func (sc *Subchannel) ReleaseStream() {
	atomic.AddInt32(&sc.activeStreams, -1)
}

// =============================================================================
// SECTION 4: Load Balancer Interface
// =============================================================================

// Picker selects a subchannel for an RPC
type Picker interface {
	Pick() (*Subchannel, error)
}

// LoadBalancer manages subchannels and provides pickers
type LoadBalancer interface {
	// UpdateAddresses is called when resolver provides new addresses
	UpdateAddresses([]Address)
	// GetPicker returns the current picker
	GetPicker() Picker
}

// RoundRobinBalancer implements round-robin load balancing
type RoundRobinBalancer struct {
	mu          sync.Mutex
	subchannels []*Subchannel
	picker      *RoundRobinPicker
}

// NewRoundRobinBalancer creates a round-robin load balancer
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

// UpdateAddresses creates/removes subchannels based on new addresses
func (b *RoundRobinBalancer) UpdateAddresses(addrs []Address) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Build address set for comparison
	newAddrs := make(map[string]Address)
	for _, addr := range addrs {
		newAddrs[addr.Addr] = addr
	}

	// Keep existing subchannels that are still in the list
	var kept []*Subchannel
	for _, sc := range b.subchannels {
		if _, ok := newAddrs[sc.addr.Addr]; ok {
			kept = append(kept, sc)
			delete(newAddrs, sc.addr.Addr)
		}
		// Else: subchannel's address removed, let it be garbage collected
	}

	// Create new subchannels for new addresses
	for _, addr := range newAddrs {
		sc := NewSubchannel(addr)
		kept = append(kept, sc)
		// Start connecting in background
		go sc.Connect(context.Background())
	}

	b.subchannels = kept
	b.picker = &RoundRobinPicker{subchannels: kept}
}

func (b *RoundRobinBalancer) GetPicker() Picker {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.picker
}

// RoundRobinPicker implements round-robin picking
type RoundRobinPicker struct {
	subchannels []*Subchannel
	next        uint64
}

// Pick returns the next ready subchannel
func (p *RoundRobinPicker) Pick() (*Subchannel, error) {
	if len(p.subchannels) == 0 {
		return nil, fmt.Errorf("no subchannels available")
	}

	// Find next ready subchannel (round-robin with skip)
	for i := 0; i < len(p.subchannels); i++ {
		idx := atomic.AddUint64(&p.next, 1) % uint64(len(p.subchannels))
		sc := p.subchannels[idx]
		if sc.CanAcceptStream() {
			return sc, nil
		}
	}

	return nil, fmt.Errorf("no ready subchannels")
}

// =============================================================================
// SECTION 5: Interceptors
// =============================================================================

// UnaryInvoker is the handler for a unary RPC
type UnaryInvoker func(ctx context.Context, method string, req, reply interface{}) error

// UnaryInterceptor intercepts unary RPCs
type UnaryInterceptor func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker) error

// StreamInvoker is the handler for a streaming RPC
type StreamInvoker func(ctx context.Context, method string) (interface{}, error)

// StreamInterceptor intercepts streaming RPCs
type StreamInterceptor func(ctx context.Context, method string, invoker StreamInvoker) (interface{}, error)

// ChainUnaryInterceptors chains multiple interceptors
func ChainUnaryInterceptors(interceptors ...UnaryInterceptor) UnaryInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker) error {
		chain := invoker
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(ctx context.Context, method string, req, reply interface{}) error {
				return interceptor(ctx, method, req, reply, next)
			}
		}
		return chain(ctx, method, req, reply)
	}
}

// LoggingInterceptor logs RPC calls
func LoggingInterceptor(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker) error {
	start := time.Now()
	err := invoker(ctx, method, req, reply)
	duration := time.Since(start)

	status := "OK"
	if err != nil {
		status = err.Error()
	}

	fmt.Printf("[gRPC] %s | %v | %s\n", method, duration, status)
	return err
}

// RetryInterceptor implements automatic retry
func RetryInterceptor(maxRetries int, backoff time.Duration) UnaryInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, invoker UnaryInvoker) error {
		var lastErr error
		for i := 0; i <= maxRetries; i++ {
			err := invoker(ctx, method, req, reply)
			if err == nil {
				return nil
			}
			lastErr = err

			// Check if error is retryable
			// In real gRPC, check status codes: UNAVAILABLE, RESOURCE_EXHAUSTED, etc.
			if i < maxRetries {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(backoff * time.Duration(i+1)):
				}
			}
		}
		return lastErr
	}
}

// =============================================================================
// SECTION 6: Channel - The Main Client Connection
// =============================================================================

// Channel represents a logical connection to a gRPC service
type Channel struct {
	target   string
	resolver Resolver
	balancer LoadBalancer

	// Interceptors
	unaryInterceptor  UnaryInterceptor
	streamInterceptor StreamInterceptor

	// State
	state    ConnectivityState
	mu       sync.Mutex
	stateCh  chan ConnectivityState
	shutdown bool

	// Configuration
	defaultTimeout time.Duration
	waitForReady   bool
}

// DialOption configures Channel creation
type DialOption func(*Channel)

// WithLoadBalancer sets the load balancer
func WithLoadBalancer(lb LoadBalancer) DialOption {
	return func(c *Channel) {
		c.balancer = lb
	}
}

// WithUnaryInterceptor adds a unary interceptor
func WithUnaryInterceptor(i UnaryInterceptor) DialOption {
	return func(c *Channel) {
		if c.unaryInterceptor != nil {
			c.unaryInterceptor = ChainUnaryInterceptors(c.unaryInterceptor, i)
		} else {
			c.unaryInterceptor = i
		}
	}
}

// WithDefaultTimeout sets the default RPC timeout
func WithDefaultTimeout(d time.Duration) DialOption {
	return func(c *Channel) {
		c.defaultTimeout = d
	}
}

// Dial creates a new Channel to the target
func Dial(target string, opts ...DialOption) (*Channel, error) {
	c := &Channel{
		target:         target,
		resolver:       NewDNSResolver(target),
		balancer:       NewRoundRobinBalancer(),
		state:          Idle,
		stateCh:        make(chan ConnectivityState, 10),
		defaultTimeout: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Initial resolution
	addrs, err := c.resolver.Resolve(target)
	if err != nil {
		return nil, fmt.Errorf("initial resolution failed: %w", err)
	}
	c.balancer.UpdateAddresses(addrs)

	// Start background resolver updates
	go c.watchResolver()

	return c, nil
}

func (c *Channel) watchResolver() {
	// In real implementation, watch for DNS changes
	// and call c.balancer.UpdateAddresses()
}

// Invoke performs a unary RPC
func (c *Channel) Invoke(ctx context.Context, method string, req, reply interface{}) error {
	// Apply default timeout if none set
	if _, ok := ctx.Deadline(); !ok && c.defaultTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.defaultTimeout)
		defer cancel()
	}

	// Create the actual invoker
	invoker := func(ctx context.Context, method string, req, reply interface{}) error {
		return c.invoke(ctx, method, req, reply)
	}

	// Apply interceptors if configured
	if c.unaryInterceptor != nil {
		return c.unaryInterceptor(ctx, method, req, reply, invoker)
	}

	return invoker(ctx, method, req, reply)
}

func (c *Channel) invoke(ctx context.Context, method string, req, reply interface{}) error {
	// Pick a subchannel
	picker := c.balancer.GetPicker()
	sc, err := picker.Pick()
	if err != nil {
		return fmt.Errorf("pick failed: %w", err)
	}

	// Acquire a stream slot
	if !sc.AcquireStream() {
		return fmt.Errorf("subchannel at max streams")
	}
	defer sc.ReleaseStream()

	// In reality: serialize request, send HTTP/2 DATA frames,
	// wait for response headers and DATA frames, deserialize response

	// Simulated RPC
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond): // Simulated latency
		return nil
	}
}

// Close shuts down the channel
func (c *Channel) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown = true
	c.state = Shutdown
	return nil
}

// =============================================================================
// SECTION 7: Demo
// =============================================================================

func main() {
	fmt.Println("Run 'go test -v' to see gRPC internals in action")
}
