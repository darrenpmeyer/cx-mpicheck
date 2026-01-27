# Library usage

The `mpicheck` package provides all core functionality for lockfile discovery, parsing, MPIAPI querying, and risk filtering.

Key types

- `mpicheck.Config`: Controls scanning, outputs, MPIAPI settings, and batching.
- `mpicheck.Run(ctx, cfg, logf)`: Executes the full flow.
- `mpicheck.RunResult`: Summarizes the run (counts and timings).
- `mpicheck.Error`: Typed error with a CI-friendly exit code.
- `mpicheck.RiskPolicy`: Controls which risks trigger a non-zero exit.
- `mpicheck.BuildRiskPolicy(scoreExprs, titleMatchers)`: Parses score rules and title matchers.
- `mpicheck.EvaluatePolicy(records, policy)`: Counts total risks and those that match policy.
- `mpicheck.LoadRiskFile(path)`: Loads `cx.mpiapi-risks.json` for offline policy checks.

Installation

```sh
go get github.com/darrenpmeyer/cx-mpicheck@latest
```

Then import the library package:

```go
import "github.com/darrenpmeyer/cx-mpicheck/mpicheck"
```

Minimal example

```go
package main

import (
	"context"
	"fmt"
	"os"

	"cx-mpicheck/mpicheck"
)

func main() {
	cfg := mpicheck.DefaultConfig()
	cfg.RootDir = "."
	cfg.APIKey = os.Getenv("CHECKMARX_MPIAPI_KEY")

	logf := func(format string, args ...interface{}) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}

	result, err := mpicheck.Run(context.Background(), cfg, logf)
	if err != nil {
		if typed, ok := err.(*mpicheck.Error); ok {
			fmt.Fprintf(os.Stderr, "error (code %d): %v\n", typed.Code, typed)
			os.Exit(typed.Code)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Risks found: %d\n", result.RisksFound)
}
```

Config fields

- `RootDir`: Root directory to scan. Defaults to `.`.
- `ExplicitPaths`: Specific lockfile paths to include.
- `IncludeMode`: `IncludeOnly` (only explicit paths) or `IncludeAlso` (explicit + discovery).
- `ExcludePaths`: Paths to exclude (file or directory).
- `FakeLockfiles`: Project definition files to generate temporary lockfiles from (e.g., `package.json`). You can also use `manager:path` (e.g., `pnpm:package.json`).
- `FakeLockfileOut`: Output directory for generated fake lockfiles.
- `DeleteFakeLockfiles`: Delete generated fake lockfiles on exit.
- `ResolveMode`: `never`, `demand`, `smart`, or `always`.
- `OutPackages`: JSON output for package inventory (`cx.packages-master.json`).
- `OutResults`: JSON output for aggregated MPIAPI results (`cx.mpiapi-results.json`).
- `OutRisks`: JSON output for filtered risk report (`cx.mpiapi-risks.json`).
- `PrintRisks`: When true, prints the risk report to STDOUT in JSON.
- `RiskExitCode`: Exit code to use when risks are found (CLI uses this).
- `BatchSize`: MPIAPI batch size (1-1000).
- `APIKey`: Checkmarx MPIAPI key (from `CHECKMARX_MPIAPI_KEY`).
- `HTTPClient`: Optional custom HTTP client (timeouts, proxies, etc.).

Risk policy

`RiskPolicy` lets you decide which risks should fail CI. If no score rules or title matchers are provided, the policy matches any risk (default behavior).

Score rules accept:

- Comparisons: `>5`, `>=5`, `<7`, `<=7`, `==10`
- Ranges: `5-9` or `5..9`
- Exact: `10`

Title matchers are case-insensitive substrings that match against risk titles/names when present.

When a policy is specified in the CLI, the risk report file is backed up and rewritten to include only the matching risks.

The CLI summary log reports matched packages, total risks, packages with risks, and packages checked.

Lockfiles are logged as they are examined.

The CLI also prints a banner line at startup and reports when the MPIAPI key is missing with an example command.

Fake lockfile managers

Supported managers: `npm`, `pnpm`, `pip`, `pipenv`, `poetry`, `go`.

For `pip`, supported project definition files include `requirements.in`, `requirements.txt`, `setup.cfg`, `setup.ini`, and `pyproject.toml` (only when dependencies are present).

When a fake lockfile is generated, the tool excludes any real lockfile for the same manager in the same directory.

Outputs

- Package inventory: list of records `{type,name,version,lockfile}` (types include `npm`, `pypi`, and `go`).
- MPIAPI results: aggregated list of dictionaries returned by the API.
- Risk report: list of `{purl, risks, lockfiles}` for packages with non-empty risks. Lockfile paths are reported relative to the scanned root.

Adding new ecosystems

Add a new `LockfileHandler` in `mpicheck.Handlers()` and a parser function that returns a slice of `PackageVersion`. The rest of the pipeline (inventory, MPIAPI batching, risk filtering) works without changes.
