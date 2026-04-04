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

var validateCmd = &cobra.Command{
	Use:   "validate <file_path>",
	Short: "Validate a spec-queue, plan-queue, or plan JSON file",
	Args:  cobra.ExactArgs(1),
	RunE:  runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateType, "type", "", "File type: spec-queue, plan-queue, or plan")
	rootCmd.AddCommand(validateCmd)
}

// topKeyType maps a top-level JSON key to a known file type name.
func topKeyType(key string) string {
	switch key {
	case "specs":
		return "spec-queue"
	case "plans":
		return "plan-queue"
	case "context":
		return "plan"
	}
	return ""
}

// typeExpectedKey returns the expected top-level JSON key for a file type.
func typeExpectedKey(t string) string {
	switch t {
	case "spec-queue":
		return "specs"
	case "plan-queue":
		return "plans"
	case "plan":
		return "context"
	}
	return ""
}

// didYouMean returns a --type hint based on the found key.
func didYouMean(foundKey string) string {
	t := topKeyType(foundKey)
	if t != "" {
		return fmt.Sprintf("  Hint: did you mean --type %s?", t)
	}
	return ""
}

// jsonErrorWithLocation converts a json.Unmarshal error to a string with
// line and column numbers derived from the raw input bytes.
func jsonErrorWithLocation(data []byte, err error) string {
	var offset int64
	switch e := err.(type) {
	case *json.SyntaxError:
		offset = e.Offset
	case *json.UnmarshalTypeError:
		offset = e.Offset
	default:
		return err.Error()
	}
	// Clamp offset to valid range.
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	line, col := 1, 1
	for i := int64(0); i < offset-1; i++ {
		if data[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	// Strip the redundant offset suffix Go appends to SyntaxError messages.
	msg := err.Error()
	return fmt.Sprintf("at line %d, column %d: %s", line, col, msg)
}

func runValidate(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	out := cmd.OutOrStdout()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", filePath)
		}
		return fmt.Errorf("reading file: %w", err)
	}

	// Parse just the top-level keys to detect/verify type.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid JSON: %s", jsonErrorWithLocation(data, err))
	}

	// Find the first known key (in priority order).
	detectedKey := ""
	for _, k := range []string{"specs", "plans", "context"} {
		if _, ok := raw[k]; ok {
			detectedKey = k
			break
		}
	}

	fileType := validateType

	if fileType == "" {
		// Auto-detect.
		if detectedKey == "" {
			// Find any top-level key for the error message.
			found := ""
			for k := range raw {
				found = k
				break
			}
			fmt.Fprintf(out, "Error: cannot detect file type.\n")
			fmt.Fprintf(out, "  Expected one of these top-level keys:\n")
			fmt.Fprintf(out, "    \"specs\"    → spec-queue\n")
			fmt.Fprintf(out, "    \"plans\"    → plan-queue\n")
			fmt.Fprintf(out, "    \"context\"  → plan\n")
			if found != "" {
				fmt.Fprintf(out, "  Found: %q\n", found)
			}
			fmt.Fprintf(out, "  Hint: use --type to specify the file type explicitly.\n")
			return fmt.Errorf("cannot detect file type")
		}
		fileType = topKeyType(detectedKey)
		fmt.Fprintf(out, "Detected: %s (top-level key: %q)\n\n", fileType, detectedKey)
	} else {
		// Verify --type matches actual key.
		expected := typeExpectedKey(fileType)
		if expected == "" {
			return fmt.Errorf("--type must be spec-queue, plan-queue, or plan (got %q)", fileType)
		}
		if detectedKey != expected {
			errMsg := fmt.Sprintf("Error: --type %s expects top-level key %q, found %q.", fileType, expected, detectedKey)
			hint := didYouMean(detectedKey)
			if hint != "" {
				fmt.Fprintf(out, "%s\n%s\n", errMsg, hint)
			} else {
				fmt.Fprintf(out, "%s\n", errMsg)
			}
			return fmt.Errorf("type mismatch")
		}
		fmt.Fprintf(out, "Detected: %s (top-level key: %q)\n\n", fileType, detectedKey)
	}

	// Run validation.
	var errs []string
	var entryCount int
	var entryLabel string

	switch fileType {
	case "spec-queue":
		errs = state.ValidateSpecQueue(data)
		if len(errs) == 0 {
			var input state.SpecQueueInput
			json.Unmarshal(data, &input)
			entryCount = len(input.Specs)
		}
		entryLabel = "entries"
	case "plan-queue":
		errs = state.ValidatePlanQueue(data)
		if len(errs) == 0 {
			var input state.PlanQueueInput
			json.Unmarshal(data, &input)
			entryCount = len(input.Plans)
		}
		entryLabel = "entries"
	case "plan":
		baseDir := filepath.Dir(filePath)
		errs = state.ValidatePlanJSON(data, baseDir)
	}

	filename := filepath.Base(filePath)

	if len(errs) == 0 {
		if fileType == "plan" {
			fmt.Fprintf(out, "Validated: %s — no errors.\n", filename)
		} else {
			fmt.Fprintf(out, "Validated: %s — %d %s, no errors.\n", filename, entryCount, entryLabel)
		}
		return nil
	}

	fmt.Fprintf(out, "Error: validation failed with %d error(s):\n", len(errs))
	for i, e := range errs {
		fmt.Fprintf(out, "  %d. %s\n", i+1, e)
	}
	return fmt.Errorf("validation failed")
}
