package secrets

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// SetAPIKey saves the given key in the OS keychain under the specified service and account.
func SetAPIKey(service, account, key string) error {
	if service == "" {
		return errors.New("service cannot be empty")
	}
	if account == "" {
		return errors.New("account cannot be empty")
	}
	if key == "" {
		return errors.New("key cannot be empty")
	}

	if err := keyring.Set(service, account, key); err != nil {
		return fmt.Errorf("failed to save API key to keychain: %w", err)
	}

	return nil
}

// GetAPIKey retrieves the API key from the OS keychain for the specified service and account.
func GetAPIKey(service, account string) (string, error) {
	if service == "" {
		return "", errors.New("service cannot be empty")
	}
	if account == "" {
		return "", errors.New("account cannot be empty")
	}

	key, err := keyring.Get(service, account)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve API key from keychain: %w", err)
	}

	return key, nil
}
