package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestStructuredToolResult(t *testing.T) {
	data := map[string]interface{}{
		"result": "success",
		"value":  42,
	}
	
	result := StructuredToolResult(data)
	
	if result.IsError {
		t.Error("Expected IsError to be false")
	}
	
	if result.StructuredContent == nil {
		t.Error("Expected StructuredContent to be set")
	}
	
	if content, ok := (*result.StructuredContent).(map[string]interface{}); !ok {
		t.Error("Expected StructuredContent to be a map")
	} else if content["result"] != "success" {
		t.Error("Expected StructuredContent to contain correct data")
	}
}

func TestStructuredToolError(t *testing.T) {
	errorData := map[string]interface{}{
		"error": "Something went wrong",
		"code":  500,
	}
	
	result := StructuredToolError(errorData)
	
	if !result.IsError {
		t.Error("Expected IsError to be true")
	}
	
	if result.StructuredContent == nil {
		t.Error("Expected StructuredContent to be set")
	}
	
	if content, ok := (*result.StructuredContent).(map[string]interface{}); !ok {
		t.Error("Expected StructuredContent to be a map")
	} else if content["error"] != "Something went wrong" {
		t.Error("Expected StructuredContent to contain error data")
	}
}

func TestValidateStructuredOutput_Object(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"age": map[string]interface{}{
				"type": "number",
			},
		},
		"required": []interface{}{"name"},
	}
	
	// Valid object
	validData := map[string]interface{}{
		"name": "John",
		"age":  30,
	}
	
	err := ValidateStructuredOutput(validData, schema)
	if err != nil {
		t.Errorf("Expected validation to pass for valid object: %v", err)
	}
	
	// Missing required field
	invalidData := map[string]interface{}{
		"age": 30,
	}
	
	err = ValidateStructuredOutput(invalidData, schema)
	if err == nil {
		t.Error("Expected validation to fail for missing required field")
	}
}

func TestValidateStructuredOutput_Array(t *testing.T) {
	schema := map[string]interface{}{
		"type": "array",
		"items": map[string]interface{}{
			"type": "string",
		},
		"minItems": 1,
		"maxItems": 3,
	}
	
	// Valid array
	validData := []interface{}{"item1", "item2"}
	
	err := ValidateStructuredOutput(validData, schema)
	if err != nil {
		t.Errorf("Expected validation to pass for valid array: %v", err)
	}
	
	// Empty array (violates minItems)
	invalidData := []interface{}{}
	
	err = ValidateStructuredOutput(invalidData, schema)
	if err == nil {
		t.Error("Expected validation to fail for empty array")
	}
	
	// Too many items
	tooManyItems := []interface{}{"item1", "item2", "item3", "item4"}
	
	err = ValidateStructuredOutput(tooManyItems, schema)
	if err == nil {
		t.Error("Expected validation to fail for too many items")
	}
}

func TestValidateStructuredOutput_String(t *testing.T) {
	schema := map[string]interface{}{
		"type":      "string",
		"minLength": 3,
		"maxLength": 10,
	}
	
	// Valid string
	err := ValidateStructuredOutput("hello", schema)
	if err != nil {
		t.Errorf("Expected validation to pass for valid string: %v", err)
	}
	
	// Too short
	err = ValidateStructuredOutput("hi", schema)
	if err == nil {
		t.Error("Expected validation to fail for string too short")
	}
	
	// Too long
	err = ValidateStructuredOutput("this is way too long", schema)
	if err == nil {
		t.Error("Expected validation to fail for string too long")
	}
}

func TestValidateStructuredOutput_Number(t *testing.T) {
	schema := map[string]interface{}{
		"type":    "number",
		"minimum": 0,
		"maximum": 100,
	}
	
	// Valid number
	err := ValidateStructuredOutput(50.5, schema)
	if err != nil {
		t.Errorf("Expected validation to pass for valid number: %v", err)
	}
	
	// Below minimum
	err = ValidateStructuredOutput(-1, schema)
	if err == nil {
		t.Error("Expected validation to fail for number below minimum")
	}
	
	// Above maximum
	err = ValidateStructuredOutput(101, schema)
	if err == nil {
		t.Error("Expected validation to fail for number above maximum")
	}
}

func TestStructuredToolWrapper(t *testing.T) {
	outputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"result": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []interface{}{"result"},
	}
	
	// Create a structured tool function
	structuredFunc := func(ctx context.Context, arguments json.RawMessage) (interface{}, error) {
		return map[string]interface{}{
			"result": "success",
		}, nil
	}
	
	// Wrap it
	wrappedFunc := StructuredToolWrapper(structuredFunc, &outputSchema)
	
	// Test the wrapped function
	result, err := wrappedFunc(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Expected wrapped function to succeed: %v", err)
	}
	
	if result.IsError {
		t.Error("Expected result to not be an error")
	}
	
	if result.StructuredContent == nil {
		t.Error("Expected StructuredContent to be set")
	}
}

func TestStructuredToolWrapper_ValidationError(t *testing.T) {
	outputSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"result": map[string]interface{}{
				"type": "string",
			},
		},
		"required": []interface{}{"result"},
	}
	
	// Create a structured tool function that returns invalid data
	structuredFunc := func(ctx context.Context, arguments json.RawMessage) (interface{}, error) {
		return map[string]interface{}{
			"invalid": "data",
		}, nil
	}
	
	// Wrap it
	wrappedFunc := StructuredToolWrapper(structuredFunc, &outputSchema)
	
	// Test the wrapped function
	result, err := wrappedFunc(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Expected wrapped function to not return error: %v", err)
	}
	
	if !result.IsError {
		t.Error("Expected result to be an error due to validation failure")
	}
}

func TestOutputSchemaHelpers(t *testing.T) {
	// Test StringOutputSchema
	stringSchema := StringOutputSchema()
	if stringSchema["type"] != "string" {
		t.Error("Expected StringOutputSchema to have type 'string'")
	}
	
	// Test ObjectOutputSchema
	properties := map[string]interface{}{
		"name": map[string]interface{}{"type": "string"},
	}
	required := []string{"name"}
	objectSchema := ObjectOutputSchema(properties, required)
	if objectSchema["type"] != "object" {
		t.Error("Expected ObjectOutputSchema to have type 'object'")
	}
	
	// Test ArrayOutputSchema
	itemSchema := map[string]interface{}{"type": "string"}
	arraySchema := ArrayOutputSchema(itemSchema)
	if arraySchema["type"] != "array" {
		t.Error("Expected ArrayOutputSchema to have type 'array'")
	}
	
	// Test NumberOutputSchema
	min := 0.0
	max := 100.0
	numberSchema := NumberOutputSchema(&min, &max)
	if numberSchema["type"] != "number" {
		t.Error("Expected NumberOutputSchema to have type 'number'")
	}
	if numberSchema["minimum"] != min {
		t.Error("Expected NumberOutputSchema to have correct minimum")
	}
	if numberSchema["maximum"] != max {
		t.Error("Expected NumberOutputSchema to have correct maximum")
	}
}