package rubygemsclient

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// BundleConfig holds parsed credentials from a single .bundle/config file.
// It caches credentials keyed by BUNDLE_<HOST> format.
type BundleConfig struct {
	credentials map[string]*Credentials
}

var (
	localConfig      *BundleConfig
	globalConfig     *BundleConfig
	configLoadedOnce sync.Once
)

// ResetConfigCache clears the cached config for testing purposes.
// This should only be used in tests.
func ResetConfigCache() {
	localConfig = nil
	globalConfig = nil
	configLoadedOnce = sync.Once{}
}

// loadConfigs loads both local and global configs separately.
func loadConfigs() {
	// Load local config (.bundle/config)
	localPath := ".bundle/config"
	if data, err := os.ReadFile(localPath); err == nil {
		localConfig = parseConfigFile(data)
	}

	// Load global config (~/.bundle/config)
	if globalPath := globalBundleConfigPath(); globalPath != "" {
		if data, err := os.ReadFile(globalPath); err == nil {
			globalConfig = parseConfigFile(data)
		}
	}
}

// parseConfigFile parses a single config file into a BundleConfig.
func parseConfigFile(data []byte) *BundleConfig {
	config := &BundleConfig{
		credentials: make(map[string]*Credentials),
	}
	for k, v := range parseBundleConfigYAML(data) {
		if creds := parseCredentialValue(v); creds != nil {
			config.credentials[k] = creds
		}
	}
	if len(config.credentials) == 0 {
		return nil
	}
	return config
}

// GetLocalBundleConfig returns credentials from .bundle/config (project-local).
func GetLocalBundleConfig() *BundleConfig {
	configLoadedOnce.Do(loadConfigs)
	return localConfig
}

// GetGlobalBundleConfig returns credentials from ~/.bundle/config (user global).
func GetGlobalBundleConfig() *BundleConfig {
	configLoadedOnce.Do(loadConfigs)
	return globalConfig
}

// LoadBundleConfig loads and merges both config files for backwards compatibility.
// Priority: local (.bundle/config) > global (~/.bundle/config)
// Note: Prefer using CredentialsFor() which has the correct Bundler priority order.
func LoadBundleConfig() *BundleConfig {
	configLoadedOnce.Do(loadConfigs)

	if localConfig == nil && globalConfig == nil {
		return nil
	}

	merged := &BundleConfig{
		credentials: make(map[string]*Credentials),
	}

	// Global first (lower priority)
	if globalConfig != nil {
		for k, v := range globalConfig.credentials {
			merged.credentials[k] = v
		}
	}

	// Local second (overwrites global)
	if localConfig != nil {
		for k, v := range localConfig.credentials {
			merged.credentials[k] = v
		}
	}

	return merged
}

// CredentialsForHost returns credentials for the given host from config files.
func (c *BundleConfig) CredentialsForHost(host string) *Credentials {
	if c == nil {
		return nil
	}
	envKey := hostToEnvKey(host)
	return c.credentials[envKey]
}

// globalBundleConfigPath returns the path to the global .bundle/config.
// Checks: $BUNDLE_USER_HOME/.bundle/config, $HOME/.bundle/config
func globalBundleConfigPath() string {
	// Check BUNDLE_USER_HOME first
	if bundleHome := os.Getenv("BUNDLE_USER_HOME"); bundleHome != "" {
		return filepath.Join(bundleHome, ".bundle", "config")
	}

	// Fall back to ~/.bundle/config
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".bundle", "config")
	}

	return ""
}

// parseBundleConfigYAML parses Bundler's simple YAML config format.
// The format is:
//
//	---
//	BUNDLE_KEY: "value"
//	BUNDLE_OTHER_KEY: "other_value"
//
// Returns a map of key -> value (both strings).
func parseBundleConfigYAML(data []byte) map[string]string {
	result := make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines, comments, and YAML document markers
		if line == "" || strings.HasPrefix(line, "#") || line == "---" {
			continue
		}

		// Parse "KEY: value" or "KEY: 'value'" or 'KEY: "value"'
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		// Remove surrounding quotes if present
		value = trimQuotes(value)

		// Only store BUNDLE_ prefixed keys (potential credentials)
		if strings.HasPrefix(key, "BUNDLE_") {
			result[key] = value
		}
	}

	// Return empty map on scan error to avoid partial results
	if err := scanner.Err(); err != nil {
		return map[string]string{}
	}

	return result
}

// trimQuotes removes surrounding single or double quotes from a string.
func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
