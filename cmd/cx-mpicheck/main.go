// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"cx-mpicheck/mpicheck"
)

type stringFlag struct {
	value string
	set   bool
}

func (f *stringFlag) String() string { return f.value }
func (f *stringFlag) Set(val string) error {
	f.value = val
	f.set = true
	return nil
}

type intFlag struct {
	value int
	set   bool
}

func (f *intFlag) String() string { return fmt.Sprintf("%d", f.value) }
func (f *intFlag) Set(val string) error {
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return err
	}
	f.value = parsed
	f.set = true
	return nil
}

type boolFlag struct {
	value  bool
	set    bool
	invert bool
}

func (f *boolFlag) String() string { return fmt.Sprintf("%t", f.value) }
func (f *boolFlag) Set(val string) error {
	parsed, err := parseBool(val)
	if err != nil {
		return err
	}
	if f.invert {
		f.value = !parsed
	} else {
		f.value = parsed
	}
	f.set = true
	return nil
}
func (f *boolFlag) IsBoolFlag() bool { return true }

type stringSliceFlag struct {
	values []string
	set    bool
}

func (f *stringSliceFlag) String() string { return strings.Join(f.values, ",") }
func (f *stringSliceFlag) Set(val string) error {
	for _, item := range splitList(val) {
		if item != "" {
			f.values = append(f.values, item)
		}
	}
	f.set = true
	return nil
}

func main() {
	cfg := mpicheck.DefaultConfig()
	logger := func(format string, args ...interface{}) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}

	// Banner printed after pretty/non-pretty header.

	rootEnv := os.Getenv("CX_MPICHECK_ROOT")
	includeEnv := os.Getenv("CX_MPICHECK_INCLUDE")
	includeModeEnv := os.Getenv("CX_MPICHECK_INCLUDE_MODE")
	excludeEnv := os.Getenv("CX_MPICHECK_EXCLUDE")
	outPackagesEnv := os.Getenv("CX_MPICHECK_OUT_PACKAGES")
	outResultsEnv := os.Getenv("CX_MPICHECK_OUT_RESULTS")
	outRisksEnv := os.Getenv("CX_MPICHECK_OUT_RISKS")
	printRisksEnv := os.Getenv("CX_MPICHECK_PRINT_RISKS")
	riskExitEnv := os.Getenv("CX_MPICHECK_RISK_EXIT_CODE")
	riskScoreEnv := os.Getenv("CX_MPICHECK_RISK_SCORE")
	riskTitleEnv := os.Getenv("CX_MPICHECK_RISK_TITLE")
	forceRescanEnv := os.Getenv("CX_MPICHECK_FORCE_RESCAN")
	fakeLockfileEnv := os.Getenv("CX_MPICHECK_FAKE_LOCKFILE")
	fakeLockfileOutEnv := os.Getenv("CX_MPICHECK_FAKE_LOCKFILE_OUT")
	fakeLockfileCleanupEnv := os.Getenv("CX_MPICHECK_FAKE_LOCKFILE_CLEANUP")
	resolveEnv := os.Getenv("CX_MPICHECK_RESOLVE")
	batchSizeEnv := os.Getenv("CX_MPICHECK_BATCH_SIZE")
	verboseEnv := os.Getenv("CX_MPICHECK_VERBOSE")

	if rootEnv != "" {
		cfg.RootDir = rootEnv
	}
	if includeModeEnv != "" {
		cfg.IncludeMode = mpicheck.IncludeMode(strings.ToLower(includeModeEnv))
	}
	if includeEnv != "" {
		cfg.ExplicitPaths = splitList(includeEnv)
	}
	if excludeEnv != "" {
		cfg.ExcludePaths = splitList(excludeEnv)
	}
	if outPackagesEnv != "" {
		cfg.OutPackages = outPackagesEnv
	}
	if outResultsEnv != "" {
		cfg.OutResults = outResultsEnv
	}
	if outRisksEnv != "" {
		cfg.OutRisks = outRisksEnv
	}
	if fakeLockfileEnv != "" {
		cfg.FakeLockfiles = splitList(fakeLockfileEnv)
	}
	if fakeLockfileOutEnv != "" {
		cfg.FakeLockfileOut = fakeLockfileOutEnv
	}
	if fakeLockfileCleanupEnv != "" {
		if v, err := parseBool(fakeLockfileCleanupEnv); err == nil {
			cfg.DeleteFakeLockfiles = v
		}
	}
	if resolveEnv != "" {
		cfg.ResolveMode = mpicheck.ResolveMode(strings.ToLower(resolveEnv))
	}
	if printRisksEnv != "" {
		if v, err := parseBool(printRisksEnv); err == nil {
			cfg.PrintRisks = v
		}
	}
	if riskExitEnv != "" {
		if v, err := strconv.Atoi(riskExitEnv); err == nil {
			cfg.RiskExitCode = v
		}
	}
	riskScoreExprs := []string{}
	if riskScoreEnv != "" {
		riskScoreExprs = splitList(riskScoreEnv)
	}
	riskTitleMatchers := []string{}
	if riskTitleEnv != "" {
		riskTitleMatchers = splitList(riskTitleEnv)
	}
	forceRescan := false
	if forceRescanEnv != "" {
		if v, err := parseBool(forceRescanEnv); err == nil {
			forceRescan = v
		}
	}
	if batchSizeEnv != "" {
		if v, err := strconv.Atoi(batchSizeEnv); err == nil {
			cfg.BatchSize = v
		}
	}
	verbose := false
	if verboseEnv != "" {
		if v, err := parseBool(verboseEnv); err == nil {
			verbose = v
		}
	}

	rootFlag := &stringFlag{value: cfg.RootDir}
	includeFlag := &stringSliceFlag{values: cfg.ExplicitPaths}
	excludeFlag := &stringSliceFlag{values: cfg.ExcludePaths}
	includeModeFlag := &stringFlag{value: string(cfg.IncludeMode)}
	outPackagesFlag := &stringFlag{value: cfg.OutPackages}
	outResultsFlag := &stringFlag{value: cfg.OutResults}
	outRisksFlag := &stringFlag{value: cfg.OutRisks}
	printRisksFlag := &boolFlag{value: cfg.PrintRisks}
	riskExitFlag := &intFlag{value: cfg.RiskExitCode}
	riskScoreFlag := &stringSliceFlag{values: riskScoreExprs}
	riskTitleFlag := &stringSliceFlag{values: riskTitleMatchers}
	forceRescanFlag := &boolFlag{value: forceRescan}
	fakeLockfileFlag := &stringSliceFlag{values: cfg.FakeLockfiles}
	fakeLockfileOutFlag := &stringFlag{value: cfg.FakeLockfileOut}
	fakeLockfileCleanupFlag := &boolFlag{value: cfg.DeleteFakeLockfiles}
	resolveFlag := &stringFlag{value: string(cfg.ResolveMode)}
	batchSizeFlag := &intFlag{value: cfg.BatchSize}
	verboseFlag := &boolFlag{value: verbose}
	versionFlag := &boolFlag{value: false}
	prettyFlag := &boolFlag{value: false}

	flag.Var(rootFlag, "root", "Root directory to scan for lockfiles")
	flag.Var(includeFlag, "lockfile", "Explicit lockfile path (repeatable or comma-separated)")
	flag.Var(includeModeFlag, "include-mode", "How to combine explicit lockfiles with discovery: only|also")
	flag.Var(excludeFlag, "exclude", "Lockfile or directory to exclude (repeatable or comma-separated)")
	flag.Var(outPackagesFlag, "out-packages", "Output path for package inventory JSON")
	flag.Var(outResultsFlag, "out-results", "Output path for MPIAPI results JSON")
	flag.Var(outRisksFlag, "out-risks", "Output path for MPIAPI risks JSON")
	printRisksName := registerBoolFlag(printRisksFlag, "print-risks", cfg.PrintRisks, "Print risk output to STDOUT")
	flag.Var(riskExitFlag, "risk-exit-code", "Exit code to use when risks are found")
	flag.Var(riskScoreFlag, "risk-score", "Risk score rule (repeatable or comma-separated, e.g. \">5\", \"5-9\", \"10\")")
	flag.Var(riskTitleFlag, "risk-title", "Risk title matcher (case-insensitive substring, repeatable or comma-separated)")
	forceRescanName := registerBoolFlag(forceRescanFlag, "force-rescan", forceRescan, "Force lockfile rescan even if risk output exists")
	flag.Var(fakeLockfileFlag, "fake-lockfile", "Project definition file to generate a lockfile from (repeatable or comma-separated)")
	flag.Var(fakeLockfileOutFlag, "fake-lockfile-out", "Output directory for generated fake lockfiles")
	fakeCleanupName := registerBoolFlag(fakeLockfileCleanupFlag, "fake-lockfile-cleanup", cfg.DeleteFakeLockfiles, "Delete generated fake lockfiles on exit")
	flag.Var(resolveFlag, "resolve", "Resolve mode for fake lockfiles: never|demand|smart|always")
	flag.Var(batchSizeFlag, "batch-size", "Number of packages per MPIAPI request (1-1000)")
	verboseName := registerBoolFlag(verboseFlag, "verbose", verbose, "Enable verbose logging")
	registerBoolFlag(versionFlag, "version", false, "Print version and exit")
	registerBoolFlag(prettyFlag, "pretty", false, "Use colored, emoji-friendly output")

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Usage: %s [options]\n\nOptions:\n", os.Args[0])
		flag.VisitAll(func(f *flag.Flag) {
			defaultVal := f.DefValue
			if defaultVal != "" {
				fmt.Fprintf(out, "  --%s=%s\n      %s (default: %s)\n", f.Name, f.DefValue, f.Usage, f.DefValue)
				return
			}
			fmt.Fprintf(out, "  --%s\n      %s\n", f.Name, f.Usage)
		})
	}

	flag.Parse()

	if versionFlag.set && versionFlag.value {
		fmt.Fprintln(os.Stdout, version)
		return
	}

	if rootFlag.set {
		noteOverride(logger, "CX_MPICHECK_ROOT", "--root")
		cfg.RootDir = rootFlag.value
	}
	if includeFlag.set {
		noteOverride(logger, "CX_MPICHECK_INCLUDE", "--lockfile")
		cfg.ExplicitPaths = includeFlag.values
	}
	if includeModeFlag.set {
		noteOverride(logger, "CX_MPICHECK_INCLUDE_MODE", "--include-mode")
		cfg.IncludeMode = mpicheck.IncludeMode(strings.ToLower(includeModeFlag.value))
	}
	if excludeFlag.set {
		noteOverride(logger, "CX_MPICHECK_EXCLUDE", "--exclude")
		cfg.ExcludePaths = excludeFlag.values
	}
	if outPackagesFlag.set {
		noteOverride(logger, "CX_MPICHECK_OUT_PACKAGES", "--out-packages")
		cfg.OutPackages = outPackagesFlag.value
	}
	if outResultsFlag.set {
		noteOverride(logger, "CX_MPICHECK_OUT_RESULTS", "--out-results")
		cfg.OutResults = outResultsFlag.value
	}
	if outRisksFlag.set {
		noteOverride(logger, "CX_MPICHECK_OUT_RISKS", "--out-risks")
		cfg.OutRisks = outRisksFlag.value
	}
	if printRisksFlag.set {
		noteOverride(logger, "CX_MPICHECK_PRINT_RISKS", "--"+printRisksName)
		cfg.PrintRisks = printRisksFlag.value
	}
	if riskExitFlag.set {
		noteOverride(logger, "CX_MPICHECK_RISK_EXIT_CODE", "--risk-exit-code")
		cfg.RiskExitCode = riskExitFlag.value
	}
	if riskScoreFlag.set {
		noteOverride(logger, "CX_MPICHECK_RISK_SCORE", "--risk-score")
		riskScoreExprs = riskScoreFlag.values
	}
	if riskTitleFlag.set {
		noteOverride(logger, "CX_MPICHECK_RISK_TITLE", "--risk-title")
		riskTitleMatchers = riskTitleFlag.values
	}
	if forceRescanFlag.set {
		noteOverride(logger, "CX_MPICHECK_FORCE_RESCAN", "--"+forceRescanName)
		forceRescan = forceRescanFlag.value
	}
	if fakeLockfileFlag.set {
		noteOverride(logger, "CX_MPICHECK_FAKE_LOCKFILE", "--fake-lockfile")
		cfg.FakeLockfiles = fakeLockfileFlag.values
	}
	if fakeLockfileOutFlag.set {
		noteOverride(logger, "CX_MPICHECK_FAKE_LOCKFILE_OUT", "--fake-lockfile-out")
		cfg.FakeLockfileOut = fakeLockfileOutFlag.value
	}
	if fakeLockfileCleanupFlag.set {
		noteOverride(logger, "CX_MPICHECK_FAKE_LOCKFILE_CLEANUP", "--"+fakeCleanupName)
		cfg.DeleteFakeLockfiles = fakeLockfileCleanupFlag.value
	}
	if resolveFlag.set {
		noteOverride(logger, "CX_MPICHECK_RESOLVE", "--resolve")
		cfg.ResolveMode = mpicheck.ResolveMode(strings.ToLower(resolveFlag.value))
	}
	if batchSizeFlag.set {
		noteOverride(logger, "CX_MPICHECK_BATCH_SIZE", "--batch-size")
		cfg.BatchSize = batchSizeFlag.value
	}
	if verboseFlag.set {
		noteOverride(logger, "CX_MPICHECK_VERBOSE", "--"+verboseName)
		verbose = verboseFlag.value
	}

	if cfg.IncludeMode != mpicheck.IncludeOnly && cfg.IncludeMode != mpicheck.IncludeAlso {
		logger("Invalid include-mode: %s (expected only|also)", cfg.IncludeMode)
		os.Exit(100)
	}
	if cfg.IncludeMode == mpicheck.IncludeOnly && len(cfg.ExplicitPaths) == 0 && len(cfg.FakeLockfiles) == 0 {
		logger("include-mode is 'only' but no --lockfile or --fake-lockfile paths were provided")
		os.Exit(100)
	}
	if cfg.ResolveMode != "" && cfg.ResolveMode != mpicheck.ResolveNever && cfg.ResolveMode != mpicheck.ResolveDemand && cfg.ResolveMode != mpicheck.ResolveSmart && cfg.ResolveMode != mpicheck.ResolveAlways {
		logger("Invalid resolve mode: %s (expected never|demand|smart|always)", cfg.ResolveMode)
		os.Exit(100)
	}

	policy, err := mpicheck.BuildRiskPolicy(riskScoreExprs, riskTitleMatchers)
	if err != nil {
		logger("Invalid risk policy: %v", err)
		os.Exit(100)
	}

	apiKey := os.Getenv("CHECKMARX_MPIAPI_KEY")
	cfg.APIKey = apiKey
	if cfg.APIKey == "" {
		logger("CHECKMARX_MPIAPI_KEY is required. For example:\n  env CHECKMARX_MPIAPI_KEY=<your key> %s [options]", os.Args[0])
		os.Exit(100)
	}

	logf := logger
	if !verbose {
		logf = func(format string, args ...interface{}) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		}
	}
	prettyEnabled := prettyFlag.set && prettyFlag.value
	if prettyEnabled {
		logger("Malicious package checker for %s MPIAPI", color("Checkmarx", "magenta"))
	} else {
		logger("Malicious package checker for Checkmarx MPIAPI")
	}
	logger("(c) 2026 Darren P Meyer, sponsored by Checkmarx, released under AGPL 3.0")
	if prettyEnabled {
		logf = prettyLogger(logf)
		logger = prettyLogger(logger)
	}
	if verbose {
		extras := map[string]string{}
		if len(riskScoreExprs) > 0 {
			extras["risk-score"] = strings.Join(riskScoreExprs, ",")
		}
		if len(riskTitleMatchers) > 0 {
			extras["risk-title"] = strings.Join(riskTitleMatchers, ",")
		}
		if forceRescan {
			extras["force-rescan"] = "true"
		}
		extras["verbose"] = "true"
		if prettyFlag.set && prettyFlag.value {
			extras["pretty"] = "true"
		}
		printConfigDiffs(logf, cfg, extras)
	}

	ctx := context.Background()
	if fileExists(cfg.OutRisks) && !forceRescan {
		logger("Using existing risk report: %s", cfg.OutRisks)
		records, err := mpicheck.LoadRiskFile(cfg.OutRisks)
		if err != nil {
			logger("Error reading risk report: %v", err)
			os.Exit(130)
		}
		records = applyPolicyAndPersist(logger, cfg.OutRisks, records, policy)
		if cfg.PrintRisks {
			if err := printJSONFile(cfg.OutRisks); err != nil {
				logger("Error printing risk report: %v", err)
				os.Exit(130)
			}
		}
		exitWithPolicy(logger, cfg.RiskExitCode, records, policy, len(records), prettyEnabled)
		return
	}

	result, err := mpicheck.Run(ctx, cfg, logf)
	if err != nil {
		code := 100
		if typed, ok := err.(*mpicheck.Error); ok {
			code = typed.Code
		}
		logger("Error: %v", err)
		os.Exit(code)
	}
	logger("Risk report contains %d package(s) with risks", result.RisksFound)

	records, err := mpicheck.LoadRiskFile(cfg.OutRisks)
	if err != nil {
		logger("Error reading risk report: %v", err)
		os.Exit(130)
	}
	records = applyPolicyAndPersist(logger, cfg.OutRisks, records, policy)
	exitWithPolicy(logger, cfg.RiskExitCode, records, policy, result.CheckedPackages, prettyEnabled)
}

func noteOverride(logf func(string, ...interface{}), env, flag string) {
	if os.Getenv(env) == "" {
		return
	}
	logf("Environment variable %s overridden by %s", env, flag)
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true, nil
	case "0", "false", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean: %s", value)
	}
}

func splitList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';'
	})
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			trimmed = append(trimmed, part)
		}
	}
	return trimmed
}

func registerBoolFlag(target *boolFlag, name string, defaultVal bool, usage string) string {
	if defaultVal {
		target.invert = true
		flag.Var(target, "no-"+name, usage)
		return "no-" + name
	}
	target.invert = false
	flag.Var(target, name, usage)
	return name
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func printJSONFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, string(data))
	return nil
}

func exitWithPolicy(logger func(string, ...interface{}), riskExitCode int, records []mpicheck.RiskRecord, policy mpicheck.RiskPolicy, checkedPackages int, prettyEnabled bool) {
	_, totalRisks, _, matchedPackages := mpicheck.FilterRiskRecords(records, policy)
	logPolicySummary(logger, matchedPackages, totalRisks, len(records), checkedPackages, prettyEnabled)
	if matchedPackages > 0 {
		os.Exit(riskExitCode)
	}
	logger("No risks matched policy")
	os.Exit(0)
}

func printConfigDiffs(logf func(string, ...interface{}), cfg mpicheck.Config, extras map[string]string) {
	defaults := mpicheck.DefaultConfig()
	type diff struct {
		name  string
		value string
	}
	diffs := []diff{}
	if cfg.RootDir != defaults.RootDir {
		diffs = append(diffs, diff{name: "root", value: cfg.RootDir})
	}
	if !stringSlicesEqual(cfg.ExplicitPaths, defaults.ExplicitPaths) {
		diffs = append(diffs, diff{name: "lockfile", value: strings.Join(cfg.ExplicitPaths, ",")})
	}
	if cfg.IncludeMode != defaults.IncludeMode {
		diffs = append(diffs, diff{name: "include-mode", value: string(cfg.IncludeMode)})
	}
	if !stringSlicesEqual(cfg.ExcludePaths, defaults.ExcludePaths) {
		diffs = append(diffs, diff{name: "exclude", value: strings.Join(cfg.ExcludePaths, ",")})
	}
	if !stringSlicesEqual(cfg.FakeLockfiles, defaults.FakeLockfiles) {
		diffs = append(diffs, diff{name: "fake-lockfile", value: strings.Join(cfg.FakeLockfiles, ",")})
	}
	if cfg.FakeLockfileOut != defaults.FakeLockfileOut {
		diffs = append(diffs, diff{name: "fake-lockfile-out", value: cfg.FakeLockfileOut})
	}
	if cfg.DeleteFakeLockfiles != defaults.DeleteFakeLockfiles {
		diffs = append(diffs, diff{name: "fake-lockfile-cleanup", value: fmt.Sprintf("%t", cfg.DeleteFakeLockfiles)})
	}
	if cfg.ResolveMode != defaults.ResolveMode {
		diffs = append(diffs, diff{name: "resolve", value: string(cfg.ResolveMode)})
	}
	if cfg.OutPackages != defaults.OutPackages {
		diffs = append(diffs, diff{name: "out-packages", value: cfg.OutPackages})
	}
	if cfg.OutResults != defaults.OutResults {
		diffs = append(diffs, diff{name: "out-results", value: cfg.OutResults})
	}
	if cfg.OutRisks != defaults.OutRisks {
		diffs = append(diffs, diff{name: "out-risks", value: cfg.OutRisks})
	}
	if cfg.PrintRisks != defaults.PrintRisks {
		diffs = append(diffs, diff{name: "print-risks", value: fmt.Sprintf("%t", cfg.PrintRisks)})
	}
	if cfg.RiskExitCode != defaults.RiskExitCode {
		diffs = append(diffs, diff{name: "risk-exit-code", value: fmt.Sprintf("%d", cfg.RiskExitCode)})
	}
	if cfg.BatchSize != defaults.BatchSize {
		diffs = append(diffs, diff{name: "batch-size", value: fmt.Sprintf("%d", cfg.BatchSize)})
	}
	for name, value := range extras {
		if value == "" {
			continue
		}
		diffs = append(diffs, diff{name: name, value: value})
	}
	logf("Non-default configuration values:")
	if len(diffs) == 0 {
		logf("  (none)")
		return
	}
	for _, item := range diffs {
		logf("  %s=%s", item.name, item.value)
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func logPolicySummary(logger func(string, ...interface{}), matchedPackages int, totalRisks int, packagesWithRisks int, checkedPackages int, pretty bool) {
	if !pretty {
		logger("Packages matched policy: %d (total risks: %d, packages with risks: %d, packages checked: %d)", matchedPackages, totalRisks, packagesWithRisks, checkedPackages)
		return
	}
	msg := fmt.Sprintf("Packages matched policy: %d (total risks: %d, packages with risks: %d, packages checked: %d)", matchedPackages, totalRisks, packagesWithRisks, checkedPackages)
	if matchedPackages == 0 {
		logger("%s", color("👍 "+msg, "green"))
		return
	}
	logger("%s", color("⚠️  "+msg, "red"))
}

func prettyLogger(base func(string, ...interface{})) func(string, ...interface{}) {
	return func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		base("%s", decorateMessage(msg))
	}
}

func decorateMessage(msg string) string {
	switch {
	case strings.HasPrefix(msg, "Querying MPIAPI"):
		return color("🔎 "+msg, "cyan")
	case strings.HasPrefix(msg, "Posting batch"):
		return color("📦 "+msg, "cyan")
	case strings.HasPrefix(msg, "Running external tool"):
		return color("🛠️  "+msg, "yellow")
	case strings.HasPrefix(msg, "External tool still running"):
		return color("⏳ "+msg, "yellow")
	case strings.HasPrefix(msg, "External tool completed"):
		return color("✅ "+msg, "green")
	case strings.HasPrefix(msg, "Generating"):
		return color("✨ "+msg, "yellow")
	case strings.HasPrefix(msg, "Auto-generating"):
		return color("✨ "+msg, "yellow")
	case strings.HasPrefix(msg, "Ignoring lockfile"):
		return color("🚫 "+msg, "yellow")
	case strings.HasPrefix(msg, "Using existing risk report"):
		return color("📄 "+msg, "cyan")
	case strings.HasPrefix(msg, "Risks found"):
		return color("⚠️  "+msg, "red")
	case strings.HasPrefix(msg, "No risks found"):
		return color("✅ "+msg, "green")
	case strings.HasPrefix(msg, "Packages matched policy"):
		return color("⚠️  "+msg, "red")
	case strings.HasPrefix(msg, "No risks matched policy"):
		return color("✅ "+msg, "green")
	case strings.HasPrefix(msg, "Error:"):
		return color("❌ "+msg, "red")
	case strings.HasPrefix(msg, "Non-default configuration values:"):
		return color("⚙️  "+msg, "magenta")
	default:
		return msg
	}
}

func color(msg string, name string) string {
	code := ""
	switch name {
	case "red":
		code = "31"
	case "green":
		code = "32"
	case "yellow":
		code = "33"
	case "cyan":
		code = "36"
	case "magenta":
		code = "35"
	default:
		return msg
	}
	return fmt.Sprintf("\x1b[%sm%s\x1b[0m", code, msg)
}

func applyPolicyAndPersist(logger func(string, ...interface{}), path string, records []mpicheck.RiskRecord, policy mpicheck.RiskPolicy) []mpicheck.RiskRecord {
	filtered, _, _, _ := mpicheck.FilterRiskRecords(records, policy)
	if policy.IsDefault() {
		return filtered
	}
	if err := backupFile(path); err != nil {
		logger("Error backing up risk report: %v", err)
		os.Exit(130)
	}
	if err := writeJSONFile(path, filtered); err != nil {
		logger("Error writing filtered risk report: %v", err)
		os.Exit(130)
	}
	return filtered
}

func backupFile(path string) error {
	backup := path + ".bak"
	if fileExists(backup) {
		backup = path + "." + strconv.FormatInt(time.Now().Unix(), 10) + ".bak"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(backup, data, 0o644)
}

func writeJSONFile(path string, value interface{}) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}
