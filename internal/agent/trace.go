package agent

import (
	"encoding/json"
	"sync"
	"time"
)

// TraceEvent records a single lifecycle phase.
type TraceEvent struct {
	Phase  string    `json:"phase"`
	At     time.Time `json:"at"`
	Detail string    `json:"detail,omitempty"`
}

// Trace collects lifecycle events during an agent run.
type Trace struct {
	mu     sync.Mutex
	events []TraceEvent
}

// NewTrace creates an empty trace.
func NewTrace() *Trace {
	return &Trace{}
}

// Record appends a timestamped event.
func (t *Trace) Record(phase, detail string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, TraceEvent{
		Phase:  phase,
		At:     time.Now(),
		Detail: detail,
	})
}

// JSON serializes the trace events.
func (t *Trace) JSON() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	data, _ := json.Marshal(t.events)
	return data
}
