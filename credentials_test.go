package rubygemsclient

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	testUser     = "myuser"
	testPassword = "mypassword"
)

func TestHostToEnvKey(t *testing.T) {
	tests := []struct {
		host     string
		expected string
	}{
		{"rubygems.org", "BUNDLE_RUBYGEMS__ORG"},
		{"rubygems.pkg.github.com", "BUNDLE_RUBYGEMS__PKG__GITHUB__COM"},
		{"gems.contribsys.com", "BUNDLE_GEMS__CONTRIBSYS__COM"},
		{"my-gems.example.com", "BUNDLE_MY___GEMS__EXAMPLE__COM"},
		{"localhost:8080", "BUNDLE_LOCALHOST"},
		{"gems.example.com:443", "BUNDLE_GEMS__EXAMPLE__COM"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			result := hostToEnvKey(tt.host)
			if result != tt.expected {
				t.Errorf("hostToEnvKey(%q) = %q, want %q", tt.host, result, tt.expected)
			}
		})
	}
}

func TestParseCredentialValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		wantNil  bool
		username string
		password string
		token    string
		isToken  bool
	}{
		{
			name:    "empty string",
			value:   "",
			wantNil: true,
		},
		{
			name:     "token format (any:token)",
			value:    "any:ghp_1234567890",
			username: "any",
			password: "ghp_1234567890",
			token:    "ghp_1234567890",
			isToken:  true,
		},
		{
			name:     "basic auth format",
			value:    "user:password123",
			username: "user",
			password: "password123",
			token:    "",
			isToken:  false,
		},
		{
			name:     "bare token (no colon)",
			value:    "ghp_1234567890",
			username: "",
			password: "",
			token:    "ghp_1234567890",
			isToken:  true,
		},
		{
			name:     "password with colons",
			value:    "user:pass:word:with:colons",
			username: "user",
			password: "pass:word:with:colons",
			token:    "",
			isToken:  false,
		},
		{
			name:     "any with empty password",
			value:    "any:",
			username: "any",
			password: "",
			token:    "",
			isToken:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := parseCredentialValue(tt.value)

			if tt.wantNil {
				if creds != nil {
					t.Errorf("parseCredentialValue(%q) = %+v, want nil", tt.value, creds)
				}
				return
			}

			if creds == nil {
				t.Fatalf("parseCredentialValue(%q) = nil, want non-nil", tt.value)
			}

			if creds.Username != tt.username {
				t.Errorf("Username = %q, want %q", creds.Username, tt.username)
			}
			if creds.Password != tt.password {
				t.Errorf("Password = %q, want %q", creds.Password, tt.password)
			}
			if creds.Token != tt.token {
				t.Errorf("Token = %q, want %q", creds.Token, tt.token)
			}
			if creds.IsToken() != tt.isToken {
				t.Errorf("IsToken() = %v, want %v", creds.IsToken(), tt.isToken)
			}
		})
	}
}

func TestCredentialsFromEnv(t *testing.T) {
	// Test with BUNDLE_RUBYGEMS__PKG__GITHUB__COM
	t.Run("github packages token", func(t *testing.T) {
		key := "BUNDLE_RUBYGEMS__PKG__GITHUB__COM"
		os.Setenv(key, "any:ghp_testtoken123")
		defer os.Unsetenv(key)

		creds := CredentialsFromEnv("rubygems.pkg.github.com")
		if creds == nil {
			t.Fatal("Expected non-nil credentials")
		}

		if !creds.IsToken() {
			t.Error("Expected token-based credentials")
		}

		if creds.GetToken() != "ghp_testtoken123" {
			t.Errorf("GetToken() = %q, want %q", creds.GetToken(), "ghp_testtoken123")
		}
	})

	// Test with BUNDLE_GEMS__CONTRIBSYS__COM (Sidekiq Pro)
	t.Run("sidekiq pro token", func(t *testing.T) {
		key := "BUNDLE_GEMS__CONTRIBSYS__COM"
		os.Setenv(key, "any:sidekiq_pro_token")
		defer os.Unsetenv(key)

		creds := CredentialsFromEnv("gems.contribsys.com")
		if creds == nil {
			t.Fatal("Expected non-nil credentials")
		}

		if creds.GetToken() != "sidekiq_pro_token" {
			t.Errorf("GetToken() = %q, want %q", creds.GetToken(), "sidekiq_pro_token")
		}
	})

	// Test missing env var
	t.Run("missing env var", func(t *testing.T) {
		// Make sure the env var is not set
		os.Unsetenv("BUNDLE_EXAMPLE__COM")

		creds := CredentialsFromEnv("example.com")
		if creds != nil {
			t.Errorf("Expected nil credentials for missing env var, got %+v", creds)
		}
	})

	// Test basic auth
	t.Run("basic auth", func(t *testing.T) {
		key := "BUNDLE_PRIVATE__GEMS__COM"
		os.Setenv(key, testUser+":"+testPassword)
		defer os.Unsetenv(key)

		creds := CredentialsFromEnv("private.gems.com")
		if creds == nil {
			t.Fatal("Expected non-nil credentials")
		}

		if creds.IsToken() {
			t.Error("Expected basic auth, not token")
		}

		if creds.Username != testUser {
			t.Errorf("Username = %q, want %q", creds.Username, testUser)
		}
		if creds.Password != testPassword {
			t.Errorf("Password = %q, want %q", creds.Password, testPassword)
		}
	})
}

func TestCredentials_GetToken(t *testing.T) {
	tests := []struct {
		name  string
		creds *Credentials
		want  string
	}{
		{
			name:  "nil credentials",
			creds: nil,
			want:  "",
		},
		{
			name:  "explicit token",
			creds: &Credentials{Token: "my_token"},
			want:  "my_token",
		},
		{
			name:  "any username with password",
			creds: &Credentials{Username: "any", Password: "the_token"},
			want:  "the_token",
		},
		{
			name:  "basic auth (not token)",
			creds: &Credentials{Username: "user", Password: "pass"},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.creds.GetToken()
			if got != tt.want {
				t.Errorf("GetToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCredentials_IsToken(t *testing.T) {
	tests := []struct {
		name  string
		creds *Credentials
		want  bool
	}{
		{
			name:  "nil credentials",
			creds: nil,
			want:  false,
		},
		{
			name:  "explicit token",
			creds: &Credentials{Token: "tok"},
			want:  true,
		},
		{
			name:  "any username",
			creds: &Credentials{Username: "any", Password: "tok"},
			want:  true,
		},
		{
			name:  "basic auth",
			creds: &Credentials{Username: "user", Password: "pass"},
			want:  false,
		},
		{
			name:  "empty credentials",
			creds: &Credentials{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.creds.IsToken()
			if got != tt.want {
				t.Errorf("IsToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCredentialsFor_PriorityOrder(t *testing.T) {
	// Reset cache before test
	ResetConfigCache()
	defer ResetConfigCache()

	// Create temp directory with local .bundle/config
	tmpDir := t.TempDir()
	bundleDir := filepath.Join(tmpDir, ".bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Local config has "local_token"
	localConfig := `---
BUNDLE_EXAMPLE__COM: "any:local_token"
`
	if err := os.WriteFile(filepath.Join(bundleDir, "config"), []byte(localConfig), 0600); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Also set env var with "env_token"
	t.Setenv("BUNDLE_EXAMPLE__COM", "any:env_token")

	// Local should win over env
	creds := CredentialsFor("example.com")
	if creds == nil {
		t.Fatal("expected credentials")
	}
	if creds.Token != "local_token" {
		t.Errorf("expected local_token (local > env), got %q", creds.Token)
	}
}

func TestCredentialsFor_EnvFallback(t *testing.T) {
	// Reset cache before test
	ResetConfigCache()
	defer ResetConfigCache()

	// No config files, just env var
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	t.Setenv("BUNDLE_NOCONFIG__COM", "any:env_only_token")

	creds := CredentialsFor("noconfig.com")
	if creds == nil {
		t.Fatal("expected credentials from env")
	}
	if creds.Token != "env_only_token" {
		t.Errorf("expected env_only_token, got %q", creds.Token)
	}
}
