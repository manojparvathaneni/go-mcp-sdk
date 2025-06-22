package types

import (
	"encoding/json"
)

type Tool struct {
	Name         string                 `json:"name"`
	Description  *string                `json:"description,omitempty"`
	InputSchema  map[string]interface{} `json:"inputSchema"`
	OutputSchema *map[string]interface{} `json:"outputSchema,omitempty"`
}

type ListToolsRequest struct {
	PaginationOptions
}

type ListToolsResult struct {
	Tools      []Tool  `json:"tools"`
	NextCursor *string `json:"nextCursor,omitempty"`
}

type CallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content           []interface{}          `json:"content,omitempty"`
	StructuredContent *interface{}           `json:"structuredContent,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	IsError           bool                   `json:"isError,omitempty"`
}

type ToolListChangedNotification struct{}

func (c *CallToolResult) UnmarshalJSON(data []byte) error {
	type Alias CallToolResult
	aux := &struct {
		Content []json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	
	c.Content = make([]interface{}, len(aux.Content))
	for i, raw := range aux.Content {
		var contentType struct {
			Type string `json:"type"`
		}
		
		if err := json.Unmarshal(raw, &contentType); err != nil {
			return err
		}
		
		switch contentType.Type {
		case "text":
			var textContent TextContent
			if err := json.Unmarshal(raw, &textContent); err != nil {
				return err
			}
			c.Content[i] = textContent
		case "image":
			var imageContent ImageContent
			if err := json.Unmarshal(raw, &imageContent); err != nil {
				return err
			}
			c.Content[i] = imageContent
		case "resource":
			var resourceContent ResourceContents
			if err := json.Unmarshal(raw, &resourceContent); err != nil {
				return err
			}
			c.Content[i] = resourceContent
		default:
			c.Content[i] = raw
		}
	}
	
	return nil
}