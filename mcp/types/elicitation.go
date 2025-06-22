package types

// Elicitation types for requesting additional information from users

// ElicitRequest represents a request for additional information from the user
type ElicitRequest struct {
	// Type of elicitation (confirmation, input, choice, etc.)
	Type string `json:"type"`
	
	// Human-readable prompt to show the user
	Prompt string `json:"prompt"`
	
	// Optional title for the elicitation dialog
	Title *string `json:"title,omitempty"`
	
	// Optional structured schema for the expected response
	Schema *ElicitSchema `json:"schema,omitempty"`
	
	// Optional default value
	Default interface{} `json:"default,omitempty"`
	
	// Whether this elicitation is required (user must respond)
	Required bool `json:"required"`
	
	// Timeout for the elicitation in seconds (0 = no timeout)
	Timeout *int `json:"timeout,omitempty"`
	
	// Additional metadata for the elicitation
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ElicitResult represents the user's response to an elicitation request
type ElicitResult struct {
	// Whether the user provided a response or cancelled
	Cancelled bool `json:"cancelled"`
	
	// The user's response value (null if cancelled)
	Value interface{} `json:"value,omitempty"`
	
	// Optional reason for cancellation
	CancelReason *string `json:"cancelReason,omitempty"`
	
	// Whether the response was modified by validation
	Modified bool `json:"modified,omitempty"`
	
	// Timestamp of the response
	Timestamp int64 `json:"timestamp"`
}

// ElicitSchema defines the structure of expected elicitation responses
type ElicitSchema struct {
	// Type of the expected value
	Type string `json:"type"`
	
	// For 'choice' type: available options
	Options []ElicitOption `json:"options,omitempty"`
	
	// For 'object' type: properties schema
	Properties map[string]ElicitProperty `json:"properties,omitempty"`
	
	// Required properties for object types
	Required []string `json:"required,omitempty"`
	
	// Validation rules
	Validation *ElicitValidation `json:"validation,omitempty"`
}

// ElicitOption represents a choice option for selection-type elicitations
type ElicitOption struct {
	// Display label for the option
	Label string `json:"label"`
	
	// Value to return if this option is selected
	Value interface{} `json:"value"`
	
	// Optional description of the option
	Description *string `json:"description,omitempty"`
	
	// Whether this option is disabled
	Disabled bool `json:"disabled,omitempty"`
}

// ElicitProperty defines a property in an object schema
type ElicitProperty struct {
	// Type of the property
	Type string `json:"type"`
	
	// Human-readable label
	Label string `json:"label"`
	
	// Optional description
	Description *string `json:"description,omitempty"`
	
	// Default value
	Default interface{} `json:"default,omitempty"`
	
	// Whether this property is required
	Required bool `json:"required,omitempty"`
	
	// Validation rules for this property
	Validation *ElicitValidation `json:"validation,omitempty"`
	
	// For choice properties: available options
	Options []ElicitOption `json:"options,omitempty"`
}

// ElicitValidation defines validation rules for elicitation responses
type ElicitValidation struct {
	// Minimum length for string types
	MinLength *int `json:"minLength,omitempty"`
	
	// Maximum length for string types
	MaxLength *int `json:"maxLength,omitempty"`
	
	// Pattern (regex) for string validation
	Pattern *string `json:"pattern,omitempty"`
	
	// Minimum value for numeric types
	Minimum *float64 `json:"minimum,omitempty"`
	
	// Maximum value for numeric types
	Maximum *float64 `json:"maximum,omitempty"`
	
	// Custom validation message
	Message *string `json:"message,omitempty"`
}

// ElicitationCapability represents the elicitation capability
type ElicitationCapability struct {
	// Supported elicitation types
	SupportedTypes []string `json:"supportedTypes,omitempty"`
	
	// Whether structured schemas are supported
	SupportsSchema bool `json:"supportsSchema,omitempty"`
	
	// Whether timeouts are supported
	SupportsTimeout bool `json:"supportsTimeout,omitempty"`
}

// Predefined elicitation types
const (
	ElicitTypeConfirmation = "confirmation"  // Yes/No confirmation
	ElicitTypeInput        = "input"         // Free text input
	ElicitTypeChoice       = "choice"        // Single choice from options
	ElicitTypeMultiChoice  = "multi_choice"  // Multiple choices from options
	ElicitTypeForm         = "form"          // Structured form with multiple fields
	ElicitTypeFile         = "file"          // File selection
	ElicitTypePassword     = "password"      // Masked password input
	ElicitTypeNumber       = "number"        // Numeric input
	ElicitTypeDate         = "date"          // Date selection
)

// ElicitNotification is sent by servers to request elicitation
type ElicitNotification struct {
	// Unique ID for this elicitation request
	ElicitID string `json:"elicitId"`
	
	// The elicitation request details
	Request ElicitRequest `json:"request"`
}

// ElicitResponseNotification is sent by clients with the user's response
type ElicitResponseNotification struct {
	// ID of the elicitation being responded to
	ElicitID string `json:"elicitId"`
	
	// The user's response
	Result ElicitResult `json:"result"`
}

// ElicitCancelNotification is sent to cancel an ongoing elicitation
type ElicitCancelNotification struct {
	// ID of the elicitation to cancel
	ElicitID string `json:"elicitId"`
	
	// Reason for cancellation
	Reason *string `json:"reason,omitempty"`
}

// Helper functions for creating common elicitation requests

// NewConfirmationElicit creates a yes/no confirmation elicitation
func NewConfirmationElicit(prompt string) ElicitRequest {
	return ElicitRequest{
		Type:     ElicitTypeConfirmation,
		Prompt:   prompt,
		Required: true,
		Schema: &ElicitSchema{
			Type: "boolean",
		},
	}
}

// NewInputElicit creates a text input elicitation
func NewInputElicit(prompt string, required bool) ElicitRequest {
	return ElicitRequest{
		Type:     ElicitTypeInput,
		Prompt:   prompt,
		Required: required,
		Schema: &ElicitSchema{
			Type: "string",
		},
	}
}

// NewChoiceElicit creates a single choice elicitation
func NewChoiceElicit(prompt string, options []ElicitOption) ElicitRequest {
	return ElicitRequest{
		Type:     ElicitTypeChoice,
		Prompt:   prompt,
		Required: true,
		Schema: &ElicitSchema{
			Type:    "string",
			Options: options,
		},
	}
}

// NewFormElicit creates a structured form elicitation
func NewFormElicit(title, prompt string, properties map[string]ElicitProperty, required []string) ElicitRequest {
	return ElicitRequest{
		Type:     ElicitTypeForm,
		Title:    &title,
		Prompt:   prompt,
		Required: true,
		Schema: &ElicitSchema{
			Type:       "object",
			Properties: properties,
			Required:   required,
		},
	}
}

// ValidationError represents an elicitation validation error
type ValidationError struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}

// ElicitTimeoutError represents an elicitation timeout
type ElicitTimeoutError struct {
	ElicitID string `json:"elicitId"`
	Timeout  int    `json:"timeout"`
}

func (e *ElicitTimeoutError) Error() string {
	return "elicitation timed out after " + string(rune(e.Timeout)) + " seconds"
}

// ElicitCancelledError represents a cancelled elicitation
type ElicitCancelledError struct {
	ElicitID string  `json:"elicitId"`
	Reason   *string `json:"reason,omitempty"`
}

func (e *ElicitCancelledError) Error() string {
	if e.Reason != nil {
		return "elicitation cancelled: " + *e.Reason
	}
	return "elicitation cancelled by user"
}