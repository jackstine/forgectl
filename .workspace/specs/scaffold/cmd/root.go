package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var stateDir string

var rootCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Spec generation lifecycle manager",
	Long:  "Tracks spec generation state through a JSON-backed state machine with validated input and deterministic transitions.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&stateDir, "dir", ".", "Directory containing the scaffold state file")
}
