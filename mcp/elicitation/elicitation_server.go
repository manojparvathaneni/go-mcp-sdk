package elicitation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// ElicitationHandler defines the interface for handling elicitation requests
type ElicitationHandler interface {
	// Elicit requests information from the user and waits for a response
	Elicit(ctx context.Context, req types.ElicitRequest) (*types.ElicitResult, error)
}

// ElicitationManager manages active elicitation requests on the server side
type ElicitationManager struct {
	mu              sync.RWMutex
	activeElicits   map[string]*pendingElicit
	transport       ElicitationTransport
	responseTimeout time.Duration
	idGenerator     func() string
}

type pendingElicit struct {
	request    types.ElicitRequest
	responseCh chan types.ElicitResult
	cancelCh   chan string
	ctx        context.Context
	cancel     context.CancelFunc
	timestamp  time.Time
}

// ElicitationTransport defines the interface for sending elicitation notifications
type ElicitationTransport interface {
	SendElicitNotification(notification types.ElicitNotification) error
	SendElicitCancelNotification(notification types.ElicitCancelNotification) error
}

// NewElicitationManager creates a new elicitation manager
func NewElicitationManager(transport ElicitationTransport) *ElicitationManager {
	return &ElicitationManager{
		activeElicits:   make(map[string]*pendingElicit),
		transport:       transport,
		responseTimeout: 30 * time.Second, // Default timeout
		idGenerator:     generateElicitID,
	}
}

// SetResponseTimeout sets the default response timeout for elicitations
func (m *ElicitationManager) SetResponseTimeout(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseTimeout = timeout
}

// Elicit requests information from the user and waits for a response
func (m *ElicitationManager) Elicit(ctx context.Context, req types.ElicitRequest) (*types.ElicitResult, error) {
	// Generate unique ID for this elicitation
	elicitID := m.idGenerator()
	
	// Set up context with timeout
	timeout := m.responseTimeout
	if req.Timeout != nil && *req.Timeout > 0 {
		timeout = time.Duration(*req.Timeout) * time.Second
	}
	
	elicitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Create pending elicitation
	pending := &pendingElicit{
		request:    req,
		responseCh: make(chan types.ElicitResult, 1),
		cancelCh:   make(chan string, 1),
		ctx:        elicitCtx,
		cancel:     cancel,
		timestamp:  time.Now(),
	}
	
	// Register the pending elicitation
	m.mu.Lock()
	m.activeElicits[elicitID] = pending
	m.mu.Unlock()
	
	// Ensure cleanup on exit
	defer func() {
		m.mu.Lock()
		delete(m.activeElicits, elicitID)
		m.mu.Unlock()
	}()
	
	// Send elicitation notification to client
	notification := types.ElicitNotification{
		ElicitID: elicitID,
		Request:  req,
	}
	
	if err := m.transport.SendElicitNotification(notification); err != nil {
		return nil, fmt.Errorf("failed to send elicitation notification: %w", err)
	}
	
	// Wait for response, cancellation, or timeout
	select {
	case result := <-pending.responseCh:
		return &result, nil
		
	case reason := <-pending.cancelCh:
		return nil, &types.ElicitCancelledError{
			ElicitID: elicitID,
			Reason:   &reason,
		}
		
	case <-elicitCtx.Done():
		// Send cancellation notification
		cancelNotification := types.ElicitCancelNotification{
			ElicitID: elicitID,
			Reason:   stringPtr("timeout"),
		}
		m.transport.SendElicitCancelNotification(cancelNotification)
		
		if elicitCtx.Err() == context.DeadlineExceeded {
			return nil, &types.ElicitTimeoutError{
				ElicitID: elicitID,
				Timeout:  int(timeout.Seconds()),
			}
		}
		return nil, elicitCtx.Err()
	}
}

// HandleElicitResponse handles an elicitation response from the client
func (m *ElicitationManager) HandleElicitResponse(notification types.ElicitResponseNotification) error {
	m.mu.RLock()
	pending, exists := m.activeElicits[notification.ElicitID]
	m.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no active elicitation with ID: %s", notification.ElicitID)
	}
	
	// Validate the response against the schema if present
	if pending.request.Schema != nil {
		if err := validateElicitResponse(notification.Result, *pending.request.Schema); err != nil {
			// Send validation error back - in a real implementation,
			// you might want to ask for a corrected response
			return fmt.Errorf("validation failed: %w", err)
		}
	}
	
	// Send response to waiting goroutine
	select {
	case pending.responseCh <- notification.Result:
		return nil
	case <-pending.ctx.Done():
		return fmt.Errorf("elicitation %s has timed out", notification.ElicitID)
	}
}

// HandleElicitCancel handles an elicitation cancellation from the client
func (m *ElicitationManager) HandleElicitCancel(notification types.ElicitCancelNotification) error {
	m.mu.RLock()
	pending, exists := m.activeElicits[notification.ElicitID]
	m.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no active elicitation with ID: %s", notification.ElicitID)
	}
	
	reason := "cancelled by client"
	if notification.Reason != nil {
		reason = *notification.Reason
	}
	
	// Send cancellation to waiting goroutine
	select {
	case pending.cancelCh <- reason:
		return nil
	case <-pending.ctx.Done():
		return nil // Already timed out
	}
}

// GetActiveElicitations returns information about currently active elicitations
func (m *ElicitationManager) GetActiveElicitations() map[string]types.ElicitRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	active := make(map[string]types.ElicitRequest)
	for id, pending := range m.activeElicits {
		active[id] = pending.request
	}
	return active
}

// CancelElicitation cancels an active elicitation
func (m *ElicitationManager) CancelElicitation(elicitID string, reason string) error {
	m.mu.RLock()
	pending, exists := m.activeElicits[elicitID]
	m.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("no active elicitation with ID: %s", elicitID)
	}
	
	// Send cancellation notification to client
	notification := types.ElicitCancelNotification{
		ElicitID: elicitID,
		Reason:   &reason,
	}
	
	if err := m.transport.SendElicitCancelNotification(notification); err != nil {
		return fmt.Errorf("failed to send cancellation: %w", err)
	}
	
	// Cancel the local context
	pending.cancel()
	
	return nil
}

// validateElicitResponse validates a response against the provided schema
func validateElicitResponse(result types.ElicitResult, schema types.ElicitSchema) error {
	if result.Cancelled {
		return nil // No validation needed for cancelled responses
	}
	
	switch schema.Type {
	case "boolean":
		if _, ok := result.Value.(bool); !ok {
			return &types.ValidationError{
				Message: "expected boolean value",
				Code:    "type_mismatch",
			}
		}
	case "string":
		str, ok := result.Value.(string)
		if !ok {
			return &types.ValidationError{
				Message: "expected string value",
				Code:    "type_mismatch",
			}
		}
		if schema.Validation != nil {
			if err := validateString(str, *schema.Validation); err != nil {
				return err
			}
		}
	case "number":
		if _, ok := result.Value.(float64); !ok {
			return &types.ValidationError{
				Message: "expected number value",
				Code:    "type_mismatch",
			}
		}
	case "object":
		obj, ok := result.Value.(map[string]interface{})
		if !ok {
			return &types.ValidationError{
				Message: "expected object value",
				Code:    "type_mismatch",
			}
		}
		return validateObject(obj, schema)
	}
	
	return nil
}

// validateString validates a string value against validation rules
func validateString(value string, validation types.ElicitValidation) error {
	if validation.MinLength != nil && len(value) < *validation.MinLength {
		return &types.ValidationError{
			Message: fmt.Sprintf("minimum length is %d", *validation.MinLength),
			Code:    "min_length",
		}
	}
	
	if validation.MaxLength != nil && len(value) > *validation.MaxLength {
		return &types.ValidationError{
			Message: fmt.Sprintf("maximum length is %d", *validation.MaxLength),
			Code:    "max_length",
		}
	}
	
	// Pattern validation would go here (regex)
	// if validation.Pattern != nil { ... }
	
	return nil
}

// validateObject validates an object against the schema
func validateObject(obj map[string]interface{}, schema types.ElicitSchema) error {
	// Check required fields
	for _, required := range schema.Required {
		if _, exists := obj[required]; !exists {
			return &types.ValidationError{
				Field:   required,
				Message: "required field missing",
				Code:    "required_field",
			}
		}
	}
	
	// Validate each property
	for fieldName, value := range obj {
		if prop, exists := schema.Properties[fieldName]; exists {
			if err := validateProperty(fieldName, value, prop); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// validateProperty validates a single property
func validateProperty(fieldName string, value interface{}, prop types.ElicitProperty) error {
	switch prop.Type {
	case "string":
		str, ok := value.(string)
		if !ok {
			return &types.ValidationError{
				Field:   fieldName,
				Message: "expected string value",
				Code:    "type_mismatch",
			}
		}
		if prop.Validation != nil {
			if err := validateString(str, *prop.Validation); err != nil {
				err.(*types.ValidationError).Field = fieldName
				return err
			}
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return &types.ValidationError{
				Field:   fieldName,
				Message: "expected number value",
				Code:    "type_mismatch",
			}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return &types.ValidationError{
				Field:   fieldName,
				Message: "expected boolean value",
				Code:    "type_mismatch",
			}
		}
	}
	
	return nil
}

// generateElicitID generates a unique elicitation ID
func generateElicitID() string {
	return fmt.Sprintf("elicit_%d", time.Now().UnixNano())
}

// Helper function for string pointer
func stringPtr(s string) *string {
	return &s
}