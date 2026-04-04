package cmd

import (
	"fmt"
	"os"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var version = "v0.0.1"

var rootCmd = &cobra.Command{
	Use:          "forgectl",
	Short:        "Software development lifecycle scaffold",
	Long:         "Manages the full software development lifecycle — specifying, planning, implementing — through a JSON-backed state machine.",
	Version:      version,
	SilenceUsage: true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// resolveSession discovers the project root, loads config, and returns
// (projectRoot, stateDir, cfg). projectRoot resolves relative paths in state;
// stateDir is used for Load/Save/Exists; cfg is the loaded project configuration.
func resolveSession() (projectRoot, stateDir string, cfg state.ForgeConfig, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	projectRoot, err = state.FindProjectRoot(cwd)
	if err != nil {
		return
	}
	cfg, err = state.LoadConfig(projectRoot)
	if err != nil {
		return
	}
	stateDir = state.StateDir(projectRoot, cfg)
	return
}
