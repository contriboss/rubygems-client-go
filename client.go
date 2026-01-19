// Package rubygemsclient provides a Go client for the RubyGems.org API.
// Ruby equivalent: Gem::SpecFetcher
package rubygemsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Client provides access to RubyGems.org API.
// Ruby equivalent: Gem::RemoteFetcher
type Client struct {
	baseURL     string
	httpClient  *http.Client
	credentials *Credentials
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithCredentials sets credentials for authenticating with the gem server.
func WithCredentials(creds *Credentials) ClientOption {
	return func(c *Client) {
		c.credentials = creds
	}
}

// GemInfo represents gem metadata from RubyGems.org
type GemInfo struct {
	Name         string               `json:"name"`
	Version      string               `json:"version"`
	Dependencies DependencyCategories `json:"dependencies"`
}

// DependencyCategories represents the dependency structure from RubyGems API
type DependencyCategories struct {
	Development []Dependency `json:"development"`
	Runtime     []Dependency `json:"runtime"`
}

// Dependency represents a gem dependency
type Dependency struct {
	Name         string `json:"name"`
	Requirements string `json:"requirements"`
}

// NewClient creates a new RubyGems.org API client with connection pooling
func NewClient(opts ...ClientOption) *Client {
	return NewClientWithBaseURL("https://rubygems.org", opts...)
}

// NewClientWithBaseURL creates a client for a custom gem server
func NewClientWithBaseURL(baseURL string, opts ...ClientOption) *Client {
	// Ensure baseURL doesn't end with /
	if baseURL != "" && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}

	// Create HTTP transport with connection pooling
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxConnsPerHost:       20,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	c := &Client{
		baseURL: baseURL + "/api/v1",
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// applyAuth adds authentication headers to the request if credentials are set.
func (c *Client) applyAuth(req *http.Request) {
	if c.credentials == nil {
		return
	}

	if c.credentials.IsToken() {
		req.Header.Set("Authorization", "Bearer "+c.credentials.GetToken())
	} else if c.credentials.Username != "" {
		req.SetBasicAuth(c.credentials.Username, c.credentials.Password)
	}
}

// GetGemInfo fetches gem metadata (uses latest version's dependencies for simplicity)
func (c *Client) GetGemInfo(name, version string) (*GemInfo, error) {
	// For MVP: use latest version's dependencies for all versions
	// In production, we'd use the compact index or version-specific APIs
	url := fmt.Sprintf("%s/gems/%s.json", c.baseURL, name)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch gem info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RubyGems API returned status %d for %s", resp.StatusCode, name)
	}

	var info GemInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode gem info: %w", err)
	}

	// Override version to match what was requested
	info.Version = version
	info.Name = name

	return &info, nil
}

// VersionInfo represents version metadata from RubyGems.org
type VersionInfo struct {
	Number string `json:"number"`
}

// GetGemVersions fetches all versions for a gem
func (c *Client) GetGemVersions(name string) ([]string, error) {
	url := fmt.Sprintf("%s/versions/%s.json", c.baseURL, name)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch gem versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RubyGems API returned status %d for %s", resp.StatusCode, name)
	}

	var versions []VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("failed to decode gem versions: %w", err)
	}

	// Limit to most recent 20 versions to avoid overwhelming the resolver
	maxVersions := 20
	if len(versions) > maxVersions {
		versions = versions[:maxVersions]
	}

	versionStrings := make([]string, len(versions))
	for i, v := range versions {
		versionStrings[i] = v.Number
	}

	return versionStrings, nil
}

// GemInfoRequest represents a request for gem information
type GemInfoRequest struct {
	Name    string
	Version string
}

// GemInfoResult represents the result of a gem info request
type GemInfoResult struct {
	Request GemInfoRequest
	Info    *GemInfo
	Error   error
}

// GetMultipleGemInfo fetches gem metadata for multiple gems in parallel
func (c *Client) GetMultipleGemInfo(requests []GemInfoRequest) []GemInfoResult {
	results := make([]GemInfoResult, len(requests))
	var wg sync.WaitGroup

	// Use buffered channel to limit concurrent requests
	semaphore := make(chan struct{}, 10) // Max 10 concurrent requests

	for i, req := range requests {
		wg.Go(func() {
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			info, err := c.GetGemInfo(req.Name, req.Version)
			results[i] = GemInfoResult{
				Request: req,
				Info:    info,
				Error:   err,
			}
		})
	}

	wg.Wait()
	return results
}
