package cmd

import (
	"fmt"
	"os"
	"strings"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var (
	addQueueItemName    string
	addQueueItemDomain  string
	addQueueItemTopic   string
	addQueueItemFile    string
	addQueueItemSources []string
)

var addQueueItemCmd = &cobra.Command{
	Use:   "add-queue-item",
	Short: "Add a spec to the specifying queue",
	RunE:  runAddQueueItem,
}

func init() {
	addQueueItemCmd.Flags().StringVar(&addQueueItemName, "name", "", "Spec name (required)")
	addQueueItemCmd.Flags().StringVar(&addQueueItemDomain, "domain", "", "Domain name (required at DONE, inferred elsewhere)")
	addQueueItemCmd.Flags().StringVar(&addQueueItemTopic, "topic", "", "Spec topic (required)")
	addQueueItemCmd.Flags().StringVar(&addQueueItemFile, "file", "", "Path to spec file (required)")
	addQueueItemCmd.Flags().StringArrayVar(&addQueueItemSources, "source", nil, "Planning source path (repeatable)")
	_ = addQueueItemCmd.MarkFlagRequired("name")
	_ = addQueueItemCmd.MarkFlagRequired("topic")
	_ = addQueueItemCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(addQueueItemCmd)
}

func runAddQueueItem(cmd *cobra.Command, args []string) error {
	_, stateDir, _, err := resolveSession()
	if err != nil {
		return err
	}
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	// Phase check.
	if s.Phase != state.PhaseSpecifying {
		return fmt.Errorf("add-queue-item is only valid in the specifying phase (current phase: %s)", s.Phase)
	}

	// State check.
	validStates := map[state.StateName]bool{
		state.StateDraft:              true,
		state.StateCrossReferenceReview: true,
		state.StateDone:               true,
		state.StateReconcileReview:    true,
	}
	if !validStates[s.State] {
		return fmt.Errorf("add-queue-item is not valid in state %s", s.State)
	}

	// File existence check.
	if _, err := os.Stat(addQueueItemFile); err != nil {
		return fmt.Errorf("file %s does not exist. add-queue-item registers specs that have already been written. Create the spec file first, then register it.", addQueueItemFile)
	}

	// Domain inference.
	domain := addQueueItemDomain
	if domain == "" {
		switch s.State {
		case state.StateDone, state.StateReconcileReview:
			return fmt.Errorf("--domain is required at state %s", s.State)
		case state.StateDraft:
			if len(s.Specifying.CurrentSpecs) > 0 {
				domain = s.Specifying.CurrentSpecs[0].Domain
			}
		case state.StateCrossReferenceReview:
			domain = s.Specifying.CurrentDomain
		}
	}
	if domain == "" {
		return fmt.Errorf("--domain is required (cannot infer domain from current state)")
	}

	// Name uniqueness check.
	for _, q := range s.Specifying.Queue {
		if strings.EqualFold(q.Name, addQueueItemName) {
			return fmt.Errorf("spec name %q already exists in queue", addQueueItemName)
		}
	}
	for _, c := range s.Specifying.Completed {
		if strings.EqualFold(c.Name, addQueueItemName) {
			return fmt.Errorf("spec name %q already exists in completed specs", addQueueItemName)
		}
	}

	entry := state.SpecQueueEntry{
		Name:            addQueueItemName,
		Domain:          domain,
		Topic:           addQueueItemTopic,
		File:            addQueueItemFile,
		PlanningSources: addQueueItemSources,
	}
	s.Specifying.Queue = append(s.Specifying.Queue, entry)

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added %q to queue (domain: %s).\n", addQueueItemName, domain)
	return nil
}
