package events

import (
	"context"
	"sync"
	"time"
)

// EventType represents the type of event
type EventType string

const (
	EventResourceListed   EventType = "resource.listed"
	EventResourceRead     EventType = "resource.read"
	EventToolListed       EventType = "tool.listed"
	EventToolCalled       EventType = "tool.called"
	EventPromptListed     EventType = "prompt.listed"
	EventPromptExecuted   EventType = "prompt.executed"
	EventClientConnected  EventType = "client.connected"
	EventClientDisconnected EventType = "client.disconnected"
	EventRequestReceived  EventType = "request.received"
	EventResponseSent     EventType = "response.sent"
	EventError           EventType = "error"
)

// Event represents an event that occurred in the MCP server
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Context   context.Context        `json:"-"`
}

// EventHook is a function that gets called when an event occurs
type EventHook func(event Event) error

// EventEmitter manages event hooks and emission
type EventEmitter struct {
	hooks map[EventType][]EventHook
	mu    sync.RWMutex
}

// NewEventEmitter creates a new event emitter
func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		hooks: make(map[EventType][]EventHook),
	}
}

// On registers an event hook for a specific event type
func (e *EventEmitter) On(eventType EventType, hook EventHook) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.hooks[eventType] = append(e.hooks[eventType], hook)
}

// Off removes an event hook (removes all hooks for the event type)
func (e *EventEmitter) Off(eventType EventType) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.hooks, eventType)
}

// Emit triggers all hooks for a specific event type
func (e *EventEmitter) Emit(event Event) {
	// Set timestamp if not already set
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixNano()
	}
	
	e.mu.RLock()
	hooks := e.hooks[event.Type]
	e.mu.RUnlock()
	
	for _, hook := range hooks {
		go func(h EventHook) {
			// Execute hook in goroutine to avoid blocking
			if err := h(event); err != nil {
				// Could log error here, but for now just ignore
			}
		}(hook)
	}
}

// WithEventHook is now defined in the main mcp package to avoid circular dependency

// Common event data builders

// ResourceEventData creates event data for resource events
func ResourceEventData(uri string, result interface{}) map[string]interface{} {
	return map[string]interface{}{
		"uri":    uri,
		"result": result,
	}
}

// ToolEventData creates event data for tool events
func ToolEventData(name string, arguments interface{}, result interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name":      name,
		"arguments": arguments,
		"result":    result,
	}
}

// PromptEventData creates event data for prompt events
func PromptEventData(name string, arguments interface{}, result interface{}) map[string]interface{} {
	return map[string]interface{}{
		"name":      name,
		"arguments": arguments,
		"result":    result,
	}
}

// RequestEventData creates event data for request events
func RequestEventData(method string, params interface{}) map[string]interface{} {
	return map[string]interface{}{
		"method": method,
		"params": params,
	}
}

// ResponseEventData creates event data for response events
func ResponseEventData(method string, result interface{}, error interface{}) map[string]interface{} {
	return map[string]interface{}{
		"method": method,
		"result": result,
		"error":  error,
	}
}

// ErrorEventData creates event data for error events
func ErrorEventData(error string, context map[string]interface{}) map[string]interface{} {
	data := map[string]interface{}{
		"error": error,
	}
	for k, v := range context {
		data[k] = v
	}
	return data
}