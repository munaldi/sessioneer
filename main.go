package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/munaldi/sessioneer/internal/config"
	"github.com/munaldi/sessioneer/internal/tui"
)

var (
	flagProvider string
	flagProject  string
	flagBase     string
)

var rootCmd = &cobra.Command{
	Use:   "sessioneer",
	Short: "✨ Interactive CLI to manage Claude Code and GitHub Copilot sessions",
	Long: `Sessioneer — browse, search, fork, merge, prune, trim, rename,
and delete AI coding sessions for Claude Code and GitHub Copilot.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Resolve(flagProvider, flagProject, flagBase)
		if err != nil {
			return fmt.Errorf("invalid arguments: %w", err)
		}

		model := tui.New(cfg.Provider, cfg.BaseDir)
		p := tea.NewProgram(model, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("UI error: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.Flags().StringVarP(&flagProvider, "provider", "P", "", `AI provider: "claude" or "copilot" (default: auto-detect)`)
	rootCmd.Flags().StringVarP(&flagProject, "project", "p", "", "Project path (default: current directory)")
	rootCmd.Flags().StringVarP(&flagBase, "base", "b", "", "Session base directory (default: provider default)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
