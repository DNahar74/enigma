// Package config handles loading, validating, and bootstrapping Enigma's
// TOML configuration file.
//
// # Bootstrapping philosophy
//
// The config file stores user preferences (result count, BM25 tuning, blocked
// domains). If no file exists on first run, we auto-create one with sensible
// sensible defaults so the user has a documented template to edit.
// This separation means the config file can be committed to a dotfiles repo
// without leaking any sensitive data.
//
// # Defaults vs. validation
//
// A missing field gets a default value and a log line so the user knows what
// happened. An explicitly-set field that falls outside valid bounds returns a
// hard error rather than silently clamping — the user made an intentional
// choice, and silently changing it would be surprising.
package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

// ---------------------------------------------------------------------------
// Configuration types
// ---------------------------------------------------------------------------

// Config is the top-level Enigma configuration. Each section maps to a TOML
// table in the config file (e.g. [search], [filter], [ranking]).
type Config struct {
	Search  SearchConfig  `toml:"search"`
	Local   LocalConfig   `toml:"local"`
	Filter  FilterConfig  `toml:"filter"`
	Ranking RankingConfig `toml:"ranking"`
	Trust   TrustConfig   `toml:"trust"`
}

type LocalConfig struct {
	NotesPath string `toml:"notes_path"`
}

// SearchConfig controls how Enigma queries the search provider (Tavily).
type SearchConfig struct {
	// MaxResults is the number of results to request from Tavily.
	// Default: 20. Tavily's API allows up to configured max results.
	MaxResults int `toml:"max_results"`
	// KeychainService is the OS keychain service name used to store the API key.
	// Default: "enigma".
	KeychainService string `toml:"keychain_service"`

	// KeychainAccount is the OS keychain account name used to store the API key.
	// Default: "tavily-api-key".
	KeychainAccount string `toml:"keychain_account"`

	// TimeoutSeconds is the HTTP request timeout when calling Tavily.
	// Default: 10. Valid range: 1-60.
	TimeoutSeconds int `toml:"timeout_seconds"`

	// MarginaliaKeychainAccount is the OS keychain account name used to store the Marginalia API key.
	// Default: "marginalia-api-key". If not present in the keychain, the public tier is used.
	MarginaliaKeychainAccount string `toml:"marginalia_keychain_account"`
}

// FilterConfig controls post-search filtering of results.
type FilterConfig struct {
	// BlockedDomains is a list of domains whose results are silently dropped.
	// Example: ["pinterest.com", "quora.com"]. Default: empty.
	BlockedDomains []string `toml:"blocked_domains"`
}

// RankingConfig holds BM25 tuning parameters for local re-ranking.
type RankingConfig struct {
	// K1 controls term-frequency saturation in BM25. Higher values give more
	// weight to repeated terms. The standard default from Robertson et al.
	// (1994) is 1.2 — a good general-purpose value. Must be > 0.
	K1 float64 `toml:"k1"`

	// B controls document-length normalization in BM25. 0 means no length
	// normalization; 1 means full normalization. The standard default is 0.75.
	// Must be in [0, 1].
	B float64 `toml:"b"`

	// PersonalBoost controls how much vocabulary overlap with local notes boosts a web result.
	PersonalBoost float64 `toml:"personal_boost"`
}

// TrustConfig controls domain-level trust scores.
type TrustConfig struct {
	BoostedDomains   []string `toml:"boosted_domains"`
	PenalizedDomains []string `toml:"penalized_domains"`
}

// rawConfig mirrors Config but uses pointer types for ranking fields
// where zero is a valid, distinct-from-default value. TOML decodes into
// this type so nil means "key absent" while *0.0 means "explicitly zero."
// Only used during loading — the rest of the app sees the resolved Config.
type rawConfig struct {
	Search  SearchConfig     `toml:"search"`
	Local   LocalConfig      `toml:"local"`
	Filter  FilterConfig     `toml:"filter"`
	Ranking rawRankingConfig `toml:"ranking"`
	Trust   TrustConfig      `toml:"trust"`
}

// rawRankingConfig uses a pointer type for PersonalBoost because zero is a valid,
// distinct-from-default value (0 means "disable personal ranking", default is 5.0).
// K1 and B do not use pointers because they have no legitimate zero value;
// an absent or zero value will just be replaced by the default.
type rawRankingConfig struct {
	K1            float64  `toml:"k1"`
	B             float64  `toml:"b"`
	PersonalBoost *float64 `toml:"personal_boost"`
}

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

// DefaultConfig returns a Config with all fields set to their default values.
// These defaults are designed to work out of the box without any user
// configuration.
func DefaultConfig() Config {
	return Config{
		Search: SearchConfig{
			MaxResults:                20,               // 20 is a good balance of speed and coverage.
			KeychainService:           "enigma",         // OS keychain service name
			KeychainAccount:           "tavily-api-key", // OS keychain account name
			TimeoutSeconds:            10,               // Long enough for most networks, short enough to fail fast.
			MarginaliaKeychainAccount: "marginalia-api-key",
		},
		Local: LocalConfig{
			NotesPath: "notes",
		},
		Filter: FilterConfig{
			BlockedDomains: []string{}, // No domains blocked by default — user adds their own.
		},
		Ranking: RankingConfig{
			K1:            1.2,  // Standard BM25 default from the original 1994 paper.
			B:             0.75, // Standard BM25 default; balances length normalization.
			PersonalBoost: 5.0,
		},
		Trust: TrustConfig{
			BoostedDomains:   []string{"github.com", "stackoverflow.com", "wikipedia.org"},
			PenalizedDomains: []string{"quora.com", "pinterest.com"},
		},
	}
}

// ---------------------------------------------------------------------------
// Default path
// ---------------------------------------------------------------------------

// DefaultPath returns the platform-specific path where the config file lives.
//
// On Linux/macOS it respects XDG_CONFIG_HOME (falling back to ~/.config).
// On Windows it uses %APPDATA%.
//
// Returns an error only if the user's home/config directory cannot be
// determined — a rare situation that usually means the environment is broken.
func DefaultPath() (string, error) {
	// On non-Windows platforms, prefer the XDG_CONFIG_HOME environment
	// variable so we follow the XDG Base Directory Specification. If unset,
	// os.UserConfigDir() returns ~/.config on Linux and ~/Library/Application
	// Support on macOS — we override macOS to also use ~/.config for
	// consistency and simplicity.
	if runtime.GOOS != "windows" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "enigma", "config.toml"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining home directory: %w", err)
		}
		return filepath.Join(home, ".config", "enigma", "config.toml"), nil
	}

	// On Windows, os.UserConfigDir() returns %APPDATA% which is the standard
	// location for application configuration.
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determining config directory: %w", err)
	}
	return filepath.Join(configDir, "enigma", "config.toml"), nil
}

// ---------------------------------------------------------------------------
// Loading and validation
// ---------------------------------------------------------------------------

// Load reads the TOML config file at path, applies defaults for any missing
// fields, and validates every field.
//
// If the file does not exist, Load creates it (including parent directories)
// with the default config so the user has a documented starting point. If the
// file exists but contains malformed TOML, Load returns an error immediately.
func Load(path string) (*Config, error) {
	// Check if the file exists. We distinguish "not found" from other I/O
	// errors (e.g. permission denied) so we only auto-create when appropriate.
	_, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("checking config file %s: %w", path, err)
		}
		return createDefault(path)
	}

	return loadExisting(path)
}

// createDefault writes the default config to path and returns it. This is
// called on first run so the user gets a working config with inline comments
// explaining each option.
func createDefault(path string) (*Config, error) {
	// MkdirAll is safe to call even if the directory already exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating config directory %s: %w", dir, err)
	}

	cfg := DefaultConfig()

	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating config file %s: %w", path, err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	if err := enc.Encode(cfg); err != nil {
		return nil, fmt.Errorf("writing default config to %s: %w", path, err)
	}

	log.Printf("No config found — created default at %s", path)
	return &cfg, nil
}

// loadExisting parses an existing TOML config file into a rawConfig (which
// uses pointer types for ranking fields to distinguish absent-vs-zero),
// resolves defaults, validates, and returns the fully-populated Config.
func loadExisting(path string) (*Config, error) {
	var raw rawConfig
	meta, err := toml.DecodeFile(path, &raw)
	if err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	cfg := resolveConfig(&raw, meta)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config in %s: %w", path, err)
	}

	return cfg, nil
}

// resolveConfig builds a fully-populated Config from the raw TOML decode
// output, applying defaults for any missing fields.
//
// Non-ranking fields use meta.IsDefined() to detect absent keys (their
// Go zero values — 0, "" — are always invalid, so there's no ambiguity).
// Ranking fields use pointer-nil checks: a nil pointer means the key was
// absent from the file; a non-nil pointer (even to 0.0) means the user
// explicitly set it. This is the only way to distinguish "user wrote
// personal_boost = 0" from "user didn't mention personal_boost at all."
func resolveConfig(raw *rawConfig, meta toml.MetaData) *Config {
	defaults := DefaultConfig()

	cfg := &Config{
		Search: raw.Search,
		Local:  raw.Local,
		Filter: raw.Filter,
		Trust:  raw.Trust,
	}

	// Helper for non-pointer fields: apply the default when the TOML key
	// was absent. These fields don't have the nil-vs-zero problem because
	// their zero values are always invalid (0 MaxResults, empty service name).
	set := func(key string, target any, defaultVal any) {
		if meta.IsDefined(splitKey(key)...) {
			return
		}
		switch t := target.(type) {
		case *int:
			*t = defaultVal.(int)
		case *string:
			*t = defaultVal.(string)
		case *[]string:
			*t = defaultVal.([]string)
		}
		log.Printf("Config: %s not set, using default %v", key, defaultVal)
	}

	set("search.max_results", &cfg.Search.MaxResults, defaults.Search.MaxResults)
	set("search.keychain_service", &cfg.Search.KeychainService, defaults.Search.KeychainService)
	set("search.keychain_account", &cfg.Search.KeychainAccount, defaults.Search.KeychainAccount)
	set("search.timeout_seconds", &cfg.Search.TimeoutSeconds, defaults.Search.TimeoutSeconds)
	set("search.marginalia_keychain_account", &cfg.Search.MarginaliaKeychainAccount, defaults.Search.MarginaliaKeychainAccount)
	set("filter.blocked_domains", &cfg.Filter.BlockedDomains, defaults.Filter.BlockedDomains)
	set("local.notes_path", &cfg.Local.NotesPath, defaults.Local.NotesPath)
	set("trust.boosted_domains", &cfg.Trust.BoostedDomains, defaults.Trust.BoostedDomains)
	set("trust.penalized_domains", &cfg.Trust.PenalizedDomains, defaults.Trust.PenalizedDomains)

	if raw.Ranking.K1 == 0 {
		cfg.Ranking.K1 = defaults.Ranking.K1
		log.Printf("Config: ranking.k1 not set (or zero), using default %v", defaults.Ranking.K1)
	} else {
		cfg.Ranking.K1 = raw.Ranking.K1
	}

	if raw.Ranking.B == 0 {
		cfg.Ranking.B = defaults.Ranking.B
		log.Printf("Config: ranking.b not set (or zero), using default %v", defaults.Ranking.B)
	} else {
		cfg.Ranking.B = raw.Ranking.B
	}

	if raw.Ranking.PersonalBoost != nil {
		cfg.Ranking.PersonalBoost = *raw.Ranking.PersonalBoost
	} else {
		log.Printf("Config: ranking.personal_boost not set, using default %v", defaults.Ranking.PersonalBoost)
		cfg.Ranking.PersonalBoost = defaults.Ranking.PersonalBoost
	}

	return cfg
}

// splitKey turns a dotted key like "search.max_results" into a slice
// ["search", "max_results"] for use with toml.MetaData.IsDefined.
func splitKey(key string) []string {
	parts := []string{}
	current := ""
	for _, c := range key {
		if c == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// validate checks that every field in the resolved Config falls within
// its valid range. Returns a hard error for any invalid value — no
// silent defaults. This is consistent across all fields per §2.3:
// "there is no per-field exception."
func validate(c *Config) error {
	if c.Search.MaxResults < 1 || c.Search.MaxResults > 100 {
		return fmt.Errorf(
			"search.max_results must be between 1 and 100, got %d",
			c.Search.MaxResults,
		)
	}

	if c.Search.TimeoutSeconds < 1 || c.Search.TimeoutSeconds > 60 {
		return fmt.Errorf(
			"search.timeout_seconds must be between 1 and 60, got %d",
			c.Search.TimeoutSeconds,
		)
	}

	if c.Search.KeychainService == "" {
		return fmt.Errorf("search.keychain_service cannot be empty")
	}

	if c.Search.KeychainAccount == "" {
		return fmt.Errorf("search.keychain_account cannot be empty")
	}

	if c.Search.MarginaliaKeychainAccount == "" {
		return fmt.Errorf("search.marginalia_keychain_account cannot be empty")
	}

	if c.Ranking.K1 <= 0 {
		return fmt.Errorf("ranking.k1 must be greater than 0, got %g", c.Ranking.K1)
	}

	if c.Ranking.B < 0 || c.Ranking.B > 1 {
		return fmt.Errorf("ranking.b must be between 0 and 1, got %g", c.Ranking.B)
	}

	if c.Ranking.PersonalBoost < 0 {
		return fmt.Errorf("ranking.personal_boost must be >= 0, got %g", c.Ranking.PersonalBoost)
	}

	return nil
}
