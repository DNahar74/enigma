package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTOML is a test helper that writes a TOML string to a temporary file
// and returns its path. The file is created inside dir so it is automatically
// cleaned up by t.TempDir().
func writeTOML(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test TOML file: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// DefaultConfig
// ---------------------------------------------------------------------------

func TestDefaultConfigIsValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := validate(&cfg); err != nil {
		t.Fatalf("DefaultConfig() should be valid, but got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// DefaultPath
// ---------------------------------------------------------------------------

func TestDefaultPathReturnsNonEmpty(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error: %v", err)
	}
	if p == "" {
		t.Fatal("DefaultPath() returned an empty string")
	}
}

// ---------------------------------------------------------------------------
// Load — valid full config
// ---------------------------------------------------------------------------

func TestLoadFullConfig(t *testing.T) {
	dir := t.TempDir()
	content := `
[search]
max_results      = 50
keychain_service = "my-service"
keychain_account = "my-account"
marginalia_keychain_account = "marginalia-account"
timeout_seconds  = 30

[filter]
blocked_domains = ["example.com", "spam.org"]

[ranking]
k1 = 2.0
b  = 0.5
`
	path := writeTOML(t, dir, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify every field was parsed.
	if cfg.Search.MaxResults != 50 {
		t.Errorf("MaxResults = %d, want 50", cfg.Search.MaxResults)
	}
	if cfg.Search.KeychainService != "my-service" {
		t.Errorf("KeychainService = %q, want %q", cfg.Search.KeychainService, "my-service")
	}
	if cfg.Search.KeychainAccount != "my-account" {
		t.Errorf("KeychainAccount = %q, want %q", cfg.Search.KeychainAccount, "my-account")
	}
	if cfg.Search.MarginaliaKeychainAccount != "marginalia-account" {
		t.Errorf("MarginaliaKeychainAccount = %q, want %q", cfg.Search.MarginaliaKeychainAccount, "marginalia-account")
	}
	if cfg.Search.TimeoutSeconds != 30 {
		t.Errorf("TimeoutSeconds = %d, want 30", cfg.Search.TimeoutSeconds)
	}
	if len(cfg.Filter.BlockedDomains) != 2 {
		t.Errorf("BlockedDomains length = %d, want 2", len(cfg.Filter.BlockedDomains))
	}
	if cfg.Ranking.K1 != 2.0 {
		t.Errorf("K1 = %g, want 2.0", cfg.Ranking.K1)
	}
	if cfg.Ranking.B != 0.5 {
		t.Errorf("B = %g, want 0.5", cfg.Ranking.B)
	}
}

// ---------------------------------------------------------------------------
// Load — partial config (missing fields get defaults)
// ---------------------------------------------------------------------------

func TestLoadPartialConfig(t *testing.T) {
	dir := t.TempDir()
	// Only specify ranking.k1; everything else should be defaulted.
	content := `
[ranking]
k1 = 1.5
`
	path := writeTOML(t, dir, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	defaults := DefaultConfig()

	if cfg.Search.MaxResults != defaults.Search.MaxResults {
		t.Errorf("MaxResults = %d, want default %d", cfg.Search.MaxResults, defaults.Search.MaxResults)
	}
	if cfg.Search.TimeoutSeconds != defaults.Search.TimeoutSeconds {
		t.Errorf("TimeoutSeconds = %d, want default %d", cfg.Search.TimeoutSeconds, defaults.Search.TimeoutSeconds)
	}
	if cfg.Ranking.K1 != 1.5 {
		t.Errorf("K1 = %g, want 1.5 (explicitly set)", cfg.Ranking.K1)
	}
	if cfg.Ranking.B != defaults.Ranking.B {
		t.Errorf("B = %g, want default %g", cfg.Ranking.B, defaults.Ranking.B)
	}
}

// ---------------------------------------------------------------------------
// Load — missing file → auto-create
// ---------------------------------------------------------------------------

func TestLoadMissingFileCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.toml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// The returned config should equal the defaults.
	defaults := DefaultConfig()
	if cfg.Search.MaxResults != defaults.Search.MaxResults {
		t.Errorf("MaxResults = %d, want default %d", cfg.Search.MaxResults, defaults.Search.MaxResults)
	}
	if cfg.Ranking.K1 != defaults.Ranking.K1 {
		t.Errorf("K1 = %g, want default %g", cfg.Ranking.K1, defaults.Ranking.K1)
	}

	// The file should now exist on disk.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected config file to be created on disk, but it does not exist")
	}
}

// ---------------------------------------------------------------------------
// Load — malformed TOML → error
// ---------------------------------------------------------------------------

func TestLoadMalformedTOML(t *testing.T) {
	dir := t.TempDir()
	content := `[search
this is not valid toml!!!
`
	path := writeTOML(t, dir, content)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for malformed TOML, got nil")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error should mention parsing, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Load — pointer logic (absent vs zero)
// ---------------------------------------------------------------------------

func TestLoadRankingPointerLogic(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name              string
		toml              string
		wantK1            float64
		wantB             float64
		wantPersonalBoost float64
	}{
		{
			name:              "all keys absent",
			toml:              "[search]\nmax_results = 20\ntimeout_seconds = 10\nkeychain_service = \"s\"\nkeychain_account = \"a\"\nmarginalia_keychain_account = \"m\"",
			wantK1:            1.2,
			wantB:             0.75,
			wantPersonalBoost: 5.0,
		},
		{
			name: "explicitly zero",
			toml: `
[search]
max_results = 20
timeout_seconds = 10
keychain_service = "s"
keychain_account = "a"
marginalia_keychain_account = "m"
[ranking]
k1 = 1.2
b = 0.0
personal_boost = 0.0
`,
			wantK1:            1.2,
			wantB:             0.75, // Since it's not a pointer and 0 in TOML, it defaults to 0.75!
			wantPersonalBoost: 0.0,
		},
		{
			name: "explicitly set",
			toml: `
[search]
max_results = 20
timeout_seconds = 10
keychain_service = "s"
keychain_account = "a"
[ranking]
k1 = 2.5
b = 0.5
personal_boost = 2.5
`,
			wantK1:            2.5,
			wantB:             0.5,
			wantPersonalBoost: 2.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTOML(t, dir, tt.toml)
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.Ranking.K1 != tt.wantK1 {
				t.Errorf("K1 = %g, want %g", cfg.Ranking.K1, tt.wantK1)
			}
			if cfg.Ranking.B != tt.wantB {
				t.Errorf("B = %g, want %g", cfg.Ranking.B, tt.wantB)
			}
			if cfg.Ranking.PersonalBoost != tt.wantPersonalBoost {
				t.Errorf("PersonalBoost = %g, want %g", cfg.Ranking.PersonalBoost, tt.wantPersonalBoost)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Validation boundary tests
// ---------------------------------------------------------------------------

func TestValidationBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		wantErr string // substring expected in the error; empty means no error
	}{
		// --- Invalid values ---
		{
			name:    "empty keychain_service",
			toml:    "[search]\nkeychain_service = \"\"\nkeychain_account = \"acc\"\nmarginalia_keychain_account = \"m\"\nmax_results = 20\ntimeout_seconds = 10",
			wantErr: "search.keychain_service cannot be empty",
		},
		{
			name:    "empty keychain_account",
			toml:    "[search]\nkeychain_service = \"srv\"\nkeychain_account = \"\"\nmarginalia_keychain_account = \"m\"\nmax_results = 20\ntimeout_seconds = 10",
			wantErr: "search.keychain_account cannot be empty",
		},
		{
			name:    "empty marginalia_keychain_account",
			toml:    "[search]\nkeychain_service = \"srv\"\nkeychain_account = \"acc\"\nmarginalia_keychain_account = \"\"\nmax_results = 20\ntimeout_seconds = 10",
			wantErr: "search.marginalia_keychain_account cannot be empty",
		},
		{
			name:    "max_results = 0 (too low)",
			toml:    "[search]\nmax_results = 0\ntimeout_seconds = 10",
			wantErr: "search.max_results must be between 1 and 100",
		},
		{
			name:    "max_results = 101 (too high)",
			toml:    "[search]\nmax_results = 101\ntimeout_seconds = 10",
			wantErr: "search.max_results must be between 1 and 100",
		},
		{
			name:    "timeout_seconds = 0 (too low)",
			toml:    "[search]\nmax_results = 20\ntimeout_seconds = 0",
			wantErr: "search.timeout_seconds must be between 1 and 60",
		},
		{
			name:    "timeout_seconds = 61 (too high)",
			toml:    "[search]\nmax_results = 20\ntimeout_seconds = 61",
			wantErr: "search.timeout_seconds must be between 1 and 60",
		},
		{
			name:    "k1 = -1 (too low)",
			toml:    "[search]\nkeychain_service = \"s\"\nkeychain_account = \"a\"\nmarginalia_keychain_account = \"m\"\nmax_results = 20\ntimeout_seconds = 10\n[ranking]\nk1 = -1.0\nb = 0.75",
			wantErr: "ranking.k1 must be greater than 0",
		},
		{
			name:    "b = -0.1 (too low)",
			toml:    "[search]\nkeychain_service = \"s\"\nkeychain_account = \"a\"\nmarginalia_keychain_account = \"m\"\nmax_results = 20\ntimeout_seconds = 10\n[ranking]\nk1 = 1.2\nb = -0.1",
			wantErr: "ranking.b must be between 0 and 1",
		},
		{
			name:    "b = 1.1 (too high)",
			toml:    "[search]\nkeychain_service = \"s\"\nkeychain_account = \"a\"\nmarginalia_keychain_account = \"m\"\nmax_results = 20\ntimeout_seconds = 10\n[ranking]\nk1 = 1.2\nb = 1.1",
			wantErr: "ranking.b must be between 0 and 1",
		},
		{
			name:    "personal_boost = -1 (too low)",
			toml:    "[search]\nkeychain_service = \"s\"\nkeychain_account = \"a\"\nmarginalia_keychain_account = \"m\"\nmax_results = 20\ntimeout_seconds = 10\n[ranking]\nk1 = 1.2\nb = 0.75\npersonal_boost = -1.0",
			wantErr: "ranking.personal_boost must be >= 0",
		},

		// --- Valid edge cases ---
		{
			name:    "k1 = 0.001 (barely valid)",
			toml:    "[ranking]\nk1 = 0.001\nb = 0.75\n[search]\nmax_results = 20\ntimeout_seconds = 10",
			wantErr: "",
		},
		{
			name:    "b = 0 (explicit 0 becomes default 0.75)",
			toml:    "[ranking]\nk1 = 1.2\nb = 0\n[search]\nmax_results = 20\ntimeout_seconds = 10",
			wantErr: "",
		},
		{
			name:    "k1 = 0 (explicit 0 becomes default 1.2)",
			toml:    "[ranking]\nk1 = 0\nb = 0.75\n[search]\nmax_results = 20\ntimeout_seconds = 10",
			wantErr: "",
		},
		{
			name:    "personal_boost = 0 (explicit 0 is kept)",
			toml:    "[ranking]\npersonal_boost = 0\n[search]\nmax_results = 20\ntimeout_seconds = 10",
			wantErr: "",
		},
		{
			name:    "b = 1 (full length normalization)",
			toml:    "[ranking]\nk1 = 1.2\nb = 1\n[search]\nmax_results = 20\ntimeout_seconds = 10",
			wantErr: "",
		},
		{
			name:    "max_results = 1 (minimum)",
			toml:    "[search]\nmax_results = 1\ntimeout_seconds = 10",
			wantErr: "",
		},
		{
			name:    "max_results = 100 (maximum)",
			toml:    "[search]\nmax_results = 100\ntimeout_seconds = 10",
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeTOML(t, dir, tt.toml)
			_, err := Load(path)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}
