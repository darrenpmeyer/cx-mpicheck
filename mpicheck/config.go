// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import "net/http"

// IncludeMode controls how explicit lockfile paths are used.
type IncludeMode string

const (
	IncludeOnly IncludeMode = "only" // Use only explicit lockfile paths.
	IncludeAlso IncludeMode = "also" // Use explicit lockfiles plus discovery.
)

// Config controls the behavior of a run.
type Config struct {
	RootDir             string
	ExplicitPaths       []string
	IncludeMode         IncludeMode
	ExcludePaths        []string
	FakeLockfiles       []string
	FakeLockfileOut     string
	DeleteFakeLockfiles bool
	ResolveMode         ResolveMode
	OutPackages         string
	OutResults          string
	OutRisks            string
	PrintRisks          bool
	RiskExitCode        int
	BatchSize           int
	APIKey              string
	HTTPClient          *http.Client
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		RootDir:      ".",
		IncludeMode:  IncludeAlso,
		OutPackages:  "cx.packages-master.json",
		OutResults:   "cx.mpiapi-results.json",
		OutRisks:     "cx.mpiapi-risks.json",
		RiskExitCode: 22,
		BatchSize:    1000,
		ResolveMode:  ResolveDemand,
	}
}

// ResolveMode controls fake lockfile generation behavior.
type ResolveMode string

const (
	ResolveNever  ResolveMode = "never"
	ResolveDemand ResolveMode = "demand"
	ResolveSmart  ResolveMode = "smart"
	ResolveAlways ResolveMode = "always"
)
