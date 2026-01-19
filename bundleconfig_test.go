package rubygemsclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBundleConfigYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name: "basic config",
			input: `---
BUNDLE_RUBYGEMS__PKG__GITHUB__COM: "seuros:ghp_token123"
BUNDLE_PATH: "vendor/bundle"
`,
			expected: map[string]string{
				"BUNDLE_RUBYGEMS__PKG__GITHUB__COM": "seuros:ghp_token123",
				"BUNDLE_PATH":                       "vendor/bundle",
			},
		},
		{
			name: "single quotes",
			input: `---
BUNDLE_GEMS__CONTRIBSYS__COM: 'any:sidekiq_token'
`,
			expected: map[string]string{
				"BUNDLE_GEMS__CONTRIBSYS__COM": "any:sidekiq_token",
			},
		},
		{
			name: "no quotes",
			input: `---
BUNDLE_JOBS: 4
BUNDLE_RETRY: 3
`,
			expected: map[string]string{
				"BUNDLE_JOBS":  "4",
				"BUNDLE_RETRY": "3",
			},
		},
		{
			name: "with comments",
			input: `---
# This is a comment
BUNDLE_PATH: vendor/bundle
# Another comment
BUNDLE_JOBS: 4
`,
			expected: map[string]string{
				"BUNDLE_PATH": "vendor/bundle",
				"BUNDLE_JOBS": "4",
			},
		},
		{
			name: "empty config",
			input: `---
`,
			expected: map[string]string{},
		},
		{
			name: "non-bundle keys ignored",
			input: `---
SOME_OTHER_KEY: value
BUNDLE_PATH: vendor
`,
			expected: map[string]string{
				"BUNDLE_PATH": "vendor",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBundleConfigYAML([]byte(tt.input))

			if len(result) != len(tt.expected) {
				t.Errorf("got %d keys, want %d", len(result), len(tt.expected))
			}

			for k, v := range tt.expected {
				if got := result[k]; got != v {
					t.Errorf("key %q: got %q, want %q", k, got, v)
				}
			}
		})
	}
}

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"quoted"`, "quoted"},
		{`'single'`, "single"},
		{"noquotes", "noquotes"},
		{`""`, ""},
		{`''`, ""},
		{`"`, `"`},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := trimQuotes(tt.input); got != tt.expected {
				t.Errorf("trimQuotes(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBundleConfig_CredentialsForHost(t *testing.T) {
	config := &BundleConfig{
		credentials: map[string]*Credentials{
			"BUNDLE_RUBYGEMS__PKG__GITHUB__COM": {
				Username: "any",
				Password: "ghp_token123",
				Token:    "ghp_token123",
			},
			"BUNDLE_GEMS__CONTRIBSYS__COM": {
				Username: "user",
				Password: "pass",
			},
		},
	}

	t.Run("github packages", func(t *testing.T) {
		creds := config.CredentialsForHost("rubygems.pkg.github.com")
		if creds == nil {
			t.Fatal("expected credentials")
		}
		if creds.Token != "ghp_token123" {
			t.Errorf("got token %q, want %q", creds.Token, "ghp_token123")
		}
	})

	t.Run("sidekiq pro", func(t *testing.T) {
		creds := config.CredentialsForHost("gems.contribsys.com")
		if creds == nil {
			t.Fatal("expected credentials")
		}
		if creds.Username != "user" || creds.Password != "pass" {
			t.Errorf("got %q:%q, want user:pass", creds.Username, creds.Password)
		}
	})

	t.Run("unknown host", func(t *testing.T) {
		creds := config.CredentialsForHost("unknown.example.com")
		if creds != nil {
			t.Error("expected nil for unknown host")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		var nilConfig *BundleConfig
		if creds := nilConfig.CredentialsForHost("any.host"); creds != nil {
			t.Error("expected nil for nil config")
		}
	})
}

func TestLoadBundleConfig_LocalFile(t *testing.T) {
	// Create a temporary directory with .bundle/config
	tmpDir := t.TempDir()
	bundleDir := filepath.Join(tmpDir, ".bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `---
BUNDLE_RUBYGEMS__PKG__GITHUB__COM: "any:test_token"
`
	if err := os.WriteFile(filepath.Join(bundleDir, "config"), []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	config := LoadBundleConfig()
	if config == nil {
		t.Fatal("expected config to be loaded")
	}

	creds := config.CredentialsForHost("rubygems.pkg.github.com")
	if creds == nil {
		t.Fatal("expected credentials")
	}
	if creds.Token != "test_token" {
		t.Errorf("got token %q, want %q", creds.Token, "test_token")
	}
}
