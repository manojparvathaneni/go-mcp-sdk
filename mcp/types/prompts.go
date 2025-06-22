package types

import (
	"encoding/json"
)

type PromptArgument struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
}

type Prompt struct {
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	Arguments   []PromptArgument       `json:"arguments,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type ListPromptsRequest struct {
	PaginationOptions
}

type ListPromptsResult struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor *string  `json:"nextCursor,omitempty"`
}

type GetPromptRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type GetPromptResult struct {
	Description *string       `json:"description,omitempty"`
	Messages    []interface{} `json:"messages"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type PromptMessage struct {
	Role    Role          `json:"role"`
	Content interface{}   `json:"content"`
}

type PromptListChangedNotification struct{}

func (g *GetPromptResult) UnmarshalJSON(data []byte) error {
	type Alias GetPromptResult
	aux := &struct {
		Messages []json.RawMessage `json:"messages"`
		*Alias
	}{
		Alias: (*Alias)(g),
	}
	
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	
	g.Messages = make([]interface{}, len(aux.Messages))
	for i, raw := range aux.Messages {
		var msg PromptMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return err
		}
		
		var content json.RawMessage
		if err := json.Unmarshal(raw, &struct {
			Content *json.RawMessage `json:"content"`
		}{Content: &content}); err != nil {
			return err
		}
		
		var contentType struct {
			Type string `json:"type"`
		}
		
		if err := json.Unmarshal(content, &contentType); err != nil {
			msg.Content = string(content)
			g.Messages[i] = msg
			continue
		}
		
		switch contentType.Type {
		case "text":
			var textContent TextContent
			if err := json.Unmarshal(content, &textContent); err != nil {
				return err
			}
			msg.Content = textContent
		case "image":
			var imageContent ImageContent
			if err := json.Unmarshal(content, &imageContent); err != nil {
				return err
			}
			msg.Content = imageContent
		case "resource":
			var resourceContent ResourceContents
			if err := json.Unmarshal(content, &resourceContent); err != nil {
				return err
			}
			msg.Content = resourceContent
		default:
			msg.Content = content
		}
		
		g.Messages[i] = msg
	}
	
	return nil
}