// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import "time"

// Ecosystem identifies the package ecosystem.
type Ecosystem string

const (
	EcosystemNPM  Ecosystem = "npm"
	EcosystemPyPI Ecosystem = "pypi"
	EcosystemGo   Ecosystem = "go"
)

// PackageVersion identifies a package by ecosystem, name, and exact version.
type PackageVersion struct {
	Ecosystem Ecosystem `json:"type"`
	Name      string    `json:"name"`
	Version   string    `json:"version"`
}

// PackageOccurrence records where a package version was found.
type PackageOccurrence struct {
	PackageVersion
	Lockfile string `json:"lockfile"`
}

// PackageRecord is written to cx.packages-master.json.
type PackageRecord struct {
	Ecosystem Ecosystem `json:"type"`
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Lockfile  string    `json:"lockfile"`
}

// RiskRecord is written to cx.mpiapi-risks.json.
type RiskRecord struct {
	PURL      string        `json:"purl"`
	Risks     []interface{} `json:"risks"`
	Lockfiles []string      `json:"lockfiles"`
}

// RunResult summarizes the run outcome.
type RunResult struct {
	TotalPackages   int
	UniquePackages  int
	RisksFound      int
	CheckedPackages int
	StartedAt       time.Time
	FinishedAt      time.Time
}

// LogFunc is an optional logger for progress messages.
type LogFunc func(format string, args ...interface{})
