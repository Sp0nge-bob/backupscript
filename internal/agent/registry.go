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
}

func NewRegistry() *Registry {
	return &Registry{
		nodes:         make(map[string]*NodeState),
		syncRequested: make(map[string]time.Time),
	}
}

func (r *Registry) RecordHeartbeat(node string, version string, paths []PathReport) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.nodes[node]
	if !ok {
		state = &NodeState{}
		r.nodes[node] = state
	}
	state.LastSeen = time.Now()
	state.Version = version
	state.PathReports = paths
	state.LastError = ""
}

func (r *Registry) RecordUpload(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.nodes[node]
	if !ok {
		state = &NodeState{}
		r.nodes[node] = state
	}
	now := time.Now()
	state.LastSeen = now
	state.LastUpload = now
	state.LastError = ""
	delete(r.syncRequested, node)
}

func (r *Registry) RecordError(node, errMsg string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.nodes[node]
	if !ok {
		state = &NodeState{}
		r.nodes[node] = state
	}
	state.LastSeen = time.Now()
	state.LastError = errMsg
}

func (r *Registry) RequestSync(node string) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	r.syncRequested[node] = now
	return now
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