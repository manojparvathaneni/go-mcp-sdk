package events

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestNewEventEmitter(t *testing.T) {
	emitter := NewEventEmitter()
	if emitter == nil {
		t.Fatal("Expected NewEventEmitter to return non-nil emitter")
	}
	if emitter.hooks == nil {
		t.Error("Expected hooks map to be initialized")
	}
}

func TestEventEmitter_On(t *testing.T) {
	emitter := NewEventEmitter()
	
	hook := func(event Event) error {
		return nil
	}
	
	emitter.On(EventResourceRead, hook)
	
	// Check that hook was registered
	emitter.mu.RLock()
	hooks := emitter.hooks[EventResourceRead]
	emitter.mu.RUnlock()
	
	if len(hooks) != 1 {
		t.Errorf("Expected 1 hook registered, got %d", len(hooks))
	}
}

func TestEventEmitter_On_MultipleHooks(t *testing.T) {
	emitter := NewEventEmitter()
	
	hook1 := func(event Event) error { return nil }
	hook2 := func(event Event) error { return nil }
	
	emitter.On(EventToolCalled, hook1)
	emitter.On(EventToolCalled, hook2)
	
	emitter.mu.RLock()
	hooks := emitter.hooks[EventToolCalled]
	emitter.mu.RUnlock()
	
	if len(hooks) != 2 {
		t.Errorf("Expected 2 hooks registered, got %d", len(hooks))
	}
}

func TestEventEmitter_Off(t *testing.T) {
	emitter := NewEventEmitter()
	
	hook := func(event Event) error { return nil }
	emitter.On(EventPromptExecuted, hook)
	
	// Verify hook was added
	emitter.mu.RLock()
	hooks := emitter.hooks[EventPromptExecuted]
	emitter.mu.RUnlock()
	if len(hooks) != 1 {
		t.Error("Expected hook to be registered")
	}
	
	// Remove hooks
	emitter.Off(EventPromptExecuted)
	
	// Verify hooks were removed
	emitter.mu.RLock()
	hooks = emitter.hooks[EventPromptExecuted]
	emitter.mu.RUnlock()
	if len(hooks) != 0 {
		t.Error("Expected all hooks to be removed")
	}
}

func TestEventEmitter_Emit(t *testing.T) {
	emitter := NewEventEmitter()
	var wg sync.WaitGroup
	var receivedEvent Event
	
	wg.Add(1)
	hook := func(event Event) error {
		receivedEvent = event
		wg.Done()
		return nil
	}
	
	emitter.On(EventClientConnected, hook)
	
	testEvent := Event{
		Type: EventClientConnected,
		Data: map[string]interface{}{"clientId": "test"},
		Context: context.Background(),
	}
	
	emitter.Emit(testEvent)
	
	// Wait for hook to be called
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Hook was not called within timeout")
	}
	
	if receivedEvent.Type != EventClientConnected {
		t.Errorf("Expected event type %s, got %s", EventClientConnected, receivedEvent.Type)
	}
	if receivedEvent.Data["clientId"] != "test" {
		t.Error("Expected event data to be preserved")
	}
	if receivedEvent.Timestamp == 0 {
		t.Error("Expected timestamp to be set automatically")
	}
}

func TestEventEmitter_Emit_PresetTimestamp(t *testing.T) {
	emitter := NewEventEmitter()
	var wg sync.WaitGroup
	var receivedEvent Event
	
	wg.Add(1)
	hook := func(event Event) error {
		receivedEvent = event
		wg.Done()
		return nil
	}
	
	emitter.On(EventError, hook)
	
	customTimestamp := int64(1234567890)
	testEvent := Event{
		Type: EventError,
		Timestamp: customTimestamp,
		Data: map[string]interface{}{"error": "test error"},
	}
	
	emitter.Emit(testEvent)
	
	// Wait for hook to be called
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Hook was not called within timeout")
	}
	
	if receivedEvent.Timestamp != customTimestamp {
		t.Errorf("Expected timestamp %d to be preserved, got %d", customTimestamp, receivedEvent.Timestamp)
	}
}

func TestEventEmitter_Emit_HookError(t *testing.T) {
	emitter := NewEventEmitter()
	var wg sync.WaitGroup
	
	wg.Add(1)
	hook := func(event Event) error {
		defer wg.Done()
		return errors.New("hook error")
	}
	
	emitter.On(EventRequestReceived, hook)
	
	testEvent := Event{
		Type: EventRequestReceived,
		Data: map[string]interface{}{"method": "test"},
	}
	
	// This should not panic even if hook returns error
	emitter.Emit(testEvent)
	
	// Wait for hook to be called
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success - error was handled gracefully
	case <-time.After(1 * time.Second):
		t.Fatal("Hook was not called within timeout")
	}
}

func TestEventEmitter_Emit_NoHooks(t *testing.T) {
	emitter := NewEventEmitter()
	
	testEvent := Event{
		Type: EventResponseSent,
		Data: map[string]interface{}{"status": "ok"},
	}
	
	// This should not panic when no hooks are registered
	emitter.Emit(testEvent)
}

func TestEventEmitter_Concurrent(t *testing.T) {
	emitter := NewEventEmitter()
	var wg sync.WaitGroup
	hookCount := 0
	mu := sync.Mutex{}
	
	// Register multiple hooks concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hook := func(event Event) error {
				mu.Lock()
				hookCount++
				mu.Unlock()
				return nil
			}
			emitter.On(EventToolListed, hook)
		}()
	}
	
	wg.Wait()
	
	// Emit event
	testEvent := Event{
		Type: EventToolListed,
		Data: map[string]interface{}{"count": 5},
	}
	
	emitter.Emit(testEvent)
	
	// Give time for all hooks to execute
	time.Sleep(100 * time.Millisecond)
	
	mu.Lock()
	finalCount := hookCount
	mu.Unlock()
	
	if finalCount != 10 {
		t.Errorf("Expected 10 hooks to be called, got %d", finalCount)
	}
}

func TestResourceEventData(t *testing.T) {
	uri := "file://test.txt"
	result := map[string]interface{}{"content": "test content"}
	
	data := ResourceEventData(uri, result)
	
	if data["uri"] != uri {
		t.Errorf("Expected uri %s, got %v", uri, data["uri"])
	}
	if !reflect.DeepEqual(data["result"], result) {
		t.Error("Expected result to be preserved")
	}
}

func TestToolEventData(t *testing.T) {
	name := "test-tool"
	arguments := map[string]interface{}{"param": "value"}
	result := map[string]interface{}{"output": "success"}
	
	data := ToolEventData(name, arguments, result)
	
	if data["name"] != name {
		t.Errorf("Expected name %s, got %v", name, data["name"])
	}
	if !reflect.DeepEqual(data["arguments"], arguments) {
		t.Error("Expected arguments to be preserved")
	}
	if !reflect.DeepEqual(data["result"], result) {
		t.Error("Expected result to be preserved")
	}
}

func TestPromptEventData(t *testing.T) {
	name := "test-prompt"
	arguments := map[string]interface{}{"input": "test"}
	result := map[string]interface{}{"output": "response"}
	
	data := PromptEventData(name, arguments, result)
	
	if data["name"] != name {
		t.Errorf("Expected name %s, got %v", name, data["name"])
	}
	if !reflect.DeepEqual(data["arguments"], arguments) {
		t.Error("Expected arguments to be preserved")
	}
	if !reflect.DeepEqual(data["result"], result) {
		t.Error("Expected result to be preserved")
	}
}

func TestRequestEventData(t *testing.T) {
	method := "test/method"
	params := map[string]interface{}{"param": "value"}
	
	data := RequestEventData(method, params)
	
	if data["method"] != method {
		t.Errorf("Expected method %s, got %v", method, data["method"])
	}
	if !reflect.DeepEqual(data["params"], params) {
		t.Error("Expected params to be preserved")
	}
}

func TestResponseEventData(t *testing.T) {
	method := "test/method"
	result := map[string]interface{}{"status": "ok"}
	errorData := map[string]interface{}{"code": 500}
	
	data := ResponseEventData(method, result, errorData)
	
	if data["method"] != method {
		t.Errorf("Expected method %s, got %v", method, data["method"])
	}
	if !reflect.DeepEqual(data["result"], result) {
		t.Error("Expected result to be preserved")
	}
	if !reflect.DeepEqual(data["error"], errorData) {
		t.Error("Expected error to be preserved")
	}
}

func TestErrorEventData(t *testing.T) {
	errorMsg := "test error"
	context := map[string]interface{}{
		"component": "server",
		"operation": "read",
	}
	
	data := ErrorEventData(errorMsg, context)
	
	if data["error"] != errorMsg {
		t.Errorf("Expected error %s, got %v", errorMsg, data["error"])
	}
	if data["component"] != "server" {
		t.Error("Expected context component to be preserved")
	}
	if data["operation"] != "read" {
		t.Error("Expected context operation to be preserved")
	}
}

func TestErrorEventData_EmptyContext(t *testing.T) {
	errorMsg := "test error"
	context := map[string]interface{}{}
	
	data := ErrorEventData(errorMsg, context)
	
	if data["error"] != errorMsg {
		t.Errorf("Expected error %s, got %v", errorMsg, data["error"])
	}
	
	// Should only have error key
	if len(data) != 1 {
		t.Errorf("Expected 1 key in data, got %d", len(data))
	}
}

func TestEventType_Constants(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventResourceListed, "resource.listed"},
		{EventResourceRead, "resource.read"},
		{EventToolListed, "tool.listed"},
		{EventToolCalled, "tool.called"},
		{EventPromptListed, "prompt.listed"},
		{EventPromptExecuted, "prompt.executed"},
		{EventClientConnected, "client.connected"},
		{EventClientDisconnected, "client.disconnected"},
		{EventRequestReceived, "request.received"},
		{EventResponseSent, "response.sent"},
		{EventError, "error"},
	}
	
	for _, tt := range tests {
		if string(tt.eventType) != tt.expected {
			t.Errorf("Expected EventType %s to equal %s", tt.eventType, tt.expected)
		}
	}
}