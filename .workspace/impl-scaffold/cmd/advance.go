package cmd

import (
	"fmt"
	"strings"

	"impl-scaffold/state"

	"github.com/spf13/cobra"
)

var (
	advFileFlag         string
	advVerdictFlag      string
	advMessageFlag      string
	advDeficienciesFlag string
	advFixedFlag        string
	advNotesFlag        string
)

var advanceCmd = &cobra.Command{
	Use:   "advance",
	Short: "Transition to the next state",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !state.Exists(dir) {
			return fmt.Errorf("no state file found. Run init first")
		}

		s, err := state.Load(dir)
		if err != nil {
			return err
		}

		var deficiencies []string
		if advDeficienciesFlag != "" {
			for _, d := range strings.Split(advDeficienciesFlag, ",") {
				d = strings.TrimSpace(d)
				if d != "" {
					deficiencies = append(deficiencies, d)
				}
			}
		}

		in := state.AdvanceInput{
			Notes:        advNotesFlag,
			File:         advFileFlag,
			Verdict:      advVerdictFlag,
			Message:      advMessageFlag,
			Deficiencies: deficiencies,
			Fixed:        advFixedFlag,
		}

		if err := state.Advance(s, in); err != nil {
			return err
		}

		if err := state.Save(dir, s); err != nil {
			return err
		}

		cmd.Println(state.FormatState(s))
		return nil
	},
}

func init() {
	advanceCmd.Flags().StringVar(&advFileFlag, "file", "", "Override plan file path (DRAFT only)")
	advanceCmd.Flags().StringVar(&advVerdictFlag, "verdict", "", "PASS or FAIL (EVALUATE only)")
	advanceCmd.Flags().StringVar(&advMessageFlag, "message", "", "Commit message (required with PASS)")
	advanceCmd.Flags().StringVar(&advDeficienciesFlag, "deficiencies", "", "Comma-separated deficiency names")
	advanceCmd.Flags().StringVar(&advFixedFlag, "fixed", "", "Description of fixes (REFINE)")
	advanceCmd.Flags().StringVar(&advNotesFlag, "notes", "", "Study phase notes")
	rootCmd.AddCommand(advanceCmd)
}
