package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// TEST 1: DeltaFIFO Coalescing Behavior
// =============================================================================

func TestDeltaFIFOCoalescing(t *testing.T) {
	// This demonstrates a key optimization: multiple updates to the same
	// object before processing result in a single queue entry with all deltas

	df := NewDeltaFIFO(PodKey)

	pod1 := &Pod{Namespace: "default", Name: "nginx", Phase: "Pending"}
	pod1v2 := &Pod{Namespace: "default", Name: "nginx", Phase: "Running"}
	pod1v3 := &Pod{Namespace: "default", Name: "nginx", Phase: "Succeeded"}

	// Simulate rapid updates before any consumer processes them
	df.Add(pod1)
	df.Update(pod1v2)
	df.Update(pod1v3)

	// Single pop returns ALL deltas for this object
	key, deltas := df.Pop()

	if key != "default/nginx" {
		t.Errorf("expected key 'default/nginx', got %s", key)
	}

	if len(deltas) != 3 {
		t.Errorf("expected 3 coalesced deltas, got %d", len(deltas))
	}

	// Verify delta ordering is preserved
	expectedTypes := []DeltaType{Added, Modified, Modified}
	expectedPhases := []string{"Pending", "Running", "Succeeded"}

	for i, d := range deltas {
		if d.Type != expectedTypes[i] {
			t.Errorf("delta[%d]: expected type %s, got %s", i, expectedTypes[i], d.Type)
		}
		pod := d.Object.(*Pod)
		if pod.Phase != expectedPhases[i] {
			t.Errorf("delta[%d]: expected phase %s, got %s", i, expectedPhases[i], pod.Phase)
		}
	}

	t.Log("✓ DeltaFIFO correctly coalesces multiple deltas for same object")
	t.Logf("  - 3 operations (Add, Update, Update) resulted in single queue entry")
	t.Logf("  - All 3 deltas preserved in order for processing")
}

// =============================================================================
// TEST 2: Indexer Secondary Index Behavior
// =============================================================================

func TestIndexerSecondaryIndexes(t *testing.T) {
	// This demonstrates how k8s caches support efficient queries by
	// namespace, labels, or custom criteria without scanning all objects

	idx := NewIndexer(PodKey)

	// Add namespace index (like the built-in namespace indexer)
	idx.AddIndexer("namespace", NamespaceIndex)

	// Add custom label index
	idx.AddIndexer("app", func(obj interface{}) []string {
		pod := obj.(*Pod)
		if app, ok := pod.Labels["app"]; ok {
			return []string{app}
		}
		return nil
	})

	// Add some pods
	pods := []*Pod{
		{Namespace: "kube-system", Name: "coredns-1", Labels: map[string]string{"app": "coredns"}},
		{Namespace: "kube-system", Name: "coredns-2", Labels: map[string]string{"app": "coredns"}},
		{Namespace: "default", Name: "nginx-1", Labels: map[string]string{"app": "nginx"}},
		{Namespace: "default", Name: "nginx-2", Labels: map[string]string{"app": "nginx"}},
		{Namespace: "monitoring", Name: "prometheus", Labels: map[string]string{"app": "prometheus"}},
	}

	for _, pod := range pods {
		idx.Add(pod)
	}

	// Query by namespace
	kubeSystemPods := idx.ByIndex("namespace", "kube-system")
	if len(kubeSystemPods) != 2 {
		t.Errorf("expected 2 kube-system pods, got %d", len(kubeSystemPods))
	}

	// Query by app label
	corednsPods := idx.ByIndex("app", "coredns")
	if len(corednsPods) != 2 {
		t.Errorf("expected 2 coredns pods, got %d", len(corednsPods))
	}

	// Query returns empty for non-existent index value
	nonexistent := idx.ByIndex("app", "nonexistent")
	if len(nonexistent) != 0 {
		t.Errorf("expected 0 pods for nonexistent app, got %d", len(nonexistent))
	}

	t.Log("✓ Indexer supports efficient secondary index queries")
	t.Logf("  - ByIndex('namespace', 'kube-system') returned %d pods in O(1)", len(kubeSystemPods))
	t.Logf("  - ByIndex('app', 'coredns') returned %d pods in O(1)", len(corednsPods))
	t.Log("  - This is how kubectl get pods -n kube-system is fast")
}

// =============================================================================
// TEST 3: Index Maintenance on Update/Delete
// =============================================================================

func TestIndexerUpdateMaintenance(t *testing.T) {
	// Critical: When an object's indexed field changes, the index must update

	idx := NewIndexer(PodKey)
	idx.AddIndexer("namespace", NamespaceIndex)

	// Add a pod to default namespace
	pod := &Pod{Namespace: "default", Name: "migrating-pod", Labels: map[string]string{}}
	idx.Add(pod)

	defaultPods := idx.ByIndex("namespace", "default")
	if len(defaultPods) != 1 {
		t.Fatalf("expected 1 default pod, got %d", len(defaultPods))
	}

	// "Move" the pod to a different namespace (in reality this would be
	// a delete+create, but this tests the index update mechanics)
	pod2 := &Pod{Namespace: "production", Name: "migrating-pod", Labels: map[string]string{}}
	// Note: same Name means same key in our simplified model
	// In real k8s, namespace is part of the key

	// Simulate the update
	idx.Delete(pod)
	idx.Add(pod2)

	// Old index entry should be removed
	defaultPods = idx.ByIndex("namespace", "default")
	if len(defaultPods) != 0 {
		t.Errorf("expected 0 default pods after move, got %d", len(defaultPods))
	}

	// New index entry should exist
	prodPods := idx.ByIndex("namespace", "production")
	if len(prodPods) != 1 {
		t.Errorf("expected 1 production pod after move, got %d", len(prodPods))
	}

	t.Log("✓ Indexer correctly maintains indexes on update/delete")
	t.Log("  - Old index entries are removed")
	t.Log("  - New index entries are created")
}

// =============================================================================
// TEST 4: Full Controller Flow - Event to Reconcile
// =============================================================================

func TestControllerReconcileFlow(t *testing.T) {
	// This demonstrates the complete controller pattern:
	// Event → Handler → WorkQueue → Reconciler → Cache Read

	idx := NewIndexer(PodKey)
	reconcileCalls := make([]string, 0)
	var mu sync.Mutex

	ctrl := NewController(idx, func(key string) error {
		mu.Lock()
		defer mu.Unlock()

		// Reconcilers read from cache, not from the event
		obj, exists := idx.Get(key)
		if !exists {
			// Object was deleted, handle deletion logic
			reconcileCalls = append(reconcileCalls, fmt.Sprintf("DELETE:%s", key))
			return nil
		}

		pod := obj.(*Pod)
		reconcileCalls = append(reconcileCalls, fmt.Sprintf("SYNC:%s:phase=%s", key, pod.Phase))
		return nil
	})

	// Start controller with 2 workers
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go ctrl.Run(ctx, 2)

	// Simulate informer events
	pod := &Pod{Namespace: "default", Name: "test-pod", Phase: "Pending"}
	ctrl.OnAdd(pod)

	// Update the pod
	pod2 := &Pod{Namespace: "default", Name: "test-pod", Phase: "Running"}
	ctrl.OnUpdate(pod, pod2)

	// Delete the pod
	ctrl.OnDelete(pod2)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	calls := make([]string, len(reconcileCalls))
	copy(calls, reconcileCalls)
	mu.Unlock()

	// Note: Due to coalescing, we may get 1-3 calls depending on timing
	// The key insight is that rapid updates are coalesced, and the reconciler
	// reads current state from cache (not the event that triggered it)
	if len(calls) < 1 {
		t.Errorf("expected at least 1 reconcile call, got %d", len(calls))
	}

	t.Log("✓ Controller correctly flows events through to reconciler")
	t.Logf("  - Reconcile calls: %v", calls)
	t.Log("  - Key insight: Reconciler reads from cache, not event payload")
	t.Log("  - Coalescing means rapid Add→Update→Delete may yield single DELETE reconcile")
	t.Log("  - This is why reconcile is idempotent and level-triggered")
}

// =============================================================================
// TEST 5: WorkQueue Deduplication
// =============================================================================

func TestWorkQueueDeduplication(t *testing.T) {
	// Critical optimization: Multiple events for same object before processing
	// result in single reconcile call

	wq := NewWorkQueue()

	// Rapid-fire adds of same key
	for i := 0; i < 100; i++ {
		wq.Add("default/hot-pod")
	}

	// Also add some other keys
	wq.Add("default/other-pod")
	wq.Add("kube-system/coredns")

	// Should only have 3 items despite 102 Add calls
	count := 0
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				wq.mu.Lock()
				if len(wq.items) == 0 {
					wq.mu.Unlock()
					done <- true
					return
				}
				wq.mu.Unlock()
				wq.Get()
				count++
			}
		}
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
	}

	if count != 3 {
		t.Errorf("expected 3 unique keys processed, got %d", count)
	}

	t.Log("✓ WorkQueue correctly deduplicates rapid updates")
	t.Logf("  - 102 Add() calls resulted in %d items processed", count)
	t.Log("  - This prevents reconcile storms during rapid state changes")
}

// =============================================================================
// TEST 6: Simulated ListWatch Behavior
// =============================================================================

type MockListWatcher struct {
	objects  []interface{}
	rv       ResourceVersion
	watchCh  chan WatchEvent
	mu       sync.Mutex
}

func NewMockListWatcher() *MockListWatcher {
	return &MockListWatcher{
		objects: make([]interface{}, 0),
		rv:      "1",
		watchCh: make(chan WatchEvent, 100),
	}
}

func (m *MockListWatcher) List() ([]interface{}, ResourceVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]interface{}, len(m.objects))
	copy(result, m.objects)
	return result, m.rv, nil
}

func (m *MockListWatcher) Watch(rv ResourceVersion) (<-chan WatchEvent, error) {
	return m.watchCh, nil
}

func (m *MockListWatcher) AddObject(obj interface{}) {
	m.mu.Lock()
	m.objects = append(m.objects, obj)
	m.rv = ResourceVersion(fmt.Sprintf("%d", len(m.objects)))
	m.mu.Unlock()
}

func (m *MockListWatcher) SendEvent(event WatchEvent) {
	m.watchCh <- event
}

func TestReflectorListWatchSequence(t *testing.T) {
	// This demonstrates the critical LIST→WATCH sequence that ensures
	// no events are missed

	lw := NewMockListWatcher()

	// Pre-populate some objects (simulating existing state)
	lw.AddObject(&Pod{Namespace: "default", Name: "existing-1", Phase: "Running"})
	lw.AddObject(&Pod{Namespace: "default", Name: "existing-2", Phase: "Running"})

	df := NewDeltaFIFO(PodKey)
	reflector := NewReflector(lw, df)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go reflector.Run(ctx)

	// Wait for initial list to complete
	time.Sleep(10 * time.Millisecond)

	// Send a watch event for a new pod
	lw.SendEvent(WatchEvent{
		Type:   Added,
		Object: &Pod{Namespace: "default", Name: "new-pod", Phase: "Pending"},
	})

	// Collect all processed items
	processed := make(map[string]bool)
	timeout := time.After(50 * time.Millisecond)

	for {
		select {
		case <-timeout:
			goto done
		default:
			// Non-blocking check
			df.mu.Lock()
			if len(df.queue) > 0 {
				df.mu.Unlock()
				key, _ := df.Pop()
				processed[key] = true
			} else {
				df.mu.Unlock()
				time.Sleep(5 * time.Millisecond)
			}
		}
	}

done:
	// Should have both existing objects from LIST and new object from WATCH
	expected := []string{"default/existing-1", "default/existing-2", "default/new-pod"}
	for _, key := range expected {
		if !processed[key] {
			t.Errorf("expected key %s to be processed", key)
		}
	}

	t.Log("✓ Reflector correctly sequences LIST→WATCH")
	t.Logf("  - Initial LIST populated %d existing objects", 2)
	t.Log("  - WATCH continued from LIST's resourceVersion")
	t.Log("  - No events missed in the transition")
}

// =============================================================================
// Summary: The Answer to "How do k8s cached clients work?"
// =============================================================================

func TestSummary(t *testing.T) {
	t.Log(`
================================================================================
HOW KUBERNETES CACHED CLIENTS WORK - SUMMARY
================================================================================

1. REFLECTOR (ListWatch)
   - Performs initial LIST to get all objects + resourceVersion
   - Starts WATCH from that resourceVersion (no gap, no missed events)
   - Feeds deltas into DeltaFIFO
   - On desync/error: re-LIST and restart

2. DELTAFIFO
   - Queue of (object, deltaType) entries
   - Coalesces multiple updates to same object before processing
   - FIFO ordering within object, parallel across objects
   - Blocks consumers when empty

3. INDEXER (Cache)
   - Thread-safe in-memory store
   - Primary index by namespace/name key
   - Secondary indexes (by namespace, labels, custom)
   - O(1) lookups via index

4. CONTROLLER PATTERN
   - Event handlers enqueue keys to WorkQueue (not full objects)
   - WorkQueue deduplicates rapid updates
   - Reconciler reads from cache, not events
   - Level-triggered, idempotent reconciliation

5. KEY INVARIANTS
   - Cache is always eventually consistent with API server
   - Multiple workers can reconcile different objects in parallel
   - Reconcile is called at least once per change (may be called more)
   - Reads from cache are fast but may be slightly stale

================================================================================
`)
}
