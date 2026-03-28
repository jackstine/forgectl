package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var stateDir string

var version = "v0.0.1"

var rootCmd = &cobra.Command{
	Use:     "forgectl",
	Short:   "Software development lifecycle scaffold",
	Long:    "Manages the full software development lifecycle — specifying, planning, implementing — through a JSON-backed state machine.",
	Version: version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&stateDir, "dir", ".", "Directory containing the state file")
}
