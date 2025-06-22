package types

import (
	"encoding/json"
)

type Resource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	MimeType    *string                `json:"mimeType,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type ResourceTemplate struct {
	URITemplate string                 `json:"uriTemplate"`
	Name        string                 `json:"name"`
	Description *string                `json:"description,omitempty"`
	MimeType    *string                `json:"mimeType,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type ListResourcesRequest struct {
	PaginationOptions
}

type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
	NextCursor *string   `json:"nextCursor,omitempty"`
}

type ListResourceTemplatesRequest struct {
	PaginationOptions
}

type ListResourceTemplatesResult struct {
	ResourceTemplates []ResourceTemplate `json:"resourceTemplates"`
	NextCursor       *string            `json:"nextCursor,omitempty"`
}

type ReadResourceRequest struct {
	URI string `json:"uri"`
}

type ResourceContentsText struct {
	URI      string  `json:"uri"`
	MimeType *string `json:"mimeType,omitempty"`
	Text     string  `json:"text"`
}

// isResourceContents implements the ResourceContents interface
func (r ResourceContentsText) isResourceContents() {}

type ResourceContentsBlob struct {
	URI      string  `json:"uri"`
	MimeType *string `json:"mimeType,omitempty"`
	Blob     string  `json:"blob"`
}

// isResourceContents implements the ResourceContents interface
func (r ResourceContentsBlob) isResourceContents() {}

type ReadResourceResult struct {
	Contents []interface{} `json:"contents"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type SubscribeRequest struct {
	URI string `json:"uri"`
}

type UnsubscribeRequest struct {
	URI string `json:"uri"`
}

type ResourceUpdatedNotification struct {
	URI string `json:"uri"`
}

type ResourceListChangedNotification struct{}

func (r *ReadResourceResult) UnmarshalJSON(data []byte) error {
	type Alias ReadResourceResult
	aux := &struct {
		Contents []json.RawMessage `json:"contents"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	
	r.Contents = make([]interface{}, len(aux.Contents))
	for i, raw := range aux.Contents {
		var contentType struct {
			URI      string  `json:"uri"`
			MimeType *string `json:"mimeType,omitempty"`
			Text     *string `json:"text,omitempty"`
			Blob     *string `json:"blob,omitempty"`
		}
		
		if err := json.Unmarshal(raw, &contentType); err != nil {
			return err
		}
		
		if contentType.Text != nil {
			var textContent ResourceContentsText
			if err := json.Unmarshal(raw, &textContent); err != nil {
				return err
			}
			r.Contents[i] = textContent
		} else if contentType.Blob != nil {
			var blobContent ResourceContentsBlob
			if err := json.Unmarshal(raw, &blobContent); err != nil {
				return err
			}
			r.Contents[i] = blobContent
		}
	}
	
	return nil
}