package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/DNahar74/enigma/core/config"
	"github.com/DNahar74/enigma/core/pipeline"
	"github.com/DNahar74/enigma/core/plugin"
	"github.com/DNahar74/enigma/core/query"
	"github.com/DNahar74/enigma/core/render"
	"github.com/DNahar74/enigma/core/secrets"
	"github.com/DNahar74/enigma/plugins/filter_antislop"
	"github.com/DNahar74/enigma/plugins/filter_blocklist"
	"github.com/DNahar74/enigma/plugins/filter_dedup"
	"github.com/DNahar74/enigma/plugins/rank_bm25"
	"github.com/DNahar74/enigma/plugins/rank_personal"
	"github.com/DNahar74/enigma/plugins/rank_trust"
	"github.com/DNahar74/enigma/plugins/search_local"
	"github.com/DNahar74/enigma/plugins/search_marginalia"
	"github.com/DNahar74/enigma/plugins/search_tavily"
	"github.com/DNahar74/enigma/ui/tui"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// main is the entrypoint for the Enigma CLI.
// It sets up the Cobra command structure and wires together the core dependencies.
func buildRootCmd() *cobra.Command {
	var explain bool

	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use:   "enigma",
		Short: "Enigma local-first CLI search tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.DefaultPath()
			if err != nil {
				return fmt.Errorf("failed to get default config path: %w", err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			apiKey, _ := secrets.GetAPIKey(cfg.Search.KeychainService, cfg.Search.KeychainAccount)
			marginaliaKey, _ := secrets.GetAPIKey(cfg.Search.KeychainService, cfg.Search.MarginaliaKeychainAccount)

			tavilyPlugin := search_tavily.New(apiKey, cfg.Search.MaxResults, cfg.Search.TimeoutSeconds)
			marginaliaPlugin := search_marginalia.New(marginaliaKey, cfg.Search.MaxResults, cfg.Search.TimeoutSeconds)
			localPlugin := search_local.New(cfg.Local.NotesPath)

			filterPlugin := plugin.NewCompositeFilter(
				filter_blocklist.New(cfg.Filter.BlockedDomains),
				filter_dedup.New(),
				filter_antislop.New(),
			)

			rankPlugin := plugin.NewCompositeRanker(
				rank_bm25.New(cfg.Ranking.K1, cfg.Ranking.B),
				rank_personal.New(cfg.Ranking.PersonalBoost),
				rank_trust.New(cfg.Trust.BoostedDomains, cfg.Trust.PenalizedDomains),
			)

			reg, err := plugin.NewRegistry(
				[]plugin.SearchPlugin{tavilyPlugin, marginaliaPlugin, localPlugin},
				filterPlugin,
				rankPlugin,
			)
			if err != nil {
				return fmt.Errorf("failed to create plugin registry: %w", err)
			}

			p := pipeline.New(reg, cfg)

			tuiModel := tui.New(p)
			prog := tea.NewProgram(tuiModel, tea.WithAltScreen())
			if _, err := prog.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	// searchCmd represents the main functionality of the CLI.
	// It parses the query, loads the config, initializes the plugin registry,
	// and executes the search pipeline.
	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the web",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q, err := query.Parse(args[0])
			if err != nil {
				return fmt.Errorf("failed to parse query: %w", err)
			}

			path, err := config.DefaultPath()
			if err != nil {
				return fmt.Errorf("failed to get default config path: %w", err)
			}

			cfg, err := config.Load(path)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			apiKey, err := secrets.GetAPIKey(cfg.Search.KeychainService, cfg.Search.KeychainAccount)
			if err != nil {
				return fmt.Errorf("API key not found in keychain. Please run `enigma auth set-key` to configure it: %w", err)
			}

			// 3. Initialize Plugins
			// We inject dependencies (config values) into each plugin.
			// This follows our rule: no global state, everything passed down.
			tavilyPlugin := search_tavily.New(apiKey, cfg.Search.MaxResults, cfg.Search.TimeoutSeconds)

			marginaliaKey := ""
			if key, err := secrets.GetAPIKey(cfg.Search.KeychainService, cfg.Search.MarginaliaKeychainAccount); err == nil {
				marginaliaKey = key
			}
			marginaliaPlugin := search_marginalia.New(marginaliaKey, cfg.Search.MaxResults, cfg.Search.TimeoutSeconds)

			localPlugin := search_local.New(cfg.Local.NotesPath)

			blocklistPlugin := filter_blocklist.New(cfg.Filter.BlockedDomains)
			dedupPlugin := filter_dedup.New()
			antislopPlugin := filter_antislop.New()
			filterPlugin := plugin.NewCompositeFilter(blocklistPlugin, dedupPlugin, antislopPlugin)

			bm25Plugin := rank_bm25.New(cfg.Ranking.K1, cfg.Ranking.B)
			personalPlugin := rank_personal.New(cfg.Ranking.PersonalBoost)
			trustPlugin := rank_trust.New(cfg.Trust.BoostedDomains, cfg.Trust.PenalizedDomains)

			rankPlugin := plugin.NewCompositeRanker(bm25Plugin, personalPlugin, trustPlugin)

			// 4. Build Plugin Registry
			// The registry ensures exactly one of each plugin type is wired in.
			reg, err := plugin.NewRegistry(
				[]plugin.SearchPlugin{tavilyPlugin, marginaliaPlugin, localPlugin},
				filterPlugin,
				rankPlugin,
			)
			if err != nil {
				return fmt.Errorf("failed to create plugin registry: %w", err)
			}

			// 5. Run the Pipeline
			p := pipeline.New(reg, cfg)

			results, err := p.Execute(cmd.Context(), q)
			if err != nil {
				return fmt.Errorf("pipeline execution failed: %w", err)
			}

			// 6. Display Results
			for i, r := range results {
				fmt.Print(render.Result(r, q, i+1, explain) + "\n")
			}

			return nil
		},
	}
	searchCmd.Flags().BoolVarP(&explain, "explain", "e", false, "Show score breakdown for each result")

	rootCmd.AddCommand(searchCmd)

	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication credentials",
	}
	rootCmd.AddCommand(authCmd)

	authSetKeyCmd := &cobra.Command{
		Use:   "set-key",
		Short: "Set the Tavily API key in the OS keychain",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.DefaultPath()
			if err != nil {
				return fmt.Errorf("failed to get default config path: %w", err)
			}

			cfg, err := config.Load(path)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			fmt.Print("Enter Tavily API Key: ")
			keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println() // newline after password input
			if err != nil {
				return fmt.Errorf("failed to read API key: %w", err)
			}

			if err := secrets.SetAPIKey(cfg.Search.KeychainService, cfg.Search.KeychainAccount, string(keyBytes)); err != nil {
				return fmt.Errorf("failed to save API key: %w", err)
			}

			fmt.Println("API key saved successfully to keychain.")
			return nil
		},
	}
	authCmd.AddCommand(authSetKeyCmd)

	return rootCmd
}

// main is the entrypoint for the Enigma CLI.
// It sets up the Cobra command structure and wires together the core dependencies.
func main() {
	rootCmd := buildRootCmd()
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
