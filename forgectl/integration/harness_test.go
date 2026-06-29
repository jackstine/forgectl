//go:build integration

// Package integration drives the real, built forgectl binary end-to-end over a
// real filesystem and git repo. It is the black-box CLI layer the integration
// test plan calls for: tests here exercise main.go, the cobra command layer,
// go:embed assets, and the on-disk side effects (state file, moved/created
// files, git commits, activity logs) — none of which the in-process unit tests
// touch.
//
// The whole package is gated behind the `integration` build tag so the fast
// unit suite (`go test ./...`) is unaffected. Run it with:
//
//	go test -tags=integration ./integration/...
//
// §1a isolation is baked into the shared fixture (NewProject): every project
// gets its own sandbox HOME (so the activity logger and PruneLogs never touch
// the real ~/.forgectl/logs) and a repo-local git identity (so commits don't
// depend on the developer's global gitconfig). No individual test can forget
// it because the fixture is the only way in.
package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"forgectl/state"
)

// binPath is the absolute path to the forgectl binary built once in TestMain.
var binPath string

// updateGolden mirrors the conventional `-update` flag for golden snapshots.
var updateGolden = flag.Bool("update", false, "update golden files")

// TestMain builds the forgectl binary once (exercising the real build, main.go,
// and all go:embed assets) and shares it across every test in the package.
func TestMain(m *testing.M) {
	flag.Parse()

	tmp, err := os.MkdirTemp("", "forgectl-bin-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: mktemp: %v\n", err)
		os.Exit(1)
	}

	bin := filepath.Join(tmp, "forgectl")
	// The integration package lives at forgectl/integration; the module root —
	// where `go build .` produces the binary — is one directory up.
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: abs module root: %v\n", err)
		os.Exit(1)
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = moduleRoot
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "integration: building forgectl: %v\n", err)
		os.Exit(1)
	}
	binPath = bin

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// Project is a fully isolated forgectl project rooted in t.TempDir(), with its
// own sandbox HOME and a git repo carrying a repo-local identity.
type Project struct {
	t    *testing.T
	Root string // project root (contains .forgectl/, the git repo)
	Home string // sandbox HOME for ~/.forgectl/logs and git's global config

	// StateDirRel is where the state file lives relative to Root. It defaults to
	// the config default; override it before driving when a test sets a custom
	// paths.state_dir.
	StateDirRel string
}

// NewProject builds an isolated project with a git repo and the §1a sandboxes.
// It does NOT write a config — call Init/WriteConfig, or let a bare `init`
// scaffold one, depending on what the test exercises.
func NewProject(t *testing.T) *Project {
	t.Helper()
	p := &Project{
		t:           t,
		Root:        t.TempDir(),
		Home:        t.TempDir(),
		StateDirRel: filepath.FromSlash(".forgectl/state"),
	}
	p.gitInit()
	return p
}

// gitInit initialises the repo and pins a repo-local identity so commits never
// depend on (or touch) the developer's global git config.
func (p *Project) gitInit() {
	p.t.Helper()
	p.git("init", "-q")
	p.git("config", "user.email", "forgectl-test@example.com")
	p.git("config", "user.name", "forgectl test")
	p.git("config", "commit.gpgsign", "false")
}

// git runs a git command in the project root with the sandbox HOME, failing the
// test on error. Used for fixture setup and trifecta inspection.
func (p *Project) git(args ...string) string {
	p.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = p.Root
	cmd.Env = p.env()
	out, err := cmd.CombinedOutput()
	if err != nil {
		p.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// env returns a minimal child environment with HOME redirected to the sandbox.
// Any inherited HOME/USERPROFILE is stripped first so the override actually wins
// regardless of libc getenv ordering.
func (p *Project) env() []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "HOME=") || strings.HasPrefix(kv, "USERPROFILE=") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "HOME="+p.Home, "USERPROFILE="+p.Home)
	return env
}

// Result captures one forgectl invocation's three side-channels: stdout, stderr,
// and the exit code, each separately (never merged).
type Result struct {
	Stdout string
	Stderr string
	Exit   int
}

// Out returns stdout+stderr concatenated, for convenience when a test just wants
// to search for a cue regardless of stream.
func (r Result) Out() string { return r.Stdout + r.Stderr }

// forge runs the built binary in the project root with the isolated env and
// returns the captured trifecta. A failure to even start the process (vs. a
// non-zero exit) fails the test.
func (p *Project) forge(args ...string) Result {
	return p.forgeAt(p.Root, args...)
}

// forgeAt is forge with an explicit working directory, for tests that drive the
// binary from a subdirectory (e.g. project-root discovery across a git
// boundary). The sandbox HOME is still applied.
func (p *Project) forgeAt(dir string, args ...string) Result {
	p.t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	cmd.Env = p.env()
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	res := Result{Stdout: out.String(), Stderr: errb.String()}
	if err == nil {
		res.Exit = 0
	} else if exitErr, ok := err.(*exec.ExitError); ok {
		res.Exit = exitErr.ExitCode()
	} else {
		p.t.Fatalf("forge %s: failed to run: %v", strings.Join(args, " "), err)
	}
	return res
}

// mustForge runs forge and fails the test if the exit code is non-zero. Returns
// the result for further assertions.
func (p *Project) mustForge(args ...string) Result {
	p.t.Helper()
	res := p.forge(args...)
	if res.Exit != 0 {
		p.t.Fatalf("forge %s: expected success, got exit %d\nstdout:\n%s\nstderr:\n%s",
			strings.Join(args, " "), res.Exit, res.Stdout, res.Stderr)
	}
	return res
}

// --- Fixture helpers ---

// WriteFile writes content to a path relative to the project root, creating
// parent directories as needed.
func (p *Project) WriteFile(rel, content string) {
	p.t.Helper()
	full := filepath.Join(p.Root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		p.t.Fatal(err)
	}
}

// WriteConfig writes a .forgectl/config (TOML on disk — not a Go struct).
func (p *Project) WriteConfig(toml string) {
	p.t.Helper()
	p.WriteFile(".forgectl/config", toml)
}

// Path joins a project-relative path to the root and returns the absolute path.
func (p *Project) Path(rel string) string {
	return filepath.Join(p.Root, filepath.FromSlash(rel))
}

// Exists reports whether a project-relative path exists.
func (p *Project) Exists(rel string) bool {
	_, err := os.Stat(p.Path(rel))
	return err == nil
}

// ReadFile reads a project-relative file, failing the test if it is missing.
func (p *Project) ReadFile(rel string) string {
	p.t.Helper()
	data, err := os.ReadFile(p.Path(rel))
	if err != nil {
		p.t.Fatal(err)
	}
	return string(data)
}

// --- Trifecta inspection: state ---

// State reads and unmarshals the persisted forgectl-state.json. Reading the file
// through the same struct forgectl writes is acceptable here: these are *driver*
// assertions (did the persisted phase/state/counters land correctly), not the
// contract/oracle tests, which derive their expectations independently.
func (p *Project) State() *state.ForgeState {
	p.t.Helper()
	data := p.ReadFile(filepath.Join(p.StateDirRel, "forgectl-state.json"))
	var s state.ForgeState
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		p.t.Fatalf("parsing state file: %v", err)
	}
	return &s
}

// RawState reads the state file into a generic map, for assertions that want to
// avoid coupling to the Go struct shape (e.g. checking a section is JSON null).
func (p *Project) RawState() map[string]json.RawMessage {
	p.t.Helper()
	data := p.ReadFile(filepath.Join(p.StateDirRel, "forgectl-state.json"))
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		p.t.Fatalf("parsing raw state file: %v", err)
	}
	return m
}

// StateExists reports whether the state file is present.
func (p *Project) StateExists() bool {
	return p.Exists(filepath.Join(p.StateDirRel, "forgectl-state.json"))
}

// AssertAt fails unless the persisted state is at the given phase and state.
func (p *Project) AssertAt(phase state.PhaseName, st state.StateName) {
	p.t.Helper()
	s := p.State()
	if s.Phase != phase || s.State != st {
		p.t.Fatalf("state = %s/%s, want %s/%s", s.Phase, s.State, phase, st)
	}
}

// --- Trifecta inspection: git ---

// GitLog returns the one-line commit subjects, newest first (empty if none).
func (p *Project) GitLog() []string {
	p.t.Helper()
	cmd := exec.Command("git", "log", "--pretty=%s")
	cmd.Dir = p.Root
	cmd.Env = p.env()
	out, err := cmd.Output()
	if err != nil {
		// No commits yet → git exits non-zero; treat as empty history.
		return nil
	}
	return splitNonEmpty(string(out))
}

// GitShowStat returns the --stat output for HEAD (the staged paths of the last
// commit), for asserting commit_strategy staging.
func (p *Project) GitShowStat() string {
	p.t.Helper()
	return p.git("show", "--stat", "--pretty=format:", "HEAD")
}

// --- Trifecta inspection: activity logs ---

// LogFiles returns the basenames of the activity-log files in the sandbox HOME.
func (p *Project) LogFiles() []string {
	p.t.Helper()
	dir := filepath.Join(p.Home, ".forgectl", "logs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// LogEntries parses one activity-log file (newline-delimited JSON) from the
// sandbox HOME into generic maps.
func (p *Project) LogEntries(name string) []map[string]any {
	p.t.Helper()
	path := filepath.Join(p.Home, ".forgectl", "logs", name)
	data, err := os.ReadFile(path)
	if err != nil {
		p.t.Fatalf("reading log %s: %v", name, err)
	}
	var entries []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			p.t.Fatalf("parsing log line %q: %v", line, err)
		}
		entries = append(entries, m)
	}
	return entries
}

// --- Output parsing helpers ---

// reportPathRe matches the exact-path line forgectl prints in eval states, e.g.
//
//	The sub-agent must write its report to this exact path:
//	  core/specs/.eval/batch-1-r1.md
var reportPathRe = regexp.MustCompile(`write its report to this exact path:\s*\n\s*(\S+)`)

// ExtractReportPath pulls the eval-report path forgectl tells the agent to write
// (and that --eval-report must point at) out of an EVALUATE-family output. Tests
// extract the path rather than hardcode it, because it varies by eval kind and
// domain (batch vs cross-reference vs reconciliation).
func (p *Project) ExtractReportPath(out string) string {
	p.t.Helper()
	m := reportPathRe.FindStringSubmatch(out)
	if m == nil {
		p.t.Fatalf("no eval-report path found in output:\n%s", out)
	}
	return m[1]
}

// WriteReport creates a stub eval-report file at a project-relative path
// (creating parent dirs) so a subsequent --eval-report advance can consume it.
func (p *Project) WriteReport(rel string) {
	p.t.Helper()
	p.WriteFile(rel, "# stub eval report\nPASS\n")
}

// PassEval, given an EVALUATE-family Result, extracts the required report path,
// writes a stub report there, and advances with the given verdict + that report.
// Returns the advance Result. This collapses the eval two-actor handshake (eval
// emits path → agent writes file → advance consumes path) into one call for the
// many lifecycle steps that just need to get past an eval gate.
func (p *Project) PassEval(prev Result, verdict string) Result {
	p.t.Helper()
	path := p.ExtractReportPath(prev.Out())
	p.WriteReport(path)
	return p.mustForge("advance", "--verdict", verdict, "--eval-report", path)
}

// --- misc ---

func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}
