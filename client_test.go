package rubygemsclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient()

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.baseURL != "https://rubygems.org/api/v1" {
		t.Errorf("Expected baseURL to be 'https://rubygems.org/api/v1', got %s", client.baseURL)
	}

	if client.httpClient == nil {
		t.Error("Expected non-nil HTTP client")
	}

	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout to be 30s, got %v", client.httpClient.Timeout)
	}
}

func TestGetGemInfo_Success(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gems/test-gem.json" {
			t.Errorf("Expected path '/gems/test-gem.json', got %s", r.URL.Path)
		}

		response := GemInfo{
			Name:    "test-gem",
			Version: "1.0.0",
			Dependencies: DependencyCategories{
				Runtime: []Dependency{
					{Name: "json", Requirements: ">= 1.0"},
				},
				Development: []Dependency{
					{Name: "rspec", Requirements: "~> 3.0"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	info, err := client.GetGemInfo("test-gem", "1.2.0")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Name != "test-gem" {
		t.Errorf("Expected name 'test-gem', got %s", info.Name)
	}

	// Should override version to requested version
	if info.Version != "1.2.0" {
		t.Errorf("Expected version '1.2.0', got %s", info.Version)
	}

	if len(info.Dependencies.Runtime) != 1 {
		t.Errorf("Expected 1 runtime dependency, got %d", len(info.Dependencies.Runtime))
	}

	if info.Dependencies.Runtime[0].Name != "json" {
		t.Errorf("Expected runtime dependency 'json', got %s", info.Dependencies.Runtime[0].Name)
	}
}

func TestGetGemInfo_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := client.GetGemInfo("nonexistent-gem", "1.0.0")
	if err == nil {
		t.Fatal("Expected error for 404 response")
	}

	expectedErr := "RubyGems API returned status 404"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("Expected error to contain '%s', got %s", expectedErr, err.Error())
	}
}

func TestGetGemVersions_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/versions/test-gem.json" {
			t.Errorf("Expected path '/versions/test-gem.json', got %s", r.URL.Path)
		}

		versions := []VersionInfo{
			{Number: "1.2.0"},
			{Number: "1.1.0"},
			{Number: "1.0.0"},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(versions)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	versions, err := client.GetGemVersions("test-gem")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedVersions := []string{"1.2.0", "1.1.0", "1.0.0"}
	if len(versions) != len(expectedVersions) {
		t.Errorf("Expected %d versions, got %d", len(expectedVersions), len(versions))
	}

	for i, expected := range expectedVersions {
		if versions[i] != expected {
			t.Errorf("Expected version %s at index %d, got %s", expected, i, versions[i])
		}
	}
}

func TestGetGemVersions_TooManyVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 25 versions (more than the 20 limit)
		versions := make([]VersionInfo, 25)
		for i := 0; i < 25; i++ {
			versions[i] = VersionInfo{Number: "1.0." + string(rune('0'+i))}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(versions)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	versions, err := client.GetGemVersions("test-gem")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be limited to 20 versions
	if len(versions) != 20 {
		t.Errorf("Expected 20 versions (limited), got %d", len(versions))
	}
}

func TestClientWithCredentials_Token(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Authorization header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test_token_123" {
			t.Errorf("Expected 'Bearer test_token_123', got %q", auth)
		}

		response := GemInfo{Name: "test-gem", Version: "1.0.0"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	creds := &Credentials{Token: "test_token_123"}
	client := NewClientWithBaseURL(server.URL, WithCredentials(creds))

	_, err := client.GetGemInfo("test-gem", "1.0.0")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestClientWithCredentials_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check Basic Auth
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("Expected Basic Auth to be set")
		}
		if user != "myuser" || pass != "mypassword" {
			t.Errorf("Expected 'myuser:mypassword', got %q:%q", user, pass)
		}

		response := GemInfo{Name: "test-gem", Version: "1.0.0"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	creds := &Credentials{Username: "myuser", Password: "mypassword"}
	client := NewClientWithBaseURL(server.URL, WithCredentials(creds))

	_, err := client.GetGemInfo("test-gem", "1.0.0")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestGetMultipleGemInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple mock that returns different responses based on gem name
		var response GemInfo
		if strings.Contains(r.URL.Path, "gem1") {
			response = GemInfo{Name: "gem1", Version: "1.0.0"}
		} else if strings.Contains(r.URL.Path, "gem2") {
			response = GemInfo{Name: "gem2", Version: "2.0.0"}
		} else {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	requests := []GemInfoRequest{
		{Name: "gem1", Version: "1.0.0"},
		{Name: "gem2", Version: "2.0.0"},
		{Name: "nonexistent", Version: "1.0.0"},
	}

	results := client.GetMultipleGemInfo(requests)

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Check successful results
	if results[0].Error != nil {
		t.Errorf("Expected gem1 to succeed, got error: %v", results[0].Error)
	}
	if results[0].Info.Name != "gem1" {
		t.Errorf("Expected gem1 name, got %s", results[0].Info.Name)
	}

	if results[1].Error != nil {
		t.Errorf("Expected gem2 to succeed, got error: %v", results[1].Error)
	}
	if results[1].Info.Name != "gem2" {
		t.Errorf("Expected gem2 name, got %s", results[1].Info.Name)
	}

	// Check failed result
	if results[2].Error == nil {
		t.Error("Expected nonexistent gem to fail")
	}
}
