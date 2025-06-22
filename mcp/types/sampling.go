package types

import (
	"encoding/json"
)

// Sampling and message creation types

type CreateMessageRequest struct {
	Messages         []SamplingMessage  `json:"messages"`
	ModelPreferences *ModelPreferences  `json:"modelPreferences,omitempty"`
	SystemPrompt     *string           `json:"systemPrompt,omitempty"`
	IncludeContext   *string           `json:"includeContext,omitempty"`
	Temperature      *float64          `json:"temperature,omitempty"`
	MaxTokens        int               `json:"maxTokens"`
	StopSequences    []string          `json:"stopSequences,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type CreateMessageResult struct {
	Content    interface{} `json:"content"`
	Model      string      `json:"model"`
	Role       Role        `json:"role"`
	StopReason *string     `json:"stopReason,omitempty"`
}

type SamplingMessage struct {
	Role    Role        `json:"role"`
	Content interface{} `json:"content"`
}

type ModelPreferences struct {
	Hints                []ModelHint `json:"hints,omitempty"`
	CostPriority         *float64    `json:"costPriority,omitempty"`
	SpeedPriority        *float64    `json:"speedPriority,omitempty"`
	IntelligencePriority *float64    `json:"intelligencePriority,omitempty"`
}

type ModelHint struct {
	Name string `json:"name,omitempty"`
}

// Progress notification types
type ProgressToken struct {
	Token string `json:"token"`
}

type ProgressNotification struct {
	ProgressToken ProgressToken `json:"progressToken"`
	Progress      float64       `json:"progress"`
	Total         *float64      `json:"total,omitempty"`
}

// Cancellation types
type CancelledNotification struct {
	RequestId json.RawMessage `json:"requestId"`
	Reason    *string         `json:"reason,omitempty"`
}

// Ping types
type PingRequest struct{}

type EmptyResult struct{}

// Roots types (for filesystem access)
type Root struct {
	URI  string  `json:"uri"`
	Name *string `json:"name,omitempty"`
}

type ListRootsRequest struct {
	PaginationOptions
}

type ListRootsResult struct {
	Roots      []Root  `json:"roots"`
	NextCursor *string `json:"nextCursor,omitempty"`
}

// Resource template matching
type ResourceTemplateMatch struct {
	URI       string                 `json:"uri"`
	Variables map[string]string      `json:"variables"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

func (c *CreateMessageResult) UnmarshalJSON(data []byte) error {
	type Alias CreateMessageResult
	aux := &struct {
		Content json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	
	// Try to unmarshal content as different types
	var contentType struct {
		Type string `json:"type,omitempty"`
	}
	
	if err := json.Unmarshal(aux.Content, &contentType); err != nil {
		// If it doesn't have a type field, treat as raw content
		c.Content = string(aux.Content)
		return nil
	}
	
	switch contentType.Type {
	case "text":
		var textContent TextContent
		if err := json.Unmarshal(aux.Content, &textContent); err != nil {
			return err
		}
		c.Content = textContent
	case "image":
		var imageContent ImageContent
		if err := json.Unmarshal(aux.Content, &imageContent); err != nil {
			return err
		}
		c.Content = imageContent
	case "audio":
		var audioContent AudioContent
		if err := json.Unmarshal(aux.Content, &audioContent); err != nil {
			return err
		}
		c.Content = audioContent
	case "resource":
		var resourceContent ResourceContents
		if err := json.Unmarshal(aux.Content, &resourceContent); err != nil {
			return err
		}
		c.Content = resourceContent
	default:
		c.Content = aux.Content
	}
	
	return nil
}