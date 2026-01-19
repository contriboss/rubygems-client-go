package rubygemsclient

import (
	"os"
	"strings"
)

const tokenUsername = "any"

// Credentials holds authentication info for gem sources.
// Supports both token-based auth (Bearer) and basic auth (username:password).
type Credentials struct {
	Username string
	Password string
	Token    string
}

// IsToken returns true if this is a token-based credential.
// Token auth uses "any" as username or has an explicit token field.
func (c *Credentials) IsToken() bool {
	if c == nil {
		return false
	}
	return c.Token != "" || c.Username == tokenUsername
}

// GetToken returns the token value for Bearer auth.
func (c *Credentials) GetToken() string {
	if c == nil {
		return ""
	}
	if c.Token != "" {
		return c.Token
	}
	if c.Username == tokenUsername {
		return c.Password
	}
	return ""
}

// CredentialsFor resolves credentials for a host using Bundler's full resolution order:
//  1. Local .bundle/config (project directory)
//  2. BUNDLE_<HOST> environment variable
//  3. Global ~/.bundle/config (user home)
//
// Returns nil if no credentials are found.
func CredentialsFor(host string) *Credentials {
	// 1. Check local .bundle/config first (highest priority)
	if localConfig := GetLocalBundleConfig(); localConfig != nil {
		if creds := localConfig.CredentialsForHost(host); creds != nil {
			return creds
		}
	}

	// 2. Check environment variable
	if creds := CredentialsFromEnv(host); creds != nil {
		return creds
	}

	// 3. Check global ~/.bundle/config (lowest priority)
	if globalConfig := GetGlobalBundleConfig(); globalConfig != nil {
		if creds := globalConfig.CredentialsForHost(host); creds != nil {
			return creds
		}
	}

	return nil
}

// CredentialsFromEnv resolves credentials from Bundler's BUNDLE_<HOST> env vars.
// Converts host "rubygems.pkg.github.com" → "BUNDLE_RUBYGEMS__PKG__GITHUB__COM"
// Returns nil if no credentials are found.
//
// Note: Prefer using CredentialsFor() which includes config file lookup.
func CredentialsFromEnv(host string) *Credentials {
	envKey := hostToEnvKey(host)
	value := os.Getenv(envKey)
	if value == "" {
		return nil
	}
	return parseCredentialValue(value)
}

// hostToEnvKey converts a hostname to Bundler's env var format.
// Example: "rubygems.pkg.github.com" → "BUNDLE_RUBYGEMS__PKG__GITHUB__COM"
func hostToEnvKey(host string) string {
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Check if it's actually a port (not part of IPv6)
		if !strings.Contains(host[idx:], "]") {
			host = host[:idx]
		}
	}

	// Replace dots with double underscores and convert to uppercase
	key := strings.ReplaceAll(host, ".", "__")
	key = strings.ReplaceAll(key, "-", "___")
	return "BUNDLE_" + strings.ToUpper(key)
}

// parseCredentialValue parses Bundler's credential format.
// Formats:
//   - "any:token" → token-based auth (Bearer)
//   - "username:password" → basic auth
//   - "token" → token-based auth (no colon, treat as token)
func parseCredentialValue(value string) *Credentials {
	if value == "" {
		return nil
	}

	// Check for username:password format
	if idx := strings.Index(value, ":"); idx != -1 {
		username := value[:idx]
		password := value[idx+1:]

		// "any" username means token auth
		if username == tokenUsername {
			return &Credentials{
				Username: tokenUsername,
				Password: password,
				Token:    password,
			}
		}

		return &Credentials{
			Username: username,
			Password: password,
		}
	}

	// No colon - treat as bare token
	return &Credentials{
		Token: value,
	}
}
