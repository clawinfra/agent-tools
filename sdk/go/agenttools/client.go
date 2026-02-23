// Package agenttools provides the Go SDK for agent-tools consumers and providers.
package agenttools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client is an agent-tools registry client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithAuthToken sets the DID auth token.
func WithAuthToken(token string) ClientOption {
	return func(c *Client) { c.authToken = token }
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// NewClient creates a new agent-tools client.
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Tool represents a registered tool.
type Tool struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Version     string     `json:"version"`
	Description string     `json:"description"`
	Pricing     *Pricing   `json:"pricing"`
	ProviderID  string     `json:"provider_id"`
	Endpoint    string     `json:"endpoint"`
	TimeoutMS   int64      `json:"timeout_ms"`
	Tags        []string   `json:"tags"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Pricing describes invocation cost.
type Pricing struct {
	Model      string `json:"model"`
	AmountCLAW string `json:"amount_claw,omitempty"`
}

// String returns a human-readable pricing description.
func (p *Pricing) String() string {
	if p == nil || p.Model == "free" {
		return "free"
	}
	return fmt.Sprintf("%s CLAW/%s", p.AmountCLAW, p.Model)
}

// RegisterToolRequest is input for tool registration.
type RegisterToolRequest struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
	Pricing     *Pricing       `json:"pricing,omitempty"`
	Endpoint    string         `json:"endpoint"`
	TimeoutMS   int64          `json:"timeout_ms,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
}

// ListToolsRequest is input for listing tools.
type ListToolsRequest struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}

// ToolList is a paginated list of tools.
type ToolList struct {
	Tools []*Tool `json:"tools"`
	Total int     `json:"total"`
	Page  int     `json:"page"`
	Limit int     `json:"limit"`
}

// SearchOption configures a tool search.
type SearchOption func(*searchOptions)

type searchOptions struct {
	maxPrice float64
	tag      string
	limit    int
}

// WithMaxPrice filters tools by maximum price in CLAW.
func WithMaxPrice(maxPriceCLAW float64) SearchOption {
	return func(o *searchOptions) { o.maxPrice = maxPriceCLAW }
}

// WithTag filters tools by tag.
func WithTag(tag string) SearchOption {
	return func(o *searchOptions) { o.tag = tag }
}

// WithLimit sets the maximum number of results.
func WithLimit(limit int) SearchOption {
	return func(o *searchOptions) { o.limit = limit }
}

// SearchResult is the response from a tool search.
type SearchResult struct {
	Tools []*Tool `json:"tools"`
	Total int     `json:"total"`
	Query string  `json:"query,omitempty"`
}

// RegisterTool registers a new tool in the registry.
func (c *Client) RegisterTool(ctx context.Context, req *RegisterToolRequest) (*Tool, error) {
	var tool Tool
	if err := c.post(ctx, "/v1/tools", req, &tool); err != nil {
		return nil, err
	}
	return &tool, nil
}

// GetTool retrieves a tool by ID.
func (c *Client) GetTool(ctx context.Context, id string) (*Tool, error) {
	var tool Tool
	if err := c.get(ctx, "/v1/tools/"+url.PathEscape(id), &tool); err != nil {
		return nil, err
	}
	return &tool, nil
}

// ListTools returns paginated tools.
func (c *Client) ListTools(ctx context.Context, req *ListToolsRequest) (*ToolList, error) {
	path := "/v1/tools"
	if req != nil {
		path += fmt.Sprintf("?page=%d&limit=%d", req.Page, req.Limit)
	}
	var list ToolList
	if err := c.get(ctx, path, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// SearchTools searches for tools by capability.
func (c *Client) SearchTools(ctx context.Context, query string, opts ...SearchOption) (*SearchResult, error) {
	o := &searchOptions{limit: 20}
	for _, opt := range opts {
		opt(o)
	}

	path := fmt.Sprintf("/v1/tools/search?q=%s&limit=%d", url.QueryEscape(query), o.limit)
	if o.maxPrice > 0 {
		path += fmt.Sprintf("&max_price_claw=%.2f", o.maxPrice)
	}
	if o.tag != "" {
		path += "&tag=" + url.QueryEscape(o.tag)
	}

	var result SearchResult
	if err := c.get(ctx, path, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Healthz checks the registry health.
func (c *Client) Healthz(ctx context.Context) error {
	return c.get(ctx, "/healthz", nil)
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	c.setAuth(req)
	return c.do(req, out)
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)
	return c.do(req, out)
}

func (c *Client) setAuth(req *http.Request) {
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
}

type apiErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var e apiErrorResponse
		if decErr := json.NewDecoder(resp.Body).Decode(&e); decErr == nil && e.Error.Code != "" {
			return fmt.Errorf("api error %s: %s", e.Error.Code, e.Error.Message)
		}
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// CLAWAmount returns a string representation of a CLAW amount.
func CLAWAmount(amount float64) string {
	return fmt.Sprintf("%.1f", amount)
}
