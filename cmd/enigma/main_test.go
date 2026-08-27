package main

import (
	"testing"
)

// TestBuildRootCmd verifies that the root command and its subcommands
// are constructed correctly without panicking.
func TestBuildRootCmd(t *testing.T) {
	cmd := buildRootCmd()
	if cmd == nil {
		t.Fatal("buildRootCmd() returned nil")
	}

	if cmd.Use != "enigma" {
		t.Errorf("expected root command use to be 'enigma', got %q", cmd.Use)
	}

	var hasSearch, hasAuth bool
	for _, sub := range cmd.Commands() {
		if sub.Use == "search <query>" {
			hasSearch = true
		}
		if sub.Use == "auth" {
			hasAuth = true
			var hasSetKey bool
			for _, authSub := range sub.Commands() {
				if authSub.Use == "set-key" {
					hasSetKey = true
				}
			}
			if !hasSetKey {
				t.Error("expected auth command to have 'set-key' subcommand")
			}
		}
	}

	if !hasSearch {
		t.Error("expected root command to have 'search' subcommand")
	}
	if !hasAuth {
		t.Error("expected root command to have 'auth' subcommand")
	}
}
