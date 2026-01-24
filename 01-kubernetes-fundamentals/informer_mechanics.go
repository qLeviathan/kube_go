// Package main demonstrates the internal mechanics of Kubernetes informers
// and cached clients. This is the answer to "How do k8s cached clients work?"
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// SECTION 1: The DeltaFIFO - Heart of the Caching System
// =============================================================================

// DeltaType represents the type of change observed
type DeltaType string

const (
	Added    DeltaType = "Added"
	Modified DeltaType = "Modified"
	Deleted  DeltaType = "Deleted"
	Sync     DeltaType = "Sync" // Periodic resync, not a real change
)

// Delta represents a single change event
type Delta struct {
	Type   DeltaType
	Object interface{}
}

// DeltaFIFO is a producer-consumer queue where:
// - Multiple deltas for the same object are coalesced
// - Pop() blocks until items are available
// - Items are deduped by key
//
// This is a simplified version of k8s.io/client-go/tools/cache.DeltaFIFO
type DeltaFIFO struct {
	mu sync.Mutex
	// items maps object key -> list of deltas for that object
	items map[string][]Delta
	// queue maintains FIFO order of keys
	queue []string
	// keyFunc extracts a key from an object
	keyFunc func(obj interface{}) string
	// cond for blocking Pop
	cond *sync.Cond
}

// NewDeltaFIFO creates a new DeltaFIFO with the given key function
func NewDeltaFIFO(keyFunc func(obj interface{}) string) *DeltaFIFO {
	df := &DeltaFIFO{
		items:   make(map[string][]Delta),
		queue:   make([]string, 0),
		keyFunc: keyFunc,
	}
	df.cond = sync.NewCond(&df.mu)
	return df
}

// Add enqueues an Add delta
func (df *DeltaFIFO) Add(obj interface{}) {
	df.queueDelta(Delta{Type: Added, Object: obj})
}

// Update enqueues an Update delta
func (df *DeltaFIFO) Update(obj interface{}) {
	df.queueDelta(Delta{Type: Modified, Object: obj})
}

// Delete enqueues a Delete delta
func (df *DeltaFIFO) Delete(obj interface{}) {
	df.queueDelta(Delta{Type: Deleted, Object: obj})
}

func (df *DeltaFIFO) queueDelta(d Delta) {
	df.mu.Lock()
	defer df.mu.Unlock()

	key := df.keyFunc(d.Object)

	// Key insight: If this key isn't in the queue, add it
	// If it is, we just append to existing deltas (coalescing)
	if _, exists := df.items[key]; !exists {
		df.queue = append(df.queue, key)
	}
	df.items[key] = append(df.items[key], d)

	df.cond.Signal()
}

// Pop removes and returns the oldest item (all deltas for one object)
// This blocks if the queue is empty
func (df *DeltaFIFO) Pop() (string, []Delta) {
	df.mu.Lock()
	defer df.mu.Unlock()

	for len(df.queue) == 0 {
		df.cond.Wait()
	}

	// FIFO: take from front of queue
	key := df.queue[0]
	df.queue = df.queue[1:]

	deltas := df.items[key]
	delete(df.items, key)

	return key, deltas
}

// =============================================================================
// SECTION 2: The Indexer - Thread-Safe In-Memory Cache
// =============================================================================

// Indexer maintains a thread-safe cache of objects with optional indexes
// This is a simplified version of k8s.io/client-go/tools/cache.Indexer
type Indexer struct {
	mu sync.RWMutex
	// Primary storage: key -> object
	items map[string]interface{}
	// Index functions: indexName -> func(obj) -> []indexKeys
	indexers map[string]func(obj interface{}) []string
	// Indexes: indexName -> indexKey -> set of object keys
	indices map[string]map[string]map[string]struct{}
	// KeyFunc extracts primary key from object
	keyFunc func(obj interface{}) string
}

// NewIndexer creates an Indexer with the given key function
func NewIndexer(keyFunc func(obj interface{}) string) *Indexer {
	return &Indexer{
		items:    make(map[string]interface{}),
		indexers: make(map[string]func(obj interface{}) []string),
		indices:  make(map[string]map[string]map[string]struct{}),
		keyFunc:  keyFunc,
	}
}

// AddIndexer adds a secondary index
func (idx *Indexer) AddIndexer(name string, indexFunc func(obj interface{}) []string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.indexers[name] = indexFunc
	idx.indices[name] = make(map[string]map[string]struct{})
}

// Add adds or updates an object in the cache
func (idx *Indexer) Add(obj interface{}) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	key := idx.keyFunc(obj)

	// Remove old index entries if updating
	if oldObj, exists := idx.items[key]; exists {
		idx.deleteFromIndices(key, oldObj)
	}

	// Store the object
	idx.items[key] = obj

	// Update all indices
	idx.addToIndices(key, obj)
}

// Delete removes an object from the cache
func (idx *Indexer) Delete(obj interface{}) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	key := idx.keyFunc(obj)
	if oldObj, exists := idx.items[key]; exists {
		idx.deleteFromIndices(key, oldObj)
		delete(idx.items, key)
	}
}

// Get retrieves an object by key
func (idx *Indexer) Get(key string) (interface{}, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	obj, exists := idx.items[key]
	return obj, exists
}

// ByIndex returns all objects matching an index value
func (idx *Indexer) ByIndex(indexName, indexKey string) []interface{} {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	indexMap, ok := idx.indices[indexName]
	if !ok {
		return nil
	}

	objKeys, ok := indexMap[indexKey]
	if !ok {
		return nil
	}

	result := make([]interface{}, 0, len(objKeys))
	for key := range objKeys {
		if obj, exists := idx.items[key]; exists {
			result = append(result, obj)
		}
	}
	return result
}

func (idx *Indexer) addToIndices(key string, obj interface{}) {
	for indexName, indexFunc := range idx.indexers {
		indexKeys := indexFunc(obj)
		for _, indexKey := range indexKeys {
			if idx.indices[indexName][indexKey] == nil {
				idx.indices[indexName][indexKey] = make(map[string]struct{})
			}
			idx.indices[indexName][indexKey][key] = struct{}{}
		}
	}
}

func (idx *Indexer) deleteFromIndices(key string, obj interface{}) {
	for indexName, indexFunc := range idx.indexers {
		indexKeys := indexFunc(obj)
		for _, indexKey := range indexKeys {
			if idx.indices[indexName][indexKey] != nil {
				delete(idx.indices[indexName][indexKey], key)
			}
		}
	}
}

// =============================================================================
// SECTION 3: The Reflector - ListWatch Implementation
// =============================================================================

// ResourceVersion tracks the version of the resource list
// In real k8s, this comes from etcd's revision number
type ResourceVersion string

// ListWatcher abstracts the LIST and WATCH operations against the API server
type ListWatcher interface {
	List() ([]interface{}, ResourceVersion, error)
	Watch(rv ResourceVersion) (<-chan WatchEvent, error)
}

// WatchEvent represents a single event from the watch stream
type WatchEvent struct {
	Type   DeltaType
	Object interface{}
}

// Reflector connects a ListWatcher to a DeltaFIFO
// It performs:
// 1. Initial LIST to populate the cache
// 2. WATCH to stream updates
// 3. Re-LIST on errors or desync
type Reflector struct {
	lw        ListWatcher
	store     *DeltaFIFO
	lastRV    ResourceVersion
	stopCh    chan struct{}
	isRunning bool
	mu        sync.Mutex
}

// NewReflector creates a Reflector
func NewReflector(lw ListWatcher, store *DeltaFIFO) *Reflector {
	return &Reflector{
		lw:     lw,
		store:  store,
		stopCh: make(chan struct{}),
	}
}

// Run starts the reflector's list-watch loop
func (r *Reflector) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.isRunning {
		r.mu.Unlock()
		return fmt.Errorf("reflector already running")
	}
	r.isRunning = true
	r.mu.Unlock()

	// Step 1: Initial LIST
	objects, rv, err := r.lw.List()
	if err != nil {
		return fmt.Errorf("initial list failed: %w", err)
	}

	// Populate DeltaFIFO with initial state
	for _, obj := range objects {
		r.store.Add(obj)
	}
	r.lastRV = rv

	// Step 2: WATCH from the resourceVersion we got from LIST
	// This ensures no events are missed
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		watchCh, err := r.lw.Watch(r.lastRV)
		if err != nil {
			// In real code, this would trigger a re-LIST
			time.Sleep(time.Second)
			continue
		}

		if err := r.watchHandler(ctx, watchCh); err != nil {
			// Watch ended, loop will restart it
			continue
		}
	}
}

func (r *Reflector) watchHandler(ctx context.Context, watchCh <-chan WatchEvent) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watchCh:
			if !ok {
				return fmt.Errorf("watch channel closed")
			}
			switch event.Type {
			case Added:
				r.store.Add(event.Object)
			case Modified:
				r.store.Update(event.Object)
			case Deleted:
				r.store.Delete(event.Object)
			}
		}
	}
}

// =============================================================================
// SECTION 4: The Controller - Tying It All Together
// =============================================================================

// WorkQueue is a rate-limited queue for reconciliation keys
type WorkQueue struct {
	mu    sync.Mutex
	items []string
	set   map[string]struct{} // deduplication
	cond  *sync.Cond
}

// NewWorkQueue creates a new work queue
func NewWorkQueue() *WorkQueue {
	wq := &WorkQueue{
		items: make([]string, 0),
		set:   make(map[string]struct{}),
	}
	wq.cond = sync.NewCond(&wq.mu)
	return wq
}

// Add enqueues a key (deduplicated)
func (wq *WorkQueue) Add(key string) {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	if _, exists := wq.set[key]; exists {
		return // Already queued, skip
	}
	wq.set[key] = struct{}{}
	wq.items = append(wq.items, key)
	wq.cond.Signal()
}

// Get retrieves the next key (blocks if empty)
func (wq *WorkQueue) Get() string {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	for len(wq.items) == 0 {
		wq.cond.Wait()
	}
	key := wq.items[0]
	wq.items = wq.items[1:]
	delete(wq.set, key)
	return key
}

// Controller is the canonical Kubernetes controller pattern
type Controller struct {
	indexer   *Indexer
	queue     *WorkQueue
	reconcile func(key string) error
}

// NewController creates a controller
func NewController(indexer *Indexer, reconcile func(key string) error) *Controller {
	return &Controller{
		indexer:   indexer,
		queue:     NewWorkQueue(),
		reconcile: reconcile,
	}
}

// OnAdd handles informer Add events
func (c *Controller) OnAdd(obj interface{}) {
	key := c.indexer.keyFunc(obj)
	c.indexer.Add(obj)
	c.queue.Add(key)
}

// OnUpdate handles informer Update events
func (c *Controller) OnUpdate(oldObj, newObj interface{}) {
	key := c.indexer.keyFunc(newObj)
	c.indexer.Add(newObj)
	c.queue.Add(key)
}

// OnDelete handles informer Delete events
func (c *Controller) OnDelete(obj interface{}) {
	key := c.indexer.keyFunc(obj)
	c.indexer.Delete(obj)
	c.queue.Add(key)
}

// Run starts the controller's reconcile loop
func (c *Controller) Run(ctx context.Context, workers int) {
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					key := c.queue.Get()
					if err := c.reconcile(key); err != nil {
						// In real controllers, this would requeue with backoff
						c.queue.Add(key)
					}
				}
			}
		}()
	}
	wg.Wait()
}

// =============================================================================
// SECTION 5: Example Pod Type and Demo
// =============================================================================

// Pod is a simplified Pod representation
type Pod struct {
	Namespace string
	Name      string
	Labels    map[string]string
	Phase     string
}

// PodKey returns namespace/name
func PodKey(obj interface{}) string {
	pod := obj.(*Pod)
	return pod.Namespace + "/" + pod.Name
}

// NamespaceIndex returns the namespace for indexing
func NamespaceIndex(obj interface{}) []string {
	pod := obj.(*Pod)
	return []string{pod.Namespace}
}

func main() {
	fmt.Println("Run 'go test -v' to see the informer mechanics in action")
}
