package query

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantRaw    string
		wantTokens []string
		wantErr    bool
	}{
		{
			name:    "empty string returns error",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace-only returns error",
			input:   "   \t\n  ",
			wantErr: true,
		},
		{
			name:       "single word is lowercased",
			input:      "Raft",
			wantRaw:    "Raft",
			wantTokens: []string{"raft"},
		},
		{
			name:       "multi-word produces multiple tokens",
			input:      "raft consensus algorithm",
			wantRaw:    "raft consensus algorithm",
			wantTokens: []string{"raft", "consensus", "algorithm"},
		},
		{
			name:       "duplicate tokens are deduplicated",
			input:      "raft raft consensus",
			wantRaw:    "raft raft consensus",
			wantTokens: []string{"raft", "consensus"},
		},
		{
			name:       "mixed case is lowercased",
			input:      "Raft CONSENSUS",
			wantRaw:    "Raft CONSENSUS",
			wantTokens: []string{"raft", "consensus"},
		},
		{
			name:       "extra whitespace is collapsed",
			input:      "  raft   consensus  ",
			wantRaw:    "raft   consensus",
			wantTokens: []string{"raft", "consensus"},
		},
		{
			name:       "unicode tokens are preserved without ASCII normalization",
			input:      "café résumé",
			wantRaw:    "café résumé",
			wantTokens: []string{"café", "résumé"},
		},
		{
			name:       "raw preserves trimmed original case",
			input:      "  Go Concurrency Patterns  ",
			wantRaw:    "Go Concurrency Patterns",
			wantTokens: []string{"go", "concurrency", "patterns"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}

			if got.Raw != tt.wantRaw {
				t.Errorf("Raw = %q, want %q", got.Raw, tt.wantRaw)
			}

			if len(got.Tokens) != len(tt.wantTokens) {
				t.Fatalf("Tokens = %v (len %d), want %v (len %d)",
					got.Tokens, len(got.Tokens), tt.wantTokens, len(tt.wantTokens))
			}

			for i, tok := range got.Tokens {
				if tok != tt.wantTokens[i] {
					t.Errorf("Tokens[%d] = %q, want %q", i, tok, tt.wantTokens[i])
				}
			}
		})
	}
}
