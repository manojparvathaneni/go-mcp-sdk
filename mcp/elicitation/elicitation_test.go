package elicitation

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// Mock transport for testing
type mockElicitationTransport struct {
	sentNotifications       []types.ElicitNotification
	sentCancelNotifications []types.ElicitCancelNotification
	shouldFailSend          bool
	mu                      sync.Mutex
}

func (t *mockElicitationTransport) SendElicitNotification(notification types.ElicitNotification) error {
	if t.shouldFailSend {
		return errors.New("transport send failed")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sentNotifications = append(t.sentNotifications, notification)
	return nil
}

func (t *mockElicitationTransport) SendElicitCancelNotification(notification types.ElicitCancelNotification) error {
	if t.shouldFailSend {
		return errors.New("transport cancel failed")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sentCancelNotifications = append(t.sentCancelNotifications, notification)
	return nil
}

func (t *mockElicitationTransport) getSentNotifications() []types.ElicitNotification {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]types.ElicitNotification, len(t.sentNotifications))
	copy(result, t.sentNotifications)
	return result
}

func (t *mockElicitationTransport) getSentCancelNotifications() []types.ElicitCancelNotification {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]types.ElicitCancelNotification, len(t.sentCancelNotifications))
	copy(result, t.sentCancelNotifications)
	return result
}

// Mock client transport for testing
type mockElicitationClientTransport struct {
	sentResponses       []types.ElicitResponseNotification
	sentCancellations   []types.ElicitCancelNotification
	shouldFailSend      bool
	mu                  sync.Mutex
}

func (t *mockElicitationClientTransport) SendElicitResponse(notification types.ElicitResponseNotification) error {
	if t.shouldFailSend {
		return errors.New("transport response failed")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sentResponses = append(t.sentResponses, notification)
	return nil
}

func (t *mockElicitationClientTransport) SendElicitCancel(notification types.ElicitCancelNotification) error {
	if t.shouldFailSend {
		return errors.New("transport cancel failed")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sentCancellations = append(t.sentCancellations, notification)
	return nil
}

func (t *mockElicitationClientTransport) getSentResponses() []types.ElicitResponseNotification {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]types.ElicitResponseNotification, len(t.sentResponses))
	copy(result, t.sentResponses)
	return result
}

func (t *mockElicitationClientTransport) getSentCancellations() []types.ElicitCancelNotification {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]types.ElicitCancelNotification, len(t.sentCancellations))
	copy(result, t.sentCancellations)
	return result
}

// Mock UI handler for testing
type mockUIHandler struct {
	confirmationResponse bool
	inputResponse        string
	choiceResponse       interface{}
	multiChoiceResponse  []interface{}
	formResponse         map[string]interface{}
	customResponse       interface{}
	shouldError          bool
}

func (h *mockUIHandler) ShowConfirmation(ctx context.Context, req types.ElicitRequest) (bool, error) {
	if h.shouldError {
		return false, errors.New("UI error")
	}
	return h.confirmationResponse, nil
}

func (h *mockUIHandler) ShowInput(ctx context.Context, req types.ElicitRequest) (string, error) {
	if h.shouldError {
		return "", errors.New("UI error")
	}
	return h.inputResponse, nil
}

func (h *mockUIHandler) ShowChoice(ctx context.Context, req types.ElicitRequest, options []types.ElicitOption) (interface{}, error) {
	if h.shouldError {
		return nil, errors.New("UI error")
	}
	return h.choiceResponse, nil
}

func (h *mockUIHandler) ShowMultiChoice(ctx context.Context, req types.ElicitRequest, options []types.ElicitOption) ([]interface{}, error) {
	if h.shouldError {
		return nil, errors.New("UI error")
	}
	return h.multiChoiceResponse, nil
}

func (h *mockUIHandler) ShowForm(ctx context.Context, req types.ElicitRequest, schema types.ElicitSchema) (map[string]interface{}, error) {
	if h.shouldError {
		return nil, errors.New("UI error")
	}
	return h.formResponse, nil
}

func (h *mockUIHandler) ShowCustom(ctx context.Context, req types.ElicitRequest) (interface{}, error) {
	if h.shouldError {
		return nil, errors.New("UI error")
	}
	return h.customResponse, nil
}

func TestNewElicitationManager(t *testing.T) {
	transport := &mockElicitationTransport{}
	manager := NewElicitationManager(transport)

	if manager == nil {
		t.Fatal("Expected NewElicitationManager to return non-nil")
	}
	if manager.transport != transport {
		t.Error("Expected transport to be set")
	}
	if manager.responseTimeout != 30*time.Second {
		t.Error("Expected default timeout to be 30 seconds")
	}
}

func TestElicitationManager_SetResponseTimeout(t *testing.T) {
	transport := &mockElicitationTransport{}
	manager := NewElicitationManager(transport)

	timeout := 60 * time.Second
	manager.SetResponseTimeout(timeout)

	if manager.responseTimeout != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, manager.responseTimeout)
	}
}

func TestElicitationManager_Elicit_Success(t *testing.T) {
	transport := &mockElicitationTransport{}
	manager := NewElicitationManager(transport)
	manager.SetResponseTimeout(100 * time.Millisecond)

	// Create elicitation request
	req := types.ElicitRequest{
		Type:     types.ElicitTypeInput,
		Prompt:   "Enter your name:",
		Required: true,
	}

	// Start elicitation in goroutine
	resultChan := make(chan *types.ElicitResult)
	errChan := make(chan error)

	go func() {
		result, err := manager.Elicit(context.Background(), req)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// Wait a bit for elicitation to be set up
	time.Sleep(10 * time.Millisecond)

	// Verify notification was sent
	sentNotifications := transport.getSentNotifications()
	if len(sentNotifications) != 1 {
		t.Fatalf("Expected 1 notification sent, got %d", len(sentNotifications))
	}

	elicitID := sentNotifications[0].ElicitID

	// Send response
	response := types.ElicitResponseNotification{
		ElicitID: elicitID,
		Result: types.ElicitResult{
			Value:     "John Doe",
			Timestamp: time.Now().UnixNano(),
		},
	}

	err := manager.HandleElicitResponse(response)
	if err != nil {
		t.Fatalf("HandleElicitResponse failed: %v", err)
	}

	// Wait for result
	select {
	case result := <-resultChan:
		if result.Value != "John Doe" {
			t.Errorf("Expected value 'John Doe', got %v", result.Value)
		}
	case err := <-errChan:
		t.Fatalf("Elicit failed: %v", err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Elicit timed out")
	}
}

func TestElicitationManager_Elicit_Timeout(t *testing.T) {
	transport := &mockElicitationTransport{}
	manager := NewElicitationManager(transport)
	manager.SetResponseTimeout(50 * time.Millisecond)

	req := types.ElicitRequest{
		Type:   types.ElicitTypeConfirmation,
		Prompt: "Continue?",
	}

	_, err := manager.Elicit(context.Background(), req)
	if err == nil {
		t.Error("Expected timeout error")
	}

	timeoutErr, ok := err.(*types.ElicitTimeoutError)
	if !ok {
		t.Errorf("Expected ElicitTimeoutError, got %T", err)
	} else if timeoutErr.ElicitID == "" {
		t.Error("Expected ElicitID to be set")
	}

	// Verify cancellation notification was sent
	sentCancelNotifications := transport.getSentCancelNotifications()
	if len(sentCancelNotifications) != 1 {
		t.Errorf("Expected 1 cancel notification, got %d", len(sentCancelNotifications))
	}
}

func TestElicitationManager_Elicit_TransportError(t *testing.T) {
	transport := &mockElicitationTransport{shouldFailSend: true}
	manager := NewElicitationManager(transport)

	req := types.ElicitRequest{
		Type:   types.ElicitTypeInput,
		Prompt: "Enter value:",
	}

	_, err := manager.Elicit(context.Background(), req)
	if err == nil {
		t.Error("Expected transport error")
	}
	if !strings.Contains(err.Error(), "failed to send elicitation notification") {
		t.Errorf("Expected transport error message, got: %v", err)
	}
}

func TestElicitationManager_HandleElicitResponse_ValidationError(t *testing.T) {
	transport := &mockElicitationTransport{}
	manager := NewElicitationManager(transport)

	// Create request with schema validation
	minLength := 3
	req := types.ElicitRequest{
		Type:   types.ElicitTypeInput,
		Prompt: "Enter name (min 3 chars):",
		Schema: &types.ElicitSchema{
			Type: "string",
			Validation: &types.ElicitValidation{
				MinLength: &minLength,
			},
		},
	}

	// Start elicitation
	go manager.Elicit(context.Background(), req)
	time.Sleep(10 * time.Millisecond)

	sentNotifications := transport.getSentNotifications()
	if len(sentNotifications) == 0 {
		t.Fatal("No notifications sent")
	}
	elicitID := sentNotifications[0].ElicitID

	// Send invalid response (too short)
	response := types.ElicitResponseNotification{
		ElicitID: elicitID,
		Result: types.ElicitResult{
			Value:     "Jo", // Too short
			Timestamp: time.Now().UnixNano(),
		},
	}

	err := manager.HandleElicitResponse(response)
	if err == nil {
		t.Error("Expected validation error")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestElicitationManager_HandleElicitCancel(t *testing.T) {
	transport := &mockElicitationTransport{}
	manager := NewElicitationManager(transport)

	req := types.ElicitRequest{
		Type:   types.ElicitTypeConfirmation,
		Prompt: "Continue?",
	}

	// Start elicitation in goroutine
	errChan := make(chan error)
	go func() {
		_, err := manager.Elicit(context.Background(), req)
		errChan <- err
	}()

	time.Sleep(10 * time.Millisecond)
	sentNotifications := transport.getSentNotifications()
	if len(sentNotifications) == 0 {
		t.Fatal("No notifications sent")
	}
	elicitID := sentNotifications[0].ElicitID

	// Send cancellation
	reason := "user cancelled"
	cancelNotification := types.ElicitCancelNotification{
		ElicitID: elicitID,
		Reason:   &reason,
	}

	err := manager.HandleElicitCancel(cancelNotification)
	if err != nil {
		t.Fatalf("HandleElicitCancel failed: %v", err)
	}

	// Wait for elicitation to complete with error
	select {
	case err := <-errChan:
		cancelErr, ok := err.(*types.ElicitCancelledError)
		if !ok {
			t.Errorf("Expected ElicitCancelledError, got %T", err)
		} else if *cancelErr.Reason != reason {
			t.Errorf("Expected reason '%s', got '%s'", reason, *cancelErr.Reason)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected cancellation error")
	}
}

func TestElicitationManager_GetActiveElicitations(t *testing.T) {
	transport := &mockElicitationTransport{}
	manager := NewElicitationManager(transport)

	req := types.ElicitRequest{
		Type:   types.ElicitTypeInput,
		Prompt: "Enter value:",
	}

	// Start elicitation
	go manager.Elicit(context.Background(), req)
	time.Sleep(10 * time.Millisecond)

	active := manager.GetActiveElicitations()
	if len(active) != 1 {
		t.Errorf("Expected 1 active elicitation, got %d", len(active))
	}

	for elicitID, activeReq := range active {
		if activeReq.Prompt != req.Prompt {
			t.Error("Expected active request to match original")
		}
		if elicitID == "" {
			t.Error("Expected non-empty elicit ID")
		}
	}
}

func TestElicitationManager_CancelElicitation(t *testing.T) {
	transport := &mockElicitationTransport{}
	manager := NewElicitationManager(transport)

	req := types.ElicitRequest{
		Type:   types.ElicitTypeConfirmation,
		Prompt: "Continue?",
	}

	// Start elicitation
	go manager.Elicit(context.Background(), req)
	time.Sleep(10 * time.Millisecond)

	sentNotifications := transport.getSentNotifications()
	if len(sentNotifications) == 0 {
		t.Fatal("No notifications sent")
	}
	elicitID := sentNotifications[0].ElicitID

	// Cancel elicitation
	reason := "timeout"
	err := manager.CancelElicitation(elicitID, reason)
	if err != nil {
		t.Fatalf("CancelElicitation failed: %v", err)
	}

	// Verify cancellation notification was sent
	sentCancelNotifications := transport.getSentCancelNotifications()
	if len(sentCancelNotifications) != 1 {
		t.Errorf("Expected 1 cancel notification, got %d", len(sentCancelNotifications))
	}

	cancelNotif := sentCancelNotifications[0]
	if cancelNotif.ElicitID != elicitID {
		t.Error("Expected cancel notification to have correct elicit ID")
	}
	if *cancelNotif.Reason != reason {
		t.Error("Expected cancel notification to have correct reason")
	}
}

func TestNewElicitationClient(t *testing.T) {
	handler := &mockUIHandler{}
	transport := &mockElicitationClientTransport{}
	client := NewElicitationClient(handler, transport)

	if client == nil {
		t.Fatal("Expected NewElicitationClient to return non-nil")
	}
	if client.uiHandler != handler {
		t.Error("Expected UI handler to be set")
	}
	if client.transport != transport {
		t.Error("Expected transport to be set")
	}
}

func TestElicitationClient_SetAutoRespond(t *testing.T) {
	handler := &mockUIHandler{}
	transport := &mockElicitationClientTransport{}
	client := NewElicitationClient(handler, transport)

	client.SetAutoRespond(true)
	client.mu.RLock()
	autoRespond := client.autoRespond
	client.mu.RUnlock()

	if !autoRespond {
		t.Error("Expected autoRespond to be true")
	}
}

func TestElicitationClient_HandleElicitNotification_AutoRespond(t *testing.T) {
	handler := &mockUIHandler{}
	transport := &mockElicitationClientTransport{}
	client := NewElicitationClient(handler, transport)
	client.SetAutoRespond(true)

	notification := types.ElicitNotification{
		ElicitID: "test-elicit-1",
		Request: types.ElicitRequest{
			Type:   types.ElicitTypeConfirmation,
			Prompt: "Continue?",
		},
	}

	err := client.HandleElicitNotification(context.Background(), notification)
	if err != nil {
		t.Fatalf("HandleElicitNotification failed: %v", err)
	}

	// Verify auto-response was sent
	sentResponses := transport.getSentResponses()
	if len(sentResponses) != 1 {
		t.Errorf("Expected 1 response sent, got %d", len(sentResponses))
	}

	response := sentResponses[0]
	if response.ElicitID != notification.ElicitID {
		t.Error("Expected response to have correct elicit ID")
	}
	if response.Result.Value != true {
		t.Error("Expected auto-response to be true for confirmation")
	}
}

func TestElicitationClient_HandleElicitNotification_ManualMode(t *testing.T) {
	handler := &mockUIHandler{
		confirmationResponse: false,
	}
	transport := &mockElicitationClientTransport{}
	client := NewElicitationClient(handler, transport)

	notification := types.ElicitNotification{
		ElicitID: "test-elicit-2",
		Request: types.ElicitRequest{
			Type:   types.ElicitTypeConfirmation,
			Prompt: "Delete file?",
		},
	}

	err := client.HandleElicitNotification(context.Background(), notification)
	if err != nil {
		t.Fatalf("HandleElicitNotification failed: %v", err)
	}

	// Give time for async processing
	time.Sleep(50 * time.Millisecond)

	// Verify response was sent with UI handler result
	sentResponses := transport.getSentResponses()
	if len(sentResponses) != 1 {
		t.Errorf("Expected 1 response sent, got %d", len(sentResponses))
	}

	response := sentResponses[0]
	if response.Result.Value != false {
		t.Error("Expected response to be false from UI handler")
	}
}

func TestElicitationClient_ProcessElicitation_InputType(t *testing.T) {
	handler := &mockUIHandler{
		inputResponse: "test input",
	}
	transport := &mockElicitationClientTransport{}
	client := NewElicitationClient(handler, transport)

	notification := types.ElicitNotification{
		ElicitID: "test-elicit-3",
		Request: types.ElicitRequest{
			Type:   types.ElicitTypeInput,
			Prompt: "Enter name:",
		},
	}

	err := client.processElicitation(context.Background(), notification)
	if err != nil {
		t.Fatalf("processElicitation failed: %v", err)
	}

	// Verify response
	sentResponses := transport.getSentResponses()
	if len(sentResponses) != 1 {
		t.Errorf("Expected 1 response sent, got %d", len(sentResponses))
	}

	response := sentResponses[0]
	if response.Result.Value != "test input" {
		t.Error("Expected response to contain input value")
	}
}

func TestElicitationClient_ProcessElicitation_ChoiceType(t *testing.T) {
	handler := &mockUIHandler{
		choiceResponse: "option1",
	}
	transport := &mockElicitationClientTransport{}
	client := NewElicitationClient(handler, transport)

	options := []types.ElicitOption{
		{Label: "Option 1", Value: "option1"},
		{Label: "Option 2", Value: "option2"},
	}

	notification := types.ElicitNotification{
		ElicitID: "test-elicit-4",
		Request: types.ElicitRequest{
			Type:   types.ElicitTypeChoice,
			Prompt: "Select option:",
			Schema: &types.ElicitSchema{
				Type:    "string",
				Options: options,
			},
		},
	}

	err := client.processElicitation(context.Background(), notification)
	if err != nil {
		t.Fatalf("processElicitation failed: %v", err)
	}

	// Verify response
	sentResponses := transport.getSentResponses()
	if len(sentResponses) != 1 {
		t.Errorf("Expected 1 response sent, got %d", len(sentResponses))
	}

	response := sentResponses[0]
	if response.Result.Value != "option1" {
		t.Error("Expected response to contain choice value")
	}
}

func TestElicitationClient_ProcessElicitation_FormType(t *testing.T) {
	formData := map[string]interface{}{
		"name": "John",
		"age":  30.0,
	}

	handler := &mockUIHandler{
		formResponse: formData,
	}
	transport := &mockElicitationClientTransport{}
	client := NewElicitationClient(handler, transport)

	properties := map[string]types.ElicitProperty{
		"name": {Type: "string", Label: "Name"},
		"age":  {Type: "number", Label: "Age"},
	}

	notification := types.ElicitNotification{
		ElicitID: "test-elicit-5",
		Request: types.ElicitRequest{
			Type:   types.ElicitTypeForm,
			Prompt: "Fill form:",
			Schema: &types.ElicitSchema{
				Type:       "object",
				Properties: properties,
			},
		},
	}

	err := client.processElicitation(context.Background(), notification)
	if err != nil {
		t.Fatalf("processElicitation failed: %v", err)
	}

	// Verify response
	sentResponses := transport.getSentResponses()
	if len(sentResponses) != 1 {
		t.Errorf("Expected 1 response sent, got %d", len(sentResponses))
	}

	response := sentResponses[0]
	responseForm, ok := response.Result.Value.(map[string]interface{})
	if !ok {
		t.Fatal("Expected form response to be map")
	}
	if responseForm["name"] != "John" {
		t.Error("Expected name field to be preserved")
	}
}

func TestElicitationClient_ProcessElicitation_UIError(t *testing.T) {
	handler := &mockUIHandler{
		shouldError: true,
	}
	transport := &mockElicitationClientTransport{}
	client := NewElicitationClient(handler, transport)

	notification := types.ElicitNotification{
		ElicitID: "test-elicit-6",
		Request: types.ElicitRequest{
			Type:   types.ElicitTypeInput,
			Prompt: "Enter value:",
		},
	}

	err := client.processElicitation(context.Background(), notification)
	if err != nil {
		t.Fatalf("processElicitation failed: %v", err)
	}

	// Should send cancelled response
	sentResponses := transport.getSentResponses()
	if len(sentResponses) != 1 {
		t.Errorf("Expected 1 response sent, got %d", len(sentResponses))
	}

	response := sentResponses[0]
	if !response.Result.Cancelled {
		t.Error("Expected response to be cancelled on UI error")
	}
}

func TestElicitationClient_CancelElicitation(t *testing.T) {
	handler := &mockUIHandler{}
	transport := &mockElicitationClientTransport{}
	client := NewElicitationClient(handler, transport)

	elicitID := "test-elicit-7"
	reason := "user requested"

	err := client.CancelElicitation(elicitID, reason)
	if err != nil {
		t.Fatalf("CancelElicitation failed: %v", err)
	}

	// Verify cancellation was sent
	sentCancellations := transport.getSentCancellations()
	if len(sentCancellations) != 1 {
		t.Errorf("Expected 1 cancellation sent, got %d", len(sentCancellations))
	}

	cancellation := sentCancellations[0]
	if cancellation.ElicitID != elicitID {
		t.Error("Expected cancellation to have correct elicit ID")
	}
	if *cancellation.Reason != reason {
		t.Error("Expected cancellation to have correct reason")
	}
}

func TestValidateElicitResponse_Boolean(t *testing.T) {
	schema := types.ElicitSchema{
		Type: "boolean",
	}

	// Valid boolean
	result := types.ElicitResult{Value: true}
	err := validateElicitResponse(result, schema)
	if err != nil {
		t.Errorf("Expected boolean validation to pass: %v", err)
	}

	// Invalid type
	result = types.ElicitResult{Value: "not a boolean"}
	err = validateElicitResponse(result, schema)
	if err == nil {
		t.Error("Expected boolean validation to fail for string")
	}
}

func TestValidateElicitResponse_String(t *testing.T) {
	minLength := 3
	maxLength := 10
	schema := types.ElicitSchema{
		Type: "string",
		Validation: &types.ElicitValidation{
			MinLength: &minLength,
			MaxLength: &maxLength,
		},
	}

	// Valid string
	result := types.ElicitResult{Value: "hello"}
	err := validateElicitResponse(result, schema)
	if err != nil {
		t.Errorf("Expected string validation to pass: %v", err)
	}

	// Too short
	result = types.ElicitResult{Value: "hi"}
	err = validateElicitResponse(result, schema)
	if err == nil {
		t.Error("Expected string validation to fail for too short string")
	}

	// Too long
	result = types.ElicitResult{Value: "this is way too long"}
	err = validateElicitResponse(result, schema)
	if err == nil {
		t.Error("Expected string validation to fail for too long string")
	}
}

func TestValidateElicitResponse_Object(t *testing.T) {
	properties := map[string]types.ElicitProperty{
		"name": {Type: "string", Required: true},
		"age":  {Type: "number"},
	}
	schema := types.ElicitSchema{
		Type:       "object",
		Properties: properties,
		Required:   []string{"name"},
	}

	// Valid object
	result := types.ElicitResult{
		Value: map[string]interface{}{
			"name": "John",
			"age":  30.0,
		},
	}
	err := validateElicitResponse(result, schema)
	if err != nil {
		t.Errorf("Expected object validation to pass: %v", err)
	}

	// Missing required field
	result = types.ElicitResult{
		Value: map[string]interface{}{
			"age": 30.0,
		},
	}
	err = validateElicitResponse(result, schema)
	if err == nil {
		t.Error("Expected object validation to fail for missing required field")
	}
}

func TestValidateElicitResponse_Cancelled(t *testing.T) {
	schema := types.ElicitSchema{
		Type: "string",
	}

	// Cancelled responses should not be validated
	result := types.ElicitResult{
		Cancelled: true,
		Value:     123, // Wrong type, but should be ignored
	}
	err := validateElicitResponse(result, schema)
	if err != nil {
		t.Errorf("Expected cancelled response validation to pass: %v", err)
	}
}

func TestGenerateElicitID(t *testing.T) {
	id1 := generateElicitID()
	time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	id2 := generateElicitID()

	if id1 == id2 {
		t.Error("Expected different elicit IDs")
	}
	if !strings.HasPrefix(id1, "elicit_") {
		t.Error("Expected elicit ID to have 'elicit_' prefix")
	}
	if !strings.HasPrefix(id2, "elicit_") {
		t.Error("Expected elicit ID to have 'elicit_' prefix")
	}
}

func TestNewConsoleElicitationUI(t *testing.T) {
	ui := NewConsoleElicitationUI()
	if ui == nil {
		t.Fatal("Expected NewConsoleElicitationUI to return non-nil")
	}
}

// Test console UI methods (they use auto-responses for demo)
func TestConsoleElicitationUI_ShowConfirmation(t *testing.T) {
	ui := NewConsoleElicitationUI()
	req := types.ElicitRequest{Prompt: "Continue?"}

	result, err := ui.ShowConfirmation(context.Background(), req)
	if err != nil {
		t.Fatalf("ShowConfirmation failed: %v", err)
	}
	if !result {
		t.Error("Expected auto-confirmation to return true")
	}
}

func TestConsoleElicitationUI_ShowInput(t *testing.T) {
	ui := NewConsoleElicitationUI()
	req := types.ElicitRequest{Prompt: "Enter name:"}

	result, err := ui.ShowInput(context.Background(), req)
	if err != nil {
		t.Fatalf("ShowInput failed: %v", err)
	}
	if result != "user input" {
		t.Errorf("Expected default input 'user input', got '%s'", result)
	}
}

func TestConsoleElicitationUI_ShowChoice(t *testing.T) {
	ui := NewConsoleElicitationUI()
	req := types.ElicitRequest{Prompt: "Select option:"}
	options := []types.ElicitOption{
		{Label: "Option 1", Value: "opt1"},
		{Label: "Option 2", Value: "opt2"},
	}

	result, err := ui.ShowChoice(context.Background(), req, options)
	if err != nil {
		t.Fatalf("ShowChoice failed: %v", err)
	}
	if result != "opt1" {
		t.Errorf("Expected first option value 'opt1', got %v", result)
	}
}

func TestConsoleElicitationUI_ShowChoice_NoOptions(t *testing.T) {
	ui := NewConsoleElicitationUI()
	req := types.ElicitRequest{Prompt: "Select option:"}
	options := []types.ElicitOption{}

	_, err := ui.ShowChoice(context.Background(), req, options)
	if err == nil {
		t.Error("Expected error for no options")
	}
}