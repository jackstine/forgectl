# Release Plan: Publish forgectl binaries via GitHub Releases

**Goal:** Produce a working GitHub Release (`v0.0.2`) that attaches multi-architecture
binaries, built reproducibly by CI, plus a `curl | sh` installer that fetches the right
archive for the user's OS/CPU.

**Repo:** `jackstine/forgectl` · **Go module lives in:** `forgectl/` subdirectory
**Target distribution:** GitHub release archives (tar.gz / zip) + `scripts/install/install.sh`

---

## Current State (audited 2026-06-09)

| Item | Status |
|---|---|
| `.goreleaser.yaml` (root) | Exists, **but broken** for this repo layout |
| `.github/workflows/release.yml` | Exists, triggers on `v*` tags — **has never run** |
| Tag `v0.0.1` | Pushed to origin, but **no release published** (predates workflow) |
| Published releases | **None** (`gh release list` empty) |
| Version wiring | `forgectl/cmd/root.go:12` hardcodes `var version = "v0.0.1"` |

### Blocking bugs found

1. **Subdirectory module.** GoReleaser runs `go build` from the directory containing
   `.goreleaser.yaml` (repo root), but there is **no `go.mod` at the root** — it lives in
   `forgectl/`. The current `builds.main: ./forgectl` will fail to build.
2. **Wrong ldflags target.** Config injects `-X main.version=...`, but the version variable
   is `forgectl/cmd.version`, not `main.version`. The injected version is silently ignored,
   and because `root.go` hardcodes `"v0.0.1"`, every build reports `v0.0.1` regardless of tag.

---

## Plan

### Step 1 — Wire version into the binary correctly
**File:** `forgectl/cmd/root.go`
- Change `var version = "v0.0.1"` → `var version = "dev"` and add `commit` / `date` vars:
  ```go
  var (
      version = "dev"
      commit  = "none"
      date    = "unknown"
  )
  ```
- Surface commit/date in `--version` output (extend cobra `Version` template) so released
  binaries report real build metadata; local `make build` continues to show `dev`.

### Step 2 — Fix `.goreleaser.yaml` for the subdirectory layout
**File:** `.goreleaser.yaml`
- Add `dir: forgectl` to the build and set `main: .` so GoReleaser runs `go build` inside the
  module directory.
- Repoint ldflags to the real package:
  ```yaml
  ldflags:
    - -s -w
    - -X forgectl/cmd.version={{.Version}}
    - -X forgectl/cmd.commit={{.Commit}}
    - -X forgectl/cmd.date={{.CommitDate}}
  ```
- Keep existing matrix (no change needed — already correct):
  - **OS:** linux, darwin, windows
  - **Arch:** amd64, arm64
  - **Ignored:** windows/arm64
  - → 5 binaries: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64
- Keep `CGO_ENABLED=0`, `-trimpath`, `mod_timestamp`, sha256 `checksums.txt`, archive naming.
- Archive `files:` (LICENSE, README.md, CHANGELOG.md) resolve from project root — leave as-is.

### Step 3 — Harden the release workflow
**File:** `.github/workflows/release.yml`
- Add `workflow_dispatch:` so the release can be triggered manually from the Actions tab
  (useful for re-runs without re-tagging).
- Pin Go to the module's version (`go-version-file: forgectl/go.mod`) instead of `stable`,
  for reproducibility.
- Keep `permissions: contents: write` and the `GITHUB_TOKEN` env (sufficient for archives;
  no extra secrets needed since we're not doing Homebrew).

### Step 4 — Validate locally before tagging
- Install goreleaser locally (`brew install goreleaser` — not currently installed).
- Run `goreleaser check` (config valid) and `goreleaser build --snapshot --clean`
  (full cross-compile, no publish). Confirm all 5 binaries build and `./dist/.../forgectl
  --version` reports the snapshot version + commit.
- Run `make build` to confirm the local dev path still works and prints `dev`.

### Step 5 — Author the installer script
**File:** `scripts/install/install.sh` (the `scripts/install/` dir already exists, empty)
- POSIX `sh` script that:
  1. Detects OS (`uname -s` → linux/darwin) and arch (`uname -m` → amd64/arm64).
  2. Resolves the latest release tag via the GitHub API (or honors `FORGECTL_VERSION`).
  3. Downloads the matching `forgectl_<ver>_<os>_<arch>.tar.gz` + `checksums.txt`.
  4. Verifies the sha256 checksum.
  5. Extracts `forgectl` to `~/.local/bin` (override via `BIN_DIR`), `chmod +x`.
  6. Prints a PATH hint if the bin dir isn't on `$PATH`.
- Add usage to `README.md`:
  `curl -fsSL https://raw.githubusercontent.com/jackstine/forgectl/main/scripts/install/install.sh | sh`

### Step 6 — Update CHANGELOG
**File:** `CHANGELOG.md`
- Add a `## [0.0.2] - 2026-06-09` section describing the first published, installable release.
  (Existing 0.0.1 entry has a placeholder `2025-01-01` date; leave it or correct it.)

### Step 7 — Commit, merge, tag, release
- Current branch is `reverse_engineer_simple`. Commit Steps 1–6 there, then merge to `main`
  (release workflow checks out the tag; tagging `main` is cleanest).
- Create and push the tag:
  ```bash
  git tag -a v0.0.2 -m "v0.0.2: first published release with binaries"
  git push origin v0.0.2
  ```
- The push triggers `release.yml` → GoReleaser builds the 5 binaries, generates
  `checksums.txt`, and publishes the **v0.0.2** GitHub Release with all archives attached.

### Step 8 — Verify the published release
- `gh release view v0.0.2 -R jackstine/forgectl` — confirm 5 archives + checksums attached.
- Run the installer end-to-end on this machine (darwin/arm64) and confirm
  `forgectl --version` prints `v0.0.2` with the real commit hash.

---

## Deliverables / files touched
- `forgectl/cmd/root.go` — version/commit/date vars (source change, required for versioning)
- `.goreleaser.yaml` — fix subdir build + ldflags
- `.github/workflows/release.yml` — workflow_dispatch + pinned Go
- `scripts/install/install.sh` — new installer
- `README.md` — install instructions
- `CHANGELOG.md` — 0.0.2 entry
- New tag `v0.0.2` → published GitHub Release

## Out of scope (per decision)
- Homebrew tap / formula
- Package-manager publishing (apt/scoop/etc.)

## Risks / notes
- Step 1 edits source (`root.go`). This is required to make versioning work and is unrelated
  to spec work — flagging because of the repo's "no source edits during spec sessions" rule.
- If `goreleaser check` reveals the `dir:` field behaves differently in v2, the fallback is to
  add a thin root `go.mod`/workspace or a pre-build `cd`. Local validation (Step 4) catches
  this before any tag is pushed.
