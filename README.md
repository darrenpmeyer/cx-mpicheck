# cx-mpicheck

`cx-mpicheck` is both:

- A Go library (`mpicheck`) for discovering lockfiles, building a package inventory, and querying Checkmarx MPIAPI for risks.
- A CI-focused command-line tool (`cx-mpicheck`) that uses the library to scan a repository and fail builds when risks are found.

High-level overview

- Library: scan a directory, parse supported lockfiles (npm, pnpm, pip requirements, Pipfile.lock, poetry.lock, go.mod), optionally generate fake lockfiles from project definitions (npm, pnpm, pip, pipenv, poetry, go), write inventories, call MPIAPI in batches, and produce risk reports.
- CLI: wraps the library with GNU-style flags and environment variable configuration, emits STDERR progress logs, writes JSON outputs, applies risk policies, and returns CI-friendly exit codes (when a policy is set, the risk report is backed up and rewritten to only include matching risks).
- Resolve modes: control when fake lockfiles are generated (`never`, `demand`, `smart`, `always`).

Deeper docs

- Library usage: [docs/library.md](docs/library.md)
- CLI & CI usage: [docs/cli.md](docs/cli.md)

Quick start (CLI)

```sh
export CHECKMARX_MPIAPI_KEY=YOUR_KEY
./cx-mpicheck --root . --verbose
```

Version

```sh
./cx-mpicheck --version
```

License

This project is licensed under the GNU AGPLv3. Using this tool (even with modifications) as part of your application’s build process does not change or impose the AGPL on your application’s own licensing terms.

Quick start (library)

```go
cfg := mpicheck.DefaultConfig()
cfg.RootDir = "."
cfg.APIKey = os.Getenv("CHECKMARX_MPIAPI_KEY")

result, err := mpicheck.Run(context.Background(), cfg, func(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
})
if err != nil {
	// handle error
}
fmt.Println("Risks:", result.RisksFound)
```
