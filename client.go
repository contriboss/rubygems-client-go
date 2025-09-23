// Package rubygemsclient provides a Go client for the RubyGems.org API.
// Ruby equivalent: Gem::SpecFetcher
package rubygemsclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Client provides access to RubyGems.org API.
// Ruby equivalent: Gem::RemoteFetcher
type Client struct {
	baseURL    string
	httpClient *http.Client
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
func NewClient() *Client {
	// Create HTTP transport with connection pooling
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxConnsPerHost:       20,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	return &Client{
		baseURL: "https://rubygems.org/api/v1",
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
	}
}

// GetGemInfo fetches gem metadata (uses latest version's dependencies for simplicity)
func (c *Client) GetGemInfo(name, version string) (*GemInfo, error) {
	// For MVP: use latest version's dependencies for all versions
	// In production, we'd use the compact index or version-specific APIs
	url := fmt.Sprintf("%s/gems/%s.json", c.baseURL, name)

	resp, err := c.httpClient.Get(url)
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

	resp, err := c.httpClient.Get(url)
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
		wg.Add(1)
		go func(index int, request GemInfoRequest) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			info, err := c.GetGemInfo(request.Name, request.Version)
			results[index] = GemInfoResult{
				Request: request,
				Info:    info,
				Error:   err,
			}
		}(i, req)
	}

	wg.Wait()
	return results
}

