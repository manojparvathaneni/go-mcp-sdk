package elicitation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// ElicitationUIHandler defines the interface for handling user interactions
type ElicitationUIHandler interface {
	// ShowConfirmation shows a yes/no confirmation dialog
	ShowConfirmation(ctx context.Context, req types.ElicitRequest) (bool, error)
	
	// ShowInput shows a text input dialog
	ShowInput(ctx context.Context, req types.ElicitRequest) (string, error)
	
	// ShowChoice shows a single choice selection dialog
	ShowChoice(ctx context.Context, req types.ElicitRequest, options []types.ElicitOption) (interface{}, error)
	
	// ShowMultiChoice shows a multiple choice selection dialog
	ShowMultiChoice(ctx context.Context, req types.ElicitRequest, options []types.ElicitOption) ([]interface{}, error)
	
	// ShowForm shows a structured form dialog
	ShowForm(ctx context.Context, req types.ElicitRequest, schema types.ElicitSchema) (map[string]interface{}, error)
	
	// ShowCustom shows a custom elicitation UI (for extensibility)
	ShowCustom(ctx context.Context, req types.ElicitRequest) (interface{}, error)
}

// ElicitationClient handles elicitation requests from servers
type ElicitationClient struct {
	mu          sync.RWMutex
	uiHandler   ElicitationUIHandler
	transport   ElicitationClientTransport
	autoRespond bool // For testing - automatically respond to elicitations
}

// ElicitationClientTransport defines the transport interface for client-side elicitation
type ElicitationClientTransport interface {
	SendElicitResponse(notification types.ElicitResponseNotification) error
	SendElicitCancel(notification types.ElicitCancelNotification) error
}

// NewElicitationClient creates a new elicitation client
func NewElicitationClient(uiHandler ElicitationUIHandler, transport ElicitationClientTransport) *ElicitationClient {
	return &ElicitationClient{
		uiHandler: uiHandler,
		transport: transport,
	}
}

// SetAutoRespond sets whether to automatically respond to elicitations (for testing)
func (c *ElicitationClient) SetAutoRespond(auto bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoRespond = auto
}

// HandleElicitNotification handles an elicitation notification from the server
func (c *ElicitationClient) HandleElicitNotification(ctx context.Context, notification types.ElicitNotification) error {
	c.mu.RLock()
	autoRespond := c.autoRespond
	c.mu.RUnlock()
	
	// Handle auto-respond for testing
	if autoRespond {
		return c.handleAutoResponse(notification)
	}
	
	// Handle the elicitation in a separate goroutine to avoid blocking
	go func() {
		if err := c.processElicitation(ctx, notification); err != nil {
			// Log error or send error response
			fmt.Printf("Error processing elicitation %s: %v\n", notification.ElicitID, err)
			
			// Send cancellation response
			cancelResp := types.ElicitResponseNotification{
				ElicitID: notification.ElicitID,
				Result: types.ElicitResult{
					Cancelled:    true,
					CancelReason: stringPtr(err.Error()),
					Timestamp:    getCurrentTimestamp(),
				},
			}
			c.transport.SendElicitResponse(cancelResp)
		}
	}()
	
	return nil
}

// processElicitation processes an elicitation request
func (c *ElicitationClient) processElicitation(ctx context.Context, notification types.ElicitNotification) error {
	req := notification.Request
	
	var value interface{}
	var err error
	
	// Handle different elicitation types
	switch req.Type {
	case types.ElicitTypeConfirmation:
		value, err = c.uiHandler.ShowConfirmation(ctx, req)
		
	case types.ElicitTypeInput:
		value, err = c.uiHandler.ShowInput(ctx, req)
		
	case types.ElicitTypePassword:
		// Password is handled like input but with masking in UI
		value, err = c.uiHandler.ShowInput(ctx, req)
		
	case types.ElicitTypeChoice:
		if req.Schema != nil && len(req.Schema.Options) > 0 {
			value, err = c.uiHandler.ShowChoice(ctx, req, req.Schema.Options)
		} else {
			err = fmt.Errorf("choice elicitation requires options")
		}
		
	case types.ElicitTypeMultiChoice:
		if req.Schema != nil && len(req.Schema.Options) > 0 {
			value, err = c.uiHandler.ShowMultiChoice(ctx, req, req.Schema.Options)
		} else {
			err = fmt.Errorf("multi-choice elicitation requires options")
		}
		
	case types.ElicitTypeForm:
		if req.Schema != nil {
			value, err = c.uiHandler.ShowForm(ctx, req, *req.Schema)
		} else {
			err = fmt.Errorf("form elicitation requires schema")
		}
		
	case types.ElicitTypeNumber:
		// Handle as input but validate as number
		strValue, inputErr := c.uiHandler.ShowInput(ctx, req)
		if inputErr != nil {
			err = inputErr
		} else {
			// In a real implementation, you'd parse the string to a number
			value = strValue
		}
		
	case types.ElicitTypeDate:
		// Handle as input but with date picker in UI
		value, err = c.uiHandler.ShowInput(ctx, req)
		
	default:
		// Handle custom types
		value, err = c.uiHandler.ShowCustom(ctx, req)
	}
	
	// Prepare response
	result := types.ElicitResult{
		Timestamp: getCurrentTimestamp(),
	}
	
	if err != nil {
		result.Cancelled = true
		result.CancelReason = stringPtr(err.Error())
	} else {
		result.Value = value
		
		// Validate the response if schema is provided
		if req.Schema != nil {
			if validationErr := validateElicitResponse(result, *req.Schema); validationErr != nil {
				result.Cancelled = true
				result.CancelReason = stringPtr("validation failed: " + validationErr.Error())
			}
		}
	}
	
	// Send response
	response := types.ElicitResponseNotification{
		ElicitID: notification.ElicitID,
		Result:   result,
	}
	
	return c.transport.SendElicitResponse(response)
}

// handleAutoResponse automatically responds to elicitations for testing
func (c *ElicitationClient) handleAutoResponse(notification types.ElicitNotification) error {
	req := notification.Request
	var value interface{}
	
	// Generate appropriate auto-responses based on type
	switch req.Type {
	case types.ElicitTypeConfirmation:
		value = true // Auto-confirm
		
	case types.ElicitTypeInput, types.ElicitTypePassword:
		value = "auto-response"
		
	case types.ElicitTypeChoice:
		if req.Schema != nil && len(req.Schema.Options) > 0 {
			value = req.Schema.Options[0].Value // Select first option
		} else {
			value = "auto-choice"
		}
		
	case types.ElicitTypeMultiChoice:
		if req.Schema != nil && len(req.Schema.Options) > 0 {
			value = []interface{}{req.Schema.Options[0].Value} // Select first option
		} else {
			value = []interface{}{"auto-choice"}
		}
		
	case types.ElicitTypeForm:
		// Create a simple form response
		formValue := make(map[string]interface{})
		if req.Schema != nil {
			for fieldName, prop := range req.Schema.Properties {
				switch prop.Type {
				case "string":
					formValue[fieldName] = "auto-value"
				case "number":
					formValue[fieldName] = 42.0
				case "boolean":
					formValue[fieldName] = true
				}
			}
		}
		value = formValue
		
	case types.ElicitTypeNumber:
		value = 42.0
		
	case types.ElicitTypeDate:
		value = "2024-01-01"
		
	default:
		value = "auto-response"
	}
	
	// Send auto-response
	response := types.ElicitResponseNotification{
		ElicitID: notification.ElicitID,
		Result: types.ElicitResult{
			Value:     value,
			Timestamp: getCurrentTimestamp(),
		},
	}
	
	return c.transport.SendElicitResponse(response)
}

// CancelElicitation cancels an ongoing elicitation
func (c *ElicitationClient) CancelElicitation(elicitID string, reason string) error {
	notification := types.ElicitCancelNotification{
		ElicitID: elicitID,
		Reason:   &reason,
	}
	
	return c.transport.SendElicitCancel(notification)
}

// ConsoleElicitationUI provides a simple console-based UI for elicitations
type ConsoleElicitationUI struct{}

// NewConsoleElicitationUI creates a new console-based elicitation UI
func NewConsoleElicitationUI() *ConsoleElicitationUI {
	return &ConsoleElicitationUI{}
}

func (ui *ConsoleElicitationUI) ShowConfirmation(ctx context.Context, req types.ElicitRequest) (bool, error) {
	fmt.Printf("🤔 %s (y/n): ", req.Prompt)
	
	// In a real implementation, you'd read from stdin
	// For demo purposes, we'll auto-confirm
	fmt.Println("y (auto-confirmed)")
	return true, nil
}

func (ui *ConsoleElicitationUI) ShowInput(ctx context.Context, req types.ElicitRequest) (string, error) {
	if req.Title != nil {
		fmt.Printf("📝 %s\n", *req.Title)
	}
	fmt.Printf("🔤 %s: ", req.Prompt)
	
	// In a real implementation, you'd read from stdin
	// For demo purposes, we'll return a default value
	defaultValue := "user input"
	if req.Default != nil {
		if str, ok := req.Default.(string); ok {
			defaultValue = str
		}
	}
	
	fmt.Printf("%s (auto-filled)\n", defaultValue)
	return defaultValue, nil
}

func (ui *ConsoleElicitationUI) ShowChoice(ctx context.Context, req types.ElicitRequest, options []types.ElicitOption) (interface{}, error) {
	if req.Title != nil {
		fmt.Printf("🎯 %s\n", *req.Title)
	}
	fmt.Printf("📋 %s\n", req.Prompt)
	
	for i, option := range options {
		fmt.Printf("  %d. %s", i+1, option.Label)
		if option.Description != nil {
			fmt.Printf(" - %s", *option.Description)
		}
		fmt.Println()
	}
	
	// Auto-select first option
	if len(options) > 0 {
		fmt.Printf("Selected: %s (auto-selected)\n", options[0].Label)
		return options[0].Value, nil
	}
	
	return nil, fmt.Errorf("no options available")
}

func (ui *ConsoleElicitationUI) ShowMultiChoice(ctx context.Context, req types.ElicitRequest, options []types.ElicitOption) ([]interface{}, error) {
	if req.Title != nil {
		fmt.Printf("🎯 %s\n", *req.Title)
	}
	fmt.Printf("☑️  %s (multiple selection)\n", req.Prompt)
	
	for i, option := range options {
		fmt.Printf("  %d. %s", i+1, option.Label)
		if option.Description != nil {
			fmt.Printf(" - %s", *option.Description)
		}
		fmt.Println()
	}
	
	// Auto-select first option
	if len(options) > 0 {
		fmt.Printf("Selected: %s (auto-selected)\n", options[0].Label)
		return []interface{}{options[0].Value}, nil
	}
	
	return []interface{}{}, nil
}

func (ui *ConsoleElicitationUI) ShowForm(ctx context.Context, req types.ElicitRequest, schema types.ElicitSchema) (map[string]interface{}, error) {
	if req.Title != nil {
		fmt.Printf("📄 %s\n", *req.Title)
	}
	fmt.Printf("📝 %s\n", req.Prompt)
	
	result := make(map[string]interface{})
	
	for fieldName, prop := range schema.Properties {
		fmt.Printf("  %s (%s): ", prop.Label, prop.Type)
		
		// Auto-fill based on type
		switch prop.Type {
		case "string":
			defaultValue := "auto-value"
			if prop.Default != nil {
				if str, ok := prop.Default.(string); ok {
					defaultValue = str
				}
			}
			fmt.Printf("%s (auto-filled)\n", defaultValue)
			result[fieldName] = defaultValue
			
		case "number":
			defaultValue := 42.0
			if prop.Default != nil {
				if num, ok := prop.Default.(float64); ok {
					defaultValue = num
				}
			}
			fmt.Printf("%.1f (auto-filled)\n", defaultValue)
			result[fieldName] = defaultValue
			
		case "boolean":
			defaultValue := true
			if prop.Default != nil {
				if b, ok := prop.Default.(bool); ok {
					defaultValue = b
				}
			}
			fmt.Printf("%t (auto-filled)\n", defaultValue)
			result[fieldName] = defaultValue
		}
	}
	
	return result, nil
}

func (ui *ConsoleElicitationUI) ShowCustom(ctx context.Context, req types.ElicitRequest) (interface{}, error) {
	fmt.Printf("🔧 Custom elicitation: %s\n", req.Prompt)
	fmt.Println("Returning default response (auto-handled)")
	return "custom-response", nil
}

// getCurrentTimestamp returns the current timestamp
func getCurrentTimestamp() int64 {
	return time.Now().UnixNano()
}