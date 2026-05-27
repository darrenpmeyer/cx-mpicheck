# CLI & CI usage

`cx-mpicheck` scans a directory tree for lockfiles, queries Checkmarx MPIAPI, and exits with a non-zero code when risks are found. All logs go to STDERR so JSON outputs remain clean for CI parsing.

Basic usage

```sh
export CHECKMARX_MPIAPI_KEY=YOUR_KEY
./cx-mpicheck --root . --verbose
```

CI focus

- Exit code `0`: no risks found.
- Exit code `22` (default): risks found (configurable).
- Exit codes `100-254`: errors that prevented completion (parse failures, IO errors, network issues, etc.).

Risk policy behavior

- By default, any risk triggers the non-zero exit code.
- You can narrow this with score rules and/or title matchers.
- When the risk report file already exists, the tool only evaluates the policy unless `--force-rescan` is set (even with the default policy).
- When a policy is specified, the risk report file is backed up and rewritten to include only matching risks.
- Summary logs report matched packages, total risks, packages with risks, and packages checked.

Suggested CI pattern

1) Make sure `CHECKMARX_MPIAPI_KEY` is available in the CI environment.
2) Run the tool in your repo root.
3) Fail the build when the exit code is the configured risk code.

Example (generic)

```sh
set -euo pipefail

export CHECKMARX_MPIAPI_KEY="$CHECKMARX_MPIAPI_KEY"
./cx-mpicheck --root . --out-risks cx.mpiapi-risks.json
```

Configuration

All options can be set via environment variables or command-line flags. Flags override environment variables. When both are set, a message is printed to STDERR indicating the override.

Required

- MPIAPI key: `CHECKMARX_MPIAPI_KEY`

Flags and environment variables

- `--root` / `CX_MPICHECK_ROOT`
  - Root directory to scan for lockfiles.
  - Default: `.`

- `--lockfile` / `CX_MPICHECK_INCLUDE`
  - Explicit lockfile path(s). Repeat the flag or use comma/semicolon-separated lists.
  - Example: `--lockfile package-lock.json --lockfile subdir/poetry.lock`

- `--fake-lockfile` / `CX_MPICHECK_FAKE_LOCKFILE`
  - Project definition file(s) to generate a fake lockfile from.
  - Example: `--fake-lockfile package.json`
  - You can force a specific manager with `manager:path` (e.g. `pnpm:package.json`).

- `--fake-lockfile-out` / `CX_MPICHECK_FAKE_LOCKFILE_OUT`
  - Output directory for generated fake lockfiles.
  - Default: same directory as the project definition file.

- `--fake-lockfile-cleanup` / `CX_MPICHECK_FAKE_LOCKFILE_CLEANUP`
  - Delete generated fake lockfiles when the tool exits (`true`/`false`).
  - Default: `false`

- `--resolve` / `CX_MPICHECK_RESOLVE`
  - Controls when to use local package manager tools to resolve package dependencies. Only creates fake temporary lockfiles, does not overwrite existing real ones 
    - `never` - only trust lockfiles, never resolve dependencies live
    - `demand` - trust lockfiles, resolve dependencies only when explicitly asked to via `--fake-lockfile`
    - `smart` - resolve dependencies live when no lockfile exists OR when `--fake-lockfile` requests it, use existing lockfiles otherwise
    - `always` - ignore any existing lockfiles and always resolve live (slower, more accurate)
  - Default: `demand`
  - Live resolution invokes external package managers (`npm`, `pnpm`, `pipenv`, `poetry`, `pip-compile`, `go mod tidy`) against project files in the tree. If those files can be attacker-controlled (e.g. PRs in a public-PR pipeline), see [security.md](security.md) for the trust model and recommended postures.

- `--include-mode` / `CX_MPICHECK_INCLUDE_MODE`
  - `only`: use only explicit lockfile paths; don't auto-detect
  - `also`: use explicit paths plus discover additional project and lockfiles under `--root`.
  - Default: `also`

- `--exclude` / `CX_MPICHECK_EXCLUDE`
  - Paths to omit from lockfile processing. Each entry is a file or directory.
  - Repeatable, and each value may itself be a list separated by `,` or `;`. Surrounding whitespace is trimmed and empty entries are dropped.
  - Relative paths are resolved against `--root`; absolute paths are used as-is.
  - If an entry resolves to an existing directory, every lockfile beneath it is skipped and the directory is pruned from the discovery walk (efficient — the tree below is never read). If the entry resolves to a file (or does not exist on disk as a directory), only that exact path is skipped.
  - Exclusions apply both to lockfiles found by discovery and to lockfiles supplied explicitly via `--lockfile` — an excluded path won't reappear just because it was named on the command line.
  - Symlinks are not honored for exclusion: a symlink named `vendor` pointing into the tree will not suppress its target. If you need to skip both, list both, or remove the link.
  - Example: `--exclude vendor,third_party --exclude test/fixtures/legacy.lock`

- `--out-packages` / `CX_MPICHECK_OUT_PACKAGES`
  - Package inventory output path.
  - Default: `cx.packages-master.json`

- `--out-results` / `CX_MPICHECK_OUT_RESULTS`
  - MPIAPI raw results output path.
  - Default: `cx.mpiapi-results.json`

- `--out-risks` / `CX_MPICHECK_OUT_RISKS`
  - Filtered risk output path.
  - Default: `cx.mpiapi-risks.json`

> The `--out-*` flags and `CX_MPICHECK_OUT_*` env vars are not validated for containment under `--root` and can write anywhere the CI user can write. Treat them like `PATH` — never source them from untrusted input. See [security.md](security.md) for the full posture and the owner-only file permissions cx-mpicheck applies to everything it writes.

- `--print-risks` / `CX_MPICHECK_PRINT_RISKS`
  - Print risk output to STDOUT (`true`/`false`).
  - Default: `false`

- `--risk-exit-code` / `CX_MPICHECK_RISK_EXIT_CODE`
  - Exit code used when risks are found.
  - Default: `22`

- `--risk-score` / `CX_MPICHECK_RISK_SCORE`
  - Risk score rule. Repeatable or comma/semicolon-separated.
  - Examples: `>5`, `5-9`, `10`

- `--risk-title` / `CX_MPICHECK_RISK_TITLE`
  - Case-insensitive substring to match risk titles/names. Repeatable or list format.

- `--force-rescan` / `CX_MPICHECK_FORCE_RESCAN`
  - Ignore existing risk report and rescan lockfiles (`true`/`false`).
  - Default: `false`: avoids re-scanning lockfiles that have already been scanned

- `--mpiapi-timeout` / `CX_MPICHECK_MPIAPI_TIMEOUT`
  - Per-request timeout for MPIAPI HTTP calls, as a Go duration string (e.g. `24s`, `1m30s`, `500ms`).
  - Default: `24s`. Set to `0` to disable the timeout (not recommended — a hung endpoint will then hang the run indefinitely).
  - Applied to the default HTTP client. Library callers that supply their own `cfg.HTTPClient` are responsible for setting that client's `Timeout` themselves.
  - Negative values are rejected with exit code `100`.

- `--batch-size` / `CX_MPICHECK_BATCH_SIZE`
  - MPIAPI batch size (1-1000).
  - Default: `1000`

- `--verbose` / `CX_MPICHECK_VERBOSE`
  - Enable verbose logging (`true`/`false`).
  - Default: `false`

- `--version`
  - Print version and exit.

- `--pretty`
  - Use colored, emoji-friendly output for key events.
  - The banner and policy summary use colored indicators in this mode.

Output files

- `cx.packages-master.json`: list of `{type,name,version,lockfile}` entries.
- `cx.mpiapi-results.json`: aggregated list of MPIAPI response entries.
- `cx.mpiapi-risks.json`: list of `{purl, risks, lockfiles}` for packages with risks (filtered to policy matches when a policy is set). `lockfiles` are reported relative to the scanned root.

Supported lockfiles

- npm: `package-lock.json`
- pnpm: `pnpm-lock.yaml` or `pnpm-lock.yml`
- pip: `requirements.txt`, `requirements.lock`, and `requirements*.txt`
- Pipenv: `Pipfile.lock`
- Poetry: `poetry.lock`
- Go: `go.mod`

Fake lockfile generation

- npm: `package.json` (generates `cx.npm.mpicheck.lock` by default)
- pnpm: `package.json` or `pnpm-workspace.yaml` (use `pnpm:package.json` to force pnpm)
- pip: `requirements.in`, `requirements.txt`, `setup.cfg`, `setup.ini`, or `pyproject.toml` (uses `pip-compile`, generates `cx.pip.mpicheck.lock`)
- pipenv: `Pipfile` (generates `cx.pipenv.mpicheck.lock`)
- poetry: `pyproject.toml` (generates `cx.poetry.mpicheck.lock`)
- go: `go.mod` (generates `cx.go.mpicheck.lock`)

Notes

- npm fake lockfile generation retries with `--legacy-peer-deps` if resolution fails.
- `pip` fake lockfile generation requires `pip-compile` from `pip-tools` on PATH.
- When a fake lockfile is generated, the tool ignores any real lockfile for that manager in the same directory and logs this to STDERR.
- Each lockfile examined is logged to STDERR.
 - Missing API keys are reported with an example command to set `CHECKMARX_MPIAPI_KEY`.

Resolve modes

- `never`: ignore fake lockfile generation requests; only scan existing lockfiles.
- `demand`: only generate fake lockfiles when `--fake-lockfile` (or env var) is provided.
- `smart`: like `demand`, but if no lockfiles are found it will auto-generate fake lockfiles from detected project files (this is logged each time).
- `always`: ignore existing lockfiles and only use generated fake lockfiles. Exception: Go (`go.mod`) is still used because it is the dependency source.

Examples

Only fail CI for high-scoring risks:

```sh
./cx-mpicheck --risk-score \">=7\" --risk-score \"10\"
```

Fail when title contains a keyword:

```sh
./cx-mpicheck --risk-title \"remote code execution\" --risk-title \"rce\"
```

Use existing risk file unless you force a rescan:

```sh
./cx-mpicheck --force-rescan=true
```
