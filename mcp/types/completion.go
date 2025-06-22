package types

type ArgumentCompletionValue struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type CompleteRequest struct {
	Ref struct {
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"ref"`
	Argument string `json:"argument"`
}

type CompleteResult struct {
	Completion struct {
		Values   []ArgumentCompletionValue `json:"values"`
		Total    *int                      `json:"total,omitempty"`
		HasMore  bool                      `json:"hasMore,omitempty"`
	} `json:"completion"`
}