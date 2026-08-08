package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var preflightFrom string

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Report planning readiness for the domains in a plan queue",
	Long: "Reports whether every domain a planning cycle is about to plan has an empty workspace.\n\n" +
		"preflight is a read-only query: it writes no state file, creates no directories, and\n" +
		"deletes nothing. Run it as the first step of a planning cycle to find out whether the\n" +
		"workspace close-out procedure is required before advancing.",
	Args: cobra.NoArgs,
	RunE: runPreflight,
}

func init() {
	preflightCmd.Flags().StringVar(&preflightFrom, "from", "", "Plan queue JSON file to check (defaults to the active session's pending plan queue)")
	rootCmd.AddCommand(preflightCmd)
}

// noPlanQueueMessage is the exact wording the spec defines for an invocation
// with nothing to check.
const noPlanQueueMessage = "No plan queue to check. Pass --from <plan-queue.json> or run from a session with a generated plan queue."

func runPreflight(cmd *cobra.Command, args []string) error {
	projectRoot, cfg, queuePath, err := resolvePreflightQueue()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(queuePath)
	if err != nil {
		return fmt.Errorf("reading plan queue %s: %w", queuePath, err)
	}
	if errs := state.ValidatePlanQueue(data); len(errs) > 0 {
		return fmt.Errorf("invalid plan queue %s: %s", queuePath, errs[0])
	}

	var queue state.PlanQueueInput
	if err := json.Unmarshal(data, &queue); err != nil {
		return fmt.Errorf("invalid plan queue %s: %w", queuePath, err)
	}

	// No logging here, deliberately. The gate's activity-log entries belong to
	// the three cold-start planning *entry* points, where the verdict decides
	// whether a cycle starts. preflight only asks the question — it can be run
	// any number of times, from outside any session, and filling the log with
	// answers nobody acted on would bury the entries that did gate something.
	// The verdict goes to stdout and nowhere else.
	verdict := state.EvaluateReadiness(projectRoot, cfg, state.QueueDomains(queue))
	if verdict.Ready {
		fmt.Fprintln(cmd.OutOrStdout(), verdict.Render())
		return nil
	}
	// A blocked verdict is a failure of the query, so it goes out as an error:
	// that is what produces the non-zero exit and routes the text to stderr,
	// matching every other forgectl error path.
	return fmt.Errorf("%s", verdict.Render())
}

// resolvePreflightQueue determines which plan queue to check and the project
// context to check it against.
//
// With --from, no session — and no initialized project — is required: the
// operator runs preflight before init, precisely to find out whether init would
// be refused. So the project root is discovered rather than established:
// state.Scaffold would give one, but it creates .forgectl/ and a default config
// when they are absent, and preflight must create nothing. Falling back to the
// working directory with default config keeps the command usable in a project
// that has never been initialized while leaving the filesystem untouched.
//
// Without --from, the pending plan queue recorded by generate_planning_queue is
// the only other source.
func resolvePreflightQueue() (projectRoot string, cfg state.ForgeConfig, queuePath string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", cfg, "", err
	}

	if preflightFrom != "" {
		projectRoot, err = state.FindProjectRoot(cwd)
		if err != nil {
			// No .forgectl/ anywhere above cwd: treat cwd as the root and use
			// the default workspace_dir. Nothing is written.
			return cwd, state.DefaultForgeConfig(), preflightFrom, nil
		}
		cfg, err = state.LoadConfig(projectRoot)
		if err != nil {
			// A project root with no readable config still resolves — the gate
			// only needs workspace_dir and the domain list, both of which the
			// defaults supply.
			cfg = state.DefaultForgeConfig()
		}
		return projectRoot, cfg, preflightFrom, nil
	}

	projectRoot, stateDir, cfg, err := resolveSession()
	if err != nil || !state.Exists(stateDir) {
		return "", cfg, "", fmt.Errorf("%s", noPlanQueueMessage)
	}
	s, err := state.Load(stateDir)
	if err != nil {
		return "", cfg, "", fmt.Errorf("%s", noPlanQueueMessage)
	}
	if s.GeneratePlanningQueue == nil || s.GeneratePlanningQueue.PlanQueueFile == "" {
		return "", cfg, "", fmt.Errorf("%s", noPlanQueueMessage)
	}

	// The recorded path is project-root-relative.
	queuePath = s.GeneratePlanningQueue.PlanQueueFile
	if !filepath.IsAbs(queuePath) {
		queuePath = filepath.Join(projectRoot, queuePath)
	}
	return projectRoot, cfg, queuePath, nil
}
