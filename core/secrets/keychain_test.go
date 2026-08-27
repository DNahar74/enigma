package secrets

import (
	"testing"

	"github.com/zalando/go-keyring"
)

func TestSetAndGetAPIKey(t *testing.T) {
	keyring.MockInit()

	tests := []struct {
		name    string
		service string
		account string
		key     string
		wantErr bool
	}{
		{
			name:    "valid key",
			service: "enigma",
			account: "tavily-api-key",
			key:     "tvly-secret-123",
			wantErr: false,
		},
		{
			name:    "empty service",
			service: "",
			account: "tavily-api-key",
			key:     "tvly-secret-123",
			wantErr: true,
		},
		{
			name:    "empty account",
			service: "enigma",
			account: "",
			key:     "tvly-secret-123",
			wantErr: true,
		},
		{
			name:    "empty key",
			service: "enigma",
			account: "tavily-api-key",
			key:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetAPIKey(tt.service, tt.account, tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetAPIKey() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				got, err := GetAPIKey(tt.service, tt.account)
				if err != nil {
					t.Errorf("GetAPIKey() error = %v, want no error", err)
				}
				if got != tt.key {
					t.Errorf("GetAPIKey() got = %v, want %v", got, tt.key)
				}
			}
		})
	}
}

func TestGetAPIKey_NotFound(t *testing.T) {
	keyring.MockInit()

	_, err := GetAPIKey("enigma", "non-existent")
	if err == nil {
		t.Errorf("GetAPIKey() expected error for non-existent key, got nil")
	}
}

func TestGetAPIKey_EmptyInputs(t *testing.T) {
	keyring.MockInit()

	_, err := GetAPIKey("", "account")
	if err == nil {
		t.Errorf("GetAPIKey() expected error for empty service")
	}

	_, err = GetAPIKey("service", "")
	if err == nil {
		t.Errorf("GetAPIKey() expected error for empty account")
	}
}
