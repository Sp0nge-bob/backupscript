package agent

import (
	"fmt"
	"sync"
	"time"
)

type PathReport struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

type NodeState struct {
	LastSeen    time.Time
	LastUpload  time.Time
	Version     string
	PathReports []PathReport
	LastError   string
}

type Registry struct {
	mu            sync.RWMutex
	nodes         map[string]*NodeState
	syncRequested map[string]time.Time
	waiters       map[string]map[chan bool]struct{}
}

func NewRegistry() *Registry {
	return &Registry{
		nodes:         make(map[string]*NodeState),
		syncRequested: make(map[string]time.Time),
		waiters:       make(map[string]map[chan bool]struct{}),
	}
}

func (r *Registry) touchNode(node string) *NodeState {
	state, ok := r.nodes[node]
	if !ok {
		state = &NodeState{}
		r.nodes[node] = state
	}
	state.LastSeen = time.Now()
	return state
}

func (r *Registry) RecordContact(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.touchNode(node)
}

func (r *Registry) RecordUpload(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state := r.touchNode(node)
	now := time.Now()
	state.LastUpload = now
	state.LastError = ""
	delete(r.syncRequested, node)
}

func (r *Registry) RecordError(node, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state := r.touchNode(node)
	state.LastError = errMsg
}

func (r *Registry) RegisterWaiter(node string) chan bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan bool, 1)
	if r.waiters[node] == nil {
		r.waiters[node] = make(map[chan bool]struct{})
	}
	r.waiters[node][ch] = struct{}{}
	return ch
}

func (r *Registry) UnregisterWaiter(node string, ch chan bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if waiters, ok := r.waiters[node]; ok {
		delete(waiters, ch)
		if len(waiters) == 0 {
			delete(r.waiters, node)
		}
	}
	close(ch)
}

func (r *Registry) RequestSync(node string) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	r.syncRequested[node] = now
	r.wakeWaitersLocked(node, true)
	return now
}

func (r *Registry) WakeNode(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.wakeWaitersLocked(node, false)
}

func (r *Registry) wakeWaitersLocked(node string, syncRequired bool) {
	for ch := range r.waiters[node] {
		select {
		case ch <- syncRequired:
		default:
		}
	}
}

func (r *Registry) SyncRequired(node string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	requested, ok := r.syncRequested[node]
	if !ok {
		return false
	}

	state, ok := r.nodes[node]
	if !ok || state.LastUpload.IsZero() || !state.LastUpload.After(requested) {
		return true
	}
	return false
}

func (r *Registry) Get(node string) (NodeState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, ok := r.nodes[node]
	if !ok {
		return NodeState{}, false
	}
	return *state, true
}

func (r *Registry) IsOnline(node string, maxAge time.Duration) bool {
	state, ok := r.Get(node)
	if !ok || state.LastSeen.IsZero() {
		return false
	}
	return time.Since(state.LastSeen) <= maxAge
}

func (r *Registry) StagingAge(node string) (time.Duration, bool) {
	state, ok := r.Get(node)
	if !ok || state.LastUpload.IsZero() {
		return 0, false
	}
	return time.Since(state.LastUpload), true
}

func (r *Registry) WaitForFreshUpload(node string, since time.Time, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, ok := r.Get(node)
		if ok && !state.LastUpload.IsZero() && state.LastUpload.After(since) {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("таймаут ожидания свежих данных от %s (%s)", node, timeout)
}

func (r *Registry) WaitForContact(node string, since time.Time, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, ok := r.Get(node)
		if ok && !state.LastSeen.IsZero() && state.LastSeen.After(since) {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("агент %s не ответил за %s", node, timeout)
}

func (r *Registry) HasWaiter(node string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.waiters[node]) > 0
}