package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// StructuredToolFunc represents a tool function that returns structured content
type StructuredToolFunc func(ctx context.Context, arguments json.RawMessage) (interface{}, error)

// StructuredToolResult creates a tool result with structured content
func StructuredToolResult(structuredContent interface{}) *types.CallToolResult {
	return &types.CallToolResult{
		StructuredContent: &structuredContent,
		IsError:           false,
	}
}

// StructuredToolError creates an error tool result with structured error content
func StructuredToolError(errorContent interface{}) *types.CallToolResult {
	return &types.CallToolResult{
		StructuredContent: &errorContent,
		IsError:           true,
	}
}

// ValidateStructuredOutput validates structured content against an output schema
func ValidateStructuredOutput(content interface{}, schema map[string]interface{}) error {
	return validateAgainstSchema(content, schema, "")
}

// JSONSchemaValidator provides JSON schema validation for structured content
type JSONSchemaValidator struct {
	strict bool // Whether to enforce strict validation
}

// NewJSONSchemaValidator creates a new JSON schema validator
func NewJSONSchemaValidator(strict bool) *JSONSchemaValidator {
	return &JSONSchemaValidator{strict: strict}
}

// Validate validates content against a JSON schema
func (v *JSONSchemaValidator) Validate(content interface{}, schema map[string]interface{}) error {
	return validateAgainstSchema(content, schema, "")
}

// validateAgainstSchema validates content against a JSON schema
func validateAgainstSchema(content interface{}, schema map[string]interface{}, path string) error {
	// Get the type from schema
	schemaType, ok := schema["type"].(string)
	if !ok {
		return fmt.Errorf("schema at %s missing 'type' field", path)
	}

	// Validate based on type
	switch schemaType {
	case "object":
		return validateSchemaObject(content, schema, path)
	case "array":
		return validateSchemaArray(content, schema, path)
	case "string":
		return validateSchemaString(content, schema, path)
	case "number":
		return validateSchemaNumber(content, schema, path)
	case "integer":
		return validateSchemaInteger(content, schema, path)
	case "boolean":
		return validateSchemaBoolean(content, schema, path)
	case "null":
		return validateSchemaNull(content, path)
	default:
		return fmt.Errorf("unsupported schema type '%s' at %s", schemaType, path)
	}
}

// validateObject validates an object against an object schema
func validateSchemaObject(content interface{}, schema map[string]interface{}, path string) error {
	obj, ok := content.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected object at %s, got %T", path, content)
	}

	// Check required properties
	if required, exists := schema["required"]; exists {
		if requiredArray, ok := required.([]interface{}); ok {
			for _, reqField := range requiredArray {
				if fieldName, ok := reqField.(string); ok {
					if _, exists := obj[fieldName]; !exists {
						return fmt.Errorf("required field '%s' missing at %s", fieldName, path)
					}
				}
			}
		}
	}

	// Validate properties
	if properties, exists := schema["properties"]; exists {
		if propsMap, ok := properties.(map[string]interface{}); ok {
			for fieldName, value := range obj {
				if fieldSchema, exists := propsMap[fieldName]; exists {
					if fieldSchemaMap, ok := fieldSchema.(map[string]interface{}); ok {
						fieldPath := path + "." + fieldName
						if path == "" {
							fieldPath = fieldName
						}
						if err := validateAgainstSchema(value, fieldSchemaMap, fieldPath); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

// validateArray validates an array against an array schema
func validateSchemaArray(content interface{}, schema map[string]interface{}, path string) error {
	arr, ok := content.([]interface{})
	if !ok {
		return fmt.Errorf("expected array at %s, got %T", path, content)
	}

	// Check minimum items
	if minItems, exists := schema["minItems"]; exists {
		var minVal int
		switch v := minItems.(type) {
		case float64:
			minVal = int(v)
		case int:
			minVal = v
		default:
			return fmt.Errorf("invalid minItems type at %s: expected number, got %T", path, minItems)
		}
		if len(arr) < minVal {
			return fmt.Errorf("array at %s has %d items, minimum %d required", path, len(arr), minVal)
		}
	}

	// Check maximum items
	if maxItems, exists := schema["maxItems"]; exists {
		var maxVal int
		switch v := maxItems.(type) {
		case float64:
			maxVal = int(v)
		case int:
			maxVal = v
		default:
			return fmt.Errorf("invalid maxItems type at %s: expected number, got %T", path, maxItems)
		}
		if len(arr) > maxVal {
			return fmt.Errorf("array at %s has %d items, maximum %d allowed", path, len(arr), maxVal)
		}
	}

	// Validate items
	if items, exists := schema["items"]; exists {
		if itemSchema, ok := items.(map[string]interface{}); ok {
			for i, item := range arr {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				if err := validateAgainstSchema(item, itemSchema, itemPath); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateString validates a string against a string schema
func validateSchemaString(content interface{}, schema map[string]interface{}, path string) error {
	str, ok := content.(string)
	if !ok {
		return fmt.Errorf("expected string at %s, got %T", path, content)
	}

	// Check minimum length
	if minLength, exists := schema["minLength"]; exists {
		var minVal int
		switch v := minLength.(type) {
		case float64:
			minVal = int(v)
		case int:
			minVal = v
		default:
			return fmt.Errorf("invalid minLength type at %s: expected number, got %T", path, minLength)
		}
		if len(str) < minVal {
			return fmt.Errorf("string at %s has length %d, minimum %d required", path, len(str), minVal)
		}
	}

	// Check maximum length
	if maxLength, exists := schema["maxLength"]; exists {
		var maxVal int
		switch v := maxLength.(type) {
		case float64:
			maxVal = int(v)
		case int:
			maxVal = v
		default:
			return fmt.Errorf("invalid maxLength type at %s: expected number, got %T", path, maxLength)
		}
		if len(str) > maxVal {
			return fmt.Errorf("string at %s has length %d, maximum %d allowed", path, len(str), maxVal)
		}
	}

	// Check pattern (basic implementation)
	if pattern, exists := schema["pattern"]; exists {
		if patternStr, ok := pattern.(string); ok {
			// In a real implementation, you'd use regexp.MatchString
			_ = patternStr // Placeholder
		}
	}

	// Check enum values
	if enum, exists := schema["enum"]; exists {
		if enumArray, ok := enum.([]interface{}); ok {
			found := false
			for _, enumValue := range enumArray {
				if enumStr, ok := enumValue.(string); ok && enumStr == str {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("string at %s value '%s' not in allowed enum values", path, str)
			}
		}
	}

	return nil
}

// validateNumber validates a number against a number schema
func validateSchemaNumber(content interface{}, schema map[string]interface{}, path string) error {
	var num float64
	var ok bool

	switch v := content.(type) {
	case float64:
		num = v
		ok = true
	case float32:
		num = float64(v)
		ok = true
	case int:
		num = float64(v)
		ok = true
	case int64:
		num = float64(v)
		ok = true
	default:
		ok = false
	}

	if !ok {
		return fmt.Errorf("expected number at %s, got %T", path, content)
	}

	// Check minimum
	if minimum, exists := schema["minimum"]; exists {
		var minVal float64
		switch v := minimum.(type) {
		case float64:
			minVal = v
		case float32:
			minVal = float64(v)
		case int:
			minVal = float64(v)
		case int64:
			minVal = float64(v)
		default:
			return fmt.Errorf("invalid minimum type at %s: expected number, got %T", path, minimum)
		}
		if num < minVal {
			return fmt.Errorf("number at %s is %f, minimum %f required", path, num, minVal)
		}
	}

	// Check maximum
	if maximum, exists := schema["maximum"]; exists {
		var maxVal float64
		switch v := maximum.(type) {
		case float64:
			maxVal = v
		case float32:
			maxVal = float64(v)
		case int:
			maxVal = float64(v)
		case int64:
			maxVal = float64(v)
		default:
			return fmt.Errorf("invalid maximum type at %s: expected number, got %T", path, maximum)
		}
		if num > maxVal {
			return fmt.Errorf("number at %s is %f, maximum %f allowed", path, num, maxVal)
		}
	}

	return nil
}

// validateInteger validates an integer against an integer schema
func validateSchemaInteger(content interface{}, schema map[string]interface{}, path string) error {
	var isInt bool

	switch content.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		isInt = true
	case float64:
		// Check if it's a whole number
		if f := content.(float64); f == float64(int64(f)) {
			isInt = true
		}
	}

	if !isInt {
		return fmt.Errorf("expected integer at %s, got %T", path, content)
	}

	// Delegate to number validation for range checks
	return validateSchemaNumber(content, schema, path)
}

// validateBoolean validates a boolean against a boolean schema
func validateSchemaBoolean(content interface{}, schema map[string]interface{}, path string) error {
	if _, ok := content.(bool); !ok {
		return fmt.Errorf("expected boolean at %s, got %T", path, content)
	}
	return nil
}

// validateNull validates a null value
func validateSchemaNull(content interface{}, path string) error {
	if content != nil {
		return fmt.Errorf("expected null at %s, got %T", path, content)
	}
	return nil
}

// StructuredToolWrapper wraps a StructuredToolFunc to work with regular ToolFunc interface
func StructuredToolWrapper(structuredFunc StructuredToolFunc, outputSchema *map[string]interface{}) types.ToolFunc {
	return func(ctx context.Context, arguments json.RawMessage) (*types.CallToolResult, error) {
		// Call the structured function
		result, err := structuredFunc(ctx, arguments)
		if err != nil {
			return StructuredToolError(map[string]interface{}{
				"error":   err.Error(),
				"type":    "execution_error",
				"details": nil,
			}), nil
		}

		// Validate against output schema if provided
		if outputSchema != nil {
			if validationErr := ValidateStructuredOutput(result, *outputSchema); validationErr != nil {
				return StructuredToolError(map[string]interface{}{
					"error":   "Output validation failed",
					"type":    "validation_error",
					"details": validationErr.Error(),
				}), nil
			}
		}

		return StructuredToolResult(result), nil
	}
}

// Common output schema helpers

// StringOutputSchema creates a simple string output schema
func StringOutputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "string",
	}
}

// ObjectOutputSchema creates an object output schema
func ObjectOutputSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// ArrayOutputSchema creates an array output schema
func ArrayOutputSchema(itemSchema map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":  "array",
		"items": itemSchema,
	}
}

// NumberOutputSchema creates a number output schema with optional constraints
func NumberOutputSchema(minimum, maximum *float64) map[string]interface{} {
	schema := map[string]interface{}{
		"type": "number",
	}
	if minimum != nil {
		schema["minimum"] = *minimum
	}
	if maximum != nil {
		schema["maximum"] = *maximum
	}
	return schema
}