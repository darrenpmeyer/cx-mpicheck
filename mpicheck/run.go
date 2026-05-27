// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const mpiAPIURL = "https://api.scs.checkmarx.com/v2/packages"

const (
	errCodeConfig  = 100
	errCodeScan    = 110
	errCodeParse   = 120
	errCodeIO      = 130
	errCodeNetwork = 140
)

// MaxAPIKeyLen caps the accepted MPIAPI key length. The key is a JWT;
// real-world JWTs run a few hundred to a couple thousand characters, so
// 2048 is a sane upper bound that catches binary blobs and accidental
// file-contents-as-env without rejecting any realistic token.
const MaxAPIKeyLen = 2048

// apiKeyPattern matches a JWT: three non-empty URL-safe base64 segments
// (A-Z, a-z, 0-9, '-', '_') joined by '.'. Padding ('=') is allowed but
// rarely present in real JWTs; we accept 0-2 trailing '=' per segment
// for robustness.
var apiKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+=*\.[A-Za-z0-9_-]+=*\.[A-Za-z0-9_-]+=*$`)

// validateAPIKey enforces length and JWT-shape constraints on the MPIAPI
// key. The key value is never echoed in the returned error; only its
// length is mentioned when that is the failure mode. Returns the
// whitespace-trimmed key for use in the Authorization header.
func validateAPIKey(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", errors.New("CHECKMARX_MPIAPI_KEY is required")
	}
	if len(trimmed) > MaxAPIKeyLen {
		return "", fmt.Errorf("CHECKMARX_MPIAPI_KEY length %d exceeds maximum of %d characters", len(trimmed), MaxAPIKeyLen)
	}
	if !apiKeyPattern.MatchString(trimmed) {
		return "", errors.New("CHECKMARX_MPIAPI_KEY is not a well-formed JWT (expected three URL-safe base64 segments joined by '.')")
	}
	return trimmed, nil
}

// Run executes the full scan + MPIAPI flow.
func Run(ctx context.Context, cfg Config, logf LogFunc) (RunResult, error) {
	result := RunResult{StartedAt: time.Now()}
	if cfg.RootDir == "" {
		return result, &Error{Code: errCodeConfig, Err: errors.New("root directory is required")}
	}
	trimmedKey, err := validateAPIKey(cfg.APIKey)
	if err != nil {
		return result, &Error{Code: errCodeConfig, Err: err}
	}
	cfg.APIKey = trimmedKey
	if cfg.BatchSize <= 0 || cfg.BatchSize > 1000 {
		return result, &Error{Code: errCodeConfig, Err: fmt.Errorf("batch size must be 1-1000")}
	}
	if cfg.MPIAPITimeout < 0 {
		return result, &Error{Code: errCodeConfig, Err: fmt.Errorf("mpiapi timeout must be non-negative (got %s)", cfg.MPIAPITimeout)}
	}
	if cfg.ResolveMode == "" {
		cfg.ResolveMode = ResolveDemand
	}
	if cfg.ResolveMode != ResolveNever && cfg.ResolveMode != ResolveDemand && cfg.ResolveMode != ResolveSmart && cfg.ResolveMode != ResolveAlways {
		return result, &Error{Code: errCodeConfig, Err: fmt.Errorf("invalid resolve mode: %s", cfg.ResolveMode)}
	}

	logf = ensureLogger(logf)
	logf("Scanning for lockfiles in %s", cfg.RootDir)

	rootAbs, err := filepath.Abs(cfg.RootDir)
	if err != nil {
		return result, &Error{Code: errCodeConfig, Err: err}
	}
	// Canonicalize root so relative-path computations against lockfile
	// paths (which discovery also canonicalizes) line up. On macOS this
	// matters even without explicit symlinks because /var is a symlink
	// to /private/var.
	rootAbs = resolveSymlinks(rootAbs)

	if cfg.ResolveMode == ResolveNever && len(cfg.FakeLockfiles) > 0 {
		logf("Resolve mode 'never' set; ignoring fake lockfile generation inputs")
		cfg.FakeLockfiles = nil
	}

	autoGenerate := cfg.ResolveMode == ResolveAlways
	if cfg.ResolveMode == ResolveSmart && !autoGenerate {
		existing, err := DiscoverLockfiles(rootAbs, cfg.ExplicitPaths, cfg.IncludeMode, cfg.ExcludePaths, Handlers())
		if err != nil {
			return result, &Error{Code: errCodeScan, Err: err}
		}
		if len(existing) == 0 {
			autoGenerate = true
		}
	}

	if autoGenerate {
		projectFiles, err := DiscoverProjectFiles(rootAbs, cfg.ExcludePaths)
		if err != nil {
			return result, &Error{Code: errCodeScan, Err: err}
		}
		if len(projectFiles) == 0 {
			logf("No project files found for fake lockfile generation")
		}
		for _, path := range projectFiles {
			logf("Auto-generating fake lockfile from %s", path)
		}
		cfg.FakeLockfiles = append(cfg.FakeLockfiles, projectFiles...)
	}

	generated, ignored, err := GenerateFakeLockfiles(ctx, cfg, logf)
	if err != nil {
		return result, &Error{Code: errCodeScan, Err: err}
	}
	if len(generated) > 0 {
		cfg.ExplicitPaths = append(cfg.ExplicitPaths, generated...)
	}
	for _, ig := range ignored {
		if ig.Path == "" {
			continue
		}
		logf("Ignoring lockfile %s in favor of fake lockfile %s", ig.Path, ig.Fake)
		cfg.ExcludePaths = append(cfg.ExcludePaths, ig.Path)
	}
	if cfg.DeleteFakeLockfiles && len(generated) > 0 {
		defer cleanupFiles(logf, generated)
	}
	lockfiles, err := DiscoverLockfiles(rootAbs, cfg.ExplicitPaths, cfg.IncludeMode, cfg.ExcludePaths, Handlers())
	if err != nil {
		return result, &Error{Code: errCodeScan, Err: err}
	}
	if cfg.ResolveMode == ResolveAlways {
		lockfiles = filterLockfilesAlways(logf, lockfiles, generated)
	}
	if len(lockfiles) == 0 {
		logf("No lockfiles found")
	}

	occurrences, err := parseLockfiles(logf, lockfiles)
	if err != nil {
		return result, &Error{Code: errCodeParse, Err: err}
	}

	result.TotalPackages = len(occurrences)
	logf("Parsed %d package entries", len(occurrences))

	packageRecords := make([]PackageRecord, 0, len(occurrences))
	for _, occ := range occurrences {
		packageRecords = append(packageRecords, PackageRecord{
			Ecosystem: occ.Ecosystem,
			Name:      occ.Name,
			Version:   occ.Version,
			Lockfile:  occ.Lockfile,
		})
	}
	if err := writeJSON(cfg.OutPackages, packageRecords); err != nil {
		return result, &Error{Code: errCodeIO, Err: err}
	}
	logf("Wrote package inventory to %s", cfg.OutPackages)

	index := indexOccurrences(occurrences, rootAbs)
	result.UniquePackages = len(index)
	logf("Built %d unique package versions", len(index))

	queries := buildQueries(index, cfg.BatchSize)
	logf("Querying MPIAPI in %d batch(es)", len(queries))

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.MPIAPITimeout}
	}
	allResults, err := queryMPIAPI(ctx, client, cfg.APIKey, queries, logf)
	if err != nil {
		return result, err
	}
	result.CheckedPackages = len(allResults)
	if err := writeJSON(cfg.OutResults, allResults); err != nil {
		return result, &Error{Code: errCodeIO, Err: err}
	}
	logf("Wrote MPIAPI results to %s", cfg.OutResults)

	riskRecords := filterRisks(allResults, index)
	result.RisksFound = len(riskRecords)
	if err := writeJSON(cfg.OutRisks, riskRecords); err != nil {
		return result, &Error{Code: errCodeIO, Err: err}
	}
	logf("Wrote risk report to %s", cfg.OutRisks)

	if cfg.PrintRisks {
		payload, _ := json.MarshalIndent(riskRecords, "", "  ")
		fmt.Fprintln(os.Stdout, string(payload))
	}

	result.FinishedAt = time.Now()
	return result, nil
}

func ensureLogger(logf LogFunc) LogFunc {
	if logf == nil {
		return func(string, ...interface{}) {}
	}
	return logf
}

func parseLockfiles(logf LogFunc, lockfiles []LockfileRef) ([]PackageOccurrence, error) {
	occurrences := []PackageOccurrence{}
	handlers := Handlers()
	handlerByKind := map[string]LockfileHandler{}
	for _, h := range handlers {
		handlerByKind[h.Kind] = h
	}
	for _, lf := range lockfiles {
		logf("Examining lockfile: %s", lf.Path)
		h, ok := handlerByKind[lf.Kind]
		if !ok {
			return nil, fmt.Errorf("missing handler for lockfile kind %s", lf.Kind)
		}
		data, err := os.ReadFile(lf.Path)
		if err != nil {
			return nil, err
		}
		pkgs, err := h.Parse(lf.Path, data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", lf.Path, err)
		}
		for _, pkg := range pkgs {
			occurrences = append(occurrences, PackageOccurrence{PackageVersion: pkg, Lockfile: lf.Path})
		}
	}
	return occurrences, nil
}

func indexOccurrences(occurrences []PackageOccurrence, rootAbs string) map[string]struct {
	Package   PackageVersion
	Lockfiles []string
} {
	index := map[string]struct {
		Package   PackageVersion
		Lockfiles []string
	}{}
	for _, occ := range occurrences {
		key := packageKey(occ.PackageVersion)
		entry := index[key]
		entry.Package = occ.PackageVersion
		lockfile := occ.Lockfile
		if rel, err := filepath.Rel(rootAbs, occ.Lockfile); err == nil {
			lockfile = rel
		}
		if !contains(entry.Lockfiles, lockfile) {
			entry.Lockfiles = append(entry.Lockfiles, lockfile)
		}
		index[key] = entry
	}
	for key, entry := range index {
		sort.Strings(entry.Lockfiles)
		index[key] = entry
	}
	return index
}

func packageKey(pkg PackageVersion) string {
	return string(pkg.Ecosystem) + ":" + pkg.Name + "@" + pkg.Version
}

func buildQueries(index map[string]struct {
	Package   PackageVersion
	Lockfiles []string
}, batchSize int) [][]PackageVersion {
	pkgs := make([]PackageVersion, 0, len(index))
	for _, entry := range index {
		pkgs = append(pkgs, entry.Package)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		if pkgs[i].Ecosystem != pkgs[j].Ecosystem {
			return pkgs[i].Ecosystem < pkgs[j].Ecosystem
		}
		if pkgs[i].Name != pkgs[j].Name {
			return pkgs[i].Name < pkgs[j].Name
		}
		return pkgs[i].Version < pkgs[j].Version
	})

	queries := [][]PackageVersion{}
	for i := 0; i < len(pkgs); i += batchSize {
		end := i + batchSize
		if end > len(pkgs) {
			end = len(pkgs)
		}
		queries = append(queries, pkgs[i:end])
	}
	return queries
}

func queryMPIAPI(ctx context.Context, client *http.Client, apiKey string, batches [][]PackageVersion, logf LogFunc) ([]map[string]interface{}, error) {
	results := []map[string]interface{}{}
	for i, batch := range batches {
		payload, err := json.Marshal(batch)
		if err != nil {
			return nil, &Error{Code: errCodeIO, Err: err}
		}
		logf("Posting batch %d of %d (%d packages)", i+1, len(batches), len(batch))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, mpiAPIURL, bytes.NewReader(payload))
		if err != nil {
			return nil, &Error{Code: errCodeNetwork, Err: err}
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", apiKey)
		resp, err := client.Do(req)
		if err != nil {
			return nil, &Error{Code: errCodeNetwork, Err: err}
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, &Error{Code: errCodeNetwork, Err: err}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &Error{Code: errCodeNetwork, Err: fmt.Errorf("MPIAPI returned %d: %s", resp.StatusCode, Redact(strings.TrimSpace(string(body)), apiKey))}
		}
		var batchResults []map[string]interface{}
		if err := json.Unmarshal(body, &batchResults); err != nil {
			return nil, &Error{Code: errCodeParse, Err: err}
		}
		results = append(results, batchResults...)
	}
	return results, nil
}

func filterRisks(results []map[string]interface{}, index map[string]struct {
	Package   PackageVersion
	Lockfiles []string
}) []RiskRecord {
	records := []RiskRecord{}
	for _, res := range results {
		rawRisks, ok := res["risks"]
		if !ok {
			continue
		}
		risks, ok := rawRisks.([]interface{})
		if !ok || len(risks) == 0 {
			continue
		}
		name, _ := res["name"].(string)
		version, _ := res["version"].(string)
		typeVal, _ := res["type"].(string)
		pkg := PackageVersion{Ecosystem: Ecosystem(typeVal), Name: name, Version: version}
		key := packageKey(pkg)
		entry, ok := index[key]
		if !ok {
			continue
		}
		purl := buildPURL(pkg)
		records = append(records, RiskRecord{
			PURL:      purl,
			Risks:     risks,
			Lockfiles: entry.Lockfiles,
		})
	}
	return records
}

func buildPURL(pkg PackageVersion) string {
	name := pkg.Name
	if pkg.Ecosystem == EcosystemPyPI {
		name = normalizePyPIName(name)
	}
	return fmt.Sprintf("pkg:%s/%s@%s", pkg.Ecosystem, name, pkg.Version)
}

func writeJSON(path string, value interface{}) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), OwnerDirMode); err != nil {
		return err
	}
	return os.WriteFile(path, payload, OwnerFileMode)
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func cleanupFiles(logf LogFunc, paths []string) {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logf("Failed to remove generated lockfile %s: %v", path, err)
		}
	}
}

func filterLockfilesAlways(logf LogFunc, lockfiles []LockfileRef, generated []string) []LockfileRef {
	if len(lockfiles) == 0 {
		return lockfiles
	}
	generatedSet := map[string]struct{}{}
	for _, path := range generated {
		if path == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err == nil {
			generatedSet[abs] = struct{}{}
		} else {
			generatedSet[path] = struct{}{}
		}
	}
	filtered := make([]LockfileRef, 0, len(lockfiles))
	for _, lf := range lockfiles {
		if lf.Ecosystem == EcosystemGo {
			filtered = append(filtered, lf)
			continue
		}
		if _, ok := generatedSet[lf.Path]; ok {
			filtered = append(filtered, lf)
			continue
		}
		logf("Resolve mode 'always' ignoring lockfile %s", lf.Path)
	}
	return filtered
}
