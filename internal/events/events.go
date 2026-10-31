// Package events defines the event sink abstraction used by services to
// push real-time updates to the UI without importing any GUI framework.
package events

// Sink receives named events with a JSON-serializable payload.
type Sink interface {
	Emit(name string, data any)
}

// Noop discards all events. Useful in tests and CLI tools.
type Noop struct{}

func (Noop) Emit(string, any) {}
