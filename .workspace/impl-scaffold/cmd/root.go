package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var dir string

var rootCmd = &cobra.Command{
	Use:   "impl-scaffold",
	Short: "Implementation plan lifecycle manager",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dir, "dir", ".", "Directory containing state file")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
