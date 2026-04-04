package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var validateType string

// osExit is a variable so tests can override it to avoid calling os.Exit.
var osExit = os.Exit

var validateCmd = &cobra.Command{
	Use:   "validate <file>",
	Short: "Validate a JSON file (spec-queue, plan-queue, or plan)",
	Args:  cobra.ExactArgs(1),
	RunE:  runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateType, "type", "", "File type override: spec-queue, plan-queue, plan")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	file := args[0]

	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", file, err)
	}

	fileType := validateType
	if fileType == "" {
		fileType, err = detectFileType(data)
		if err != nil {
			return fmt.Errorf("cannot detect file type: %w", err)
		}
	}

	var errs []string
	switch fileType {
	case "spec-queue":
		errs = state.ValidateSpecQueue(data)
	case "plan-queue":
		errs = state.ValidatePlanQueue(data)
	case "plan":
		baseDir := filepath.Dir(file)
		errs = state.ValidatePlanJSON(data, baseDir)
	default:
		return fmt.Errorf("unknown type %q: must be spec-queue, plan-queue, or plan", fileType)
	}

	out := cmd.OutOrStdout()
	base := filepath.Base(file)
	if len(errs) == 0 {
		fmt.Fprintf(out, "✓ %s: valid %s\n", base, fileType)
		return nil
	}

	fmt.Fprintf(out, "FAIL: %d errors in %s\n", len(errs), base)
	for _, e := range errs {
		fmt.Fprintf(out, "  %s\n", e)
	}
	osExit(1)
	return nil
}

// detectFileType inspects top-level JSON keys to determine the file type.
func detectFileType(data []byte) (string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	_, hasSpecs := top["specs"]
	_, hasPlans := top["plans"]
	_, hasContext := top["context"]
	_, hasItems := top["items"]
	_, hasLayers := top["layers"]

	switch {
	case hasSpecs:
		return "spec-queue", nil
	case hasPlans:
		return "plan-queue", nil
	case hasContext && hasItems && hasLayers:
		return "plan", nil
	default:
		return "", fmt.Errorf("cannot determine file type from JSON keys")
	}
}
