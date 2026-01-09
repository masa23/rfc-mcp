package search

// SearchInput represents the input for the rfc_search tool
type SearchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// RFCItem represents an RFC entry in the search results
type RFCItem struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Status string `json:"status,omitempty"`
	Date   string `json:"date,omitempty"`
	URL    string `json:"url"`
}

// FetchInput represents the input for the rfc_fetch tool
type FetchInput struct {
	Number   int `json:"number"`
	MaxBytes int `json:"maxBytes,omitempty"`
}

// ExtractInput represents the input for the rfc_extract tool
type ExtractInput struct {
	Number  int    `json:"number"`
	Section string `json:"section"`
}

// SearchResults represents the output of the rfc_search tool
type SearchResults struct {
	Items []RFCItem `json:"items"`
}

// FetchResult represents the output of the rfc_fetch tool
type FetchResult struct {
	Content string `json:"content"`
}

// ExtractResult represents the output of the rfc_extract tool
type ExtractResult struct {
	Content string `json:"content"`
}
