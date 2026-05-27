// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	npmPackageJSON     = "package.json"
	pnpmWorkspaceYAML  = "pnpm-workspace.yaml"
	pnpmWorkspaceYML   = "pnpm-workspace.yml"
	pipfileName        = "Pipfile"
	poetryProjectName  = "pyproject.toml"
	setupCfgName       = "setup.cfg"
	setupIniName       = "setup.ini"
	requirementsInName = "requirements.in"
	requirementsTxt    = "requirements.txt"
	goModName          = "go.mod"
)

// GenerateFakeLockfiles creates temporary lockfiles from project definition files.
func GenerateFakeLockfiles(ctx context.Context, cfg Config, logf LogFunc) ([]string, []fakeIgnore, error) {
	if len(cfg.FakeLockfiles) == 0 {
		return nil, nil, nil
	}
	logf = ensureLogger(logf)

	rootAbs, err := filepath.Abs(cfg.RootDir)
	if err != nil {
		return nil, nil, err
	}
	specs, err := parseFakeSpecs(rootAbs, cfg.FakeLockfiles)
	if err != nil {
		return nil, nil, err
	}

	generated := []string{}
	ignored := []fakeIgnore{}
	for _, spec := range specs {
		if spec.Path == "" {
			continue
		}
		if err := ensureFileExists(spec.Path); err != nil {
			return nil, nil, err
		}

		outPath := filepath.Join(fakeLockDir(rootAbs, cfg.FakeLockfileOut, spec.Path), defaultFakeName(spec.Manager))

		var lockfile string
		var err error
		switch spec.Manager {
		case "npm":
			lockfile, err = generateNpmLockfile(ctx, spec.Path, outPath, logf)
		case "pnpm":
			lockfile, err = generatePnpmLockfile(ctx, spec.Path, outPath, logf)
		case "pip":
			lockfile, err = generatePipLockfile(ctx, spec.Path, outPath, logf)
			if err == nil && lockfile == "" {
				continue
			}
		case "pipenv":
			lockfile, err = generatePipenvLockfile(ctx, spec.Path, outPath, logf)
		case "poetry":
			lockfile, err = generatePoetryLockfile(ctx, spec.Path, outPath, logf)
		case "go":
			lockfile, err = generateGoLockfile(ctx, spec.Path, outPath, logf)
		default:
			return nil, nil, fmt.Errorf("unsupported fake lockfile manager: %s", spec.Manager)
		}
		if err != nil {
			return nil, nil, err
		}
		generated = append(generated, lockfile)
		for _, ignorePath := range lockfilesToIgnore(spec.Manager, spec.Path) {
			ignored = append(ignored, fakeIgnore{Path: ignorePath, Fake: lockfile})
		}
	}
	return generated, ignored, nil
}

func ensureFileExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("expected file, got directory: %s", path)
	}
	return nil
}

func generateNpmLockfile(ctx context.Context, defPath, outPath string, logf LogFunc) (string, error) {
	dir := filepath.Dir(defPath)
	lockPath := filepath.Join(dir, "package-lock.json")
	backupPath := ""
	if fileExists(lockPath) && filepath.Clean(lockPath) != filepath.Clean(outPath) {
		backupPath = lockPath + ".cxmpicheck." + fmt.Sprintf("%d", time.Now().Unix()) + ".bak"
		if err := os.Rename(lockPath, backupPath); err != nil {
			return "", err
		}
	}

	logf("Generating npm lockfile for %s", defPath)
	if err := requireTool("npm"); err != nil {
		return "", err
	}
	baseArgs := []string{"install", "--package-lock-only", "--ignore-scripts", "--no-audit", "--no-fund"}
	if err := runCommand(ctx, dir, logf, "npm", baseArgs...); err != nil {
		logf("npm lockfile generation failed, retrying with --legacy-peer-deps")
		if retryErr := runCommand(ctx, dir, logf, "npm", append(baseArgs, "--legacy-peer-deps")...); retryErr != nil {
			if backupPath != "" {
				_ = os.Rename(backupPath, lockPath)
			}
			return "", retryErr
		}
	}

	if !fileExists(lockPath) {
		if backupPath != "" {
			_ = os.Rename(backupPath, lockPath)
		}
		return "", fmt.Errorf("npm did not produce package-lock.json")
	}

	if err := os.MkdirAll(filepath.Dir(outPath), OwnerDirMode); err != nil {
		return "", err
	}
	if filepath.Clean(lockPath) != filepath.Clean(outPath) {
		if err := os.Rename(lockPath, outPath); err != nil {
			return "", err
		}
	}

	if backupPath != "" {
		if err := os.Rename(backupPath, lockPath); err != nil {
			return "", err
		}
	}

	if err := os.Chmod(outPath, OwnerFileMode); err != nil {
		return "", err
	}
	return outPath, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

type fakeSpec struct {
	Manager string
	Path    string
}

type fakeIgnore struct {
	Path string
	Fake string
}

func parseFakeSpecs(rootAbs string, specs []string) ([]fakeSpec, error) {
	parsed := []fakeSpec{}
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		manager, path := splitSpec(spec)
		if !filepath.IsAbs(path) {
			path = filepath.Join(rootAbs, path)
		}
		if manager == "" {
			manager = inferManager(path)
		}
		if manager == "" {
			return nil, fmt.Errorf("unsupported project definition file: %s", path)
		}
		parsed = append(parsed, fakeSpec{Manager: manager, Path: path})
	}
	return parsed, nil
}

func splitSpec(spec string) (string, string) {
	parts := strings.SplitN(spec, ":", 2)
	if len(parts) == 2 && parts[0] != "" {
		return strings.ToLower(parts[0]), parts[1]
	}
	return "", spec
}

func inferManager(path string) string {
	switch filepath.Base(path) {
	case npmPackageJSON:
		return "npm"
	case pnpmWorkspaceYAML, pnpmWorkspaceYML:
		return "pnpm"
	case pipfileName:
		return "pipenv"
	case poetryProjectName:
		if isPoetryPyproject(path) {
			return "poetry"
		}
		return "pip"
	case setupCfgName, setupIniName:
		return "pip"
	case requirementsInName, requirementsTxt:
		return "pip"
	case goModName:
		return "go"
	default:
		return ""
	}
}

func defaultFakeName(manager string) string {
	return fmt.Sprintf("cx.%s.mpicheck.lock", manager)
}

func fakeLockDir(rootAbs, outDir, defPath string) string {
	if outDir == "" {
		return filepath.Dir(defPath)
	}
	if filepath.IsAbs(outDir) {
		return outDir
	}
	return filepath.Join(rootAbs, outDir)
}

func lockfilesToIgnore(manager string, defPath string) []string {
	dir := filepath.Dir(defPath)
	switch manager {
	case "npm":
		return []string{filepath.Join(dir, "package-lock.json")}
	case "pnpm":
		return []string{
			filepath.Join(dir, "pnpm-lock.yaml"),
			filepath.Join(dir, "pnpm-lock.yml"),
		}
	case "pip":
		return []string{
			filepath.Join(dir, "requirements.txt"),
			filepath.Join(dir, "requirements.lock"),
		}
	case "pipenv":
		return []string{filepath.Join(dir, "Pipfile.lock")}
	case "poetry":
		return []string{filepath.Join(dir, "poetry.lock")}
	case "go":
		return []string{filepath.Join(dir, "go.mod")}
	default:
		return nil
	}
}

func generatePnpmLockfile(ctx context.Context, defPath, outPath string, logf LogFunc) (string, error) {
	dir := filepath.Dir(defPath)
	lockPath := filepath.Join(dir, "pnpm-lock.yaml")
	backupPath := ""
	if fileExists(lockPath) && filepath.Clean(lockPath) != filepath.Clean(outPath) {
		backupPath = lockPath + ".cxmpicheck." + fmt.Sprintf("%d", time.Now().Unix()) + ".bak"
		if err := os.Rename(lockPath, backupPath); err != nil {
			return "", err
		}
	}

	logf("Generating pnpm lockfile for %s", defPath)
	if err := requireTool("pnpm"); err != nil {
		return "", err
	}
	if err := runCommand(ctx, dir, logf, "pnpm", "install", "--lockfile-only", "--ignore-scripts", "--no-frozen-lockfile"); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, lockPath)
		}
		return "", err
	}

	if !fileExists(lockPath) {
		if backupPath != "" {
			_ = os.Rename(backupPath, lockPath)
		}
		return "", fmt.Errorf("pnpm did not produce pnpm-lock.yaml")
	}

	if err := os.MkdirAll(filepath.Dir(outPath), OwnerDirMode); err != nil {
		return "", err
	}
	if filepath.Clean(lockPath) != filepath.Clean(outPath) {
		if err := os.Rename(lockPath, outPath); err != nil {
			return "", err
		}
	}

	if backupPath != "" {
		if err := os.Rename(backupPath, lockPath); err != nil {
			return "", err
		}
	}
	if err := os.Chmod(outPath, OwnerFileMode); err != nil {
		return "", err
	}
	return outPath, nil
}

func generatePipenvLockfile(ctx context.Context, defPath, outPath string, logf LogFunc) (string, error) {
	dir := filepath.Dir(defPath)
	lockPath := filepath.Join(dir, "Pipfile.lock")
	backupPath := ""
	if fileExists(lockPath) && filepath.Clean(lockPath) != filepath.Clean(outPath) {
		backupPath = lockPath + ".cxmpicheck." + fmt.Sprintf("%d", time.Now().Unix()) + ".bak"
		if err := os.Rename(lockPath, backupPath); err != nil {
			return "", err
		}
	}

	logf("Generating pipenv lockfile for %s", defPath)
	if err := requireTool("pipenv"); err != nil {
		return "", err
	}
	if err := runCommand(ctx, dir, logf, "pipenv", "lock"); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, lockPath)
		}
		return "", err
	}

	if !fileExists(lockPath) {
		if backupPath != "" {
			_ = os.Rename(backupPath, lockPath)
		}
		return "", fmt.Errorf("pipenv did not produce Pipfile.lock")
	}

	if err := os.MkdirAll(filepath.Dir(outPath), OwnerDirMode); err != nil {
		return "", err
	}
	if filepath.Clean(lockPath) != filepath.Clean(outPath) {
		if err := os.Rename(lockPath, outPath); err != nil {
			return "", err
		}
	}

	if backupPath != "" {
		if err := os.Rename(backupPath, lockPath); err != nil {
			return "", err
		}
	}
	if err := os.Chmod(outPath, OwnerFileMode); err != nil {
		return "", err
	}
	return outPath, nil
}

func generatePoetryLockfile(ctx context.Context, defPath, outPath string, logf LogFunc) (string, error) {
	dir := filepath.Dir(defPath)
	lockPath := filepath.Join(dir, "poetry.lock")
	backupPath := ""
	if fileExists(lockPath) && filepath.Clean(lockPath) != filepath.Clean(outPath) {
		backupPath = lockPath + ".cxmpicheck." + fmt.Sprintf("%d", time.Now().Unix()) + ".bak"
		if err := os.Rename(lockPath, backupPath); err != nil {
			return "", err
		}
	}

	logf("Generating poetry lockfile for %s", defPath)
	if err := requireTool("poetry"); err != nil {
		return "", err
	}
	if err := runCommand(ctx, dir, logf, "poetry", "lock", "--no-update"); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, lockPath)
		}
		return "", err
	}

	if !fileExists(lockPath) {
		if backupPath != "" {
			_ = os.Rename(backupPath, lockPath)
		}
		return "", fmt.Errorf("poetry did not produce poetry.lock")
	}

	if err := os.MkdirAll(filepath.Dir(outPath), OwnerDirMode); err != nil {
		return "", err
	}
	if filepath.Clean(lockPath) != filepath.Clean(outPath) {
		if err := os.Rename(lockPath, outPath); err != nil {
			return "", err
		}
	}

	if backupPath != "" {
		if err := os.Rename(backupPath, lockPath); err != nil {
			return "", err
		}
	}
	if err := os.Chmod(outPath, OwnerFileMode); err != nil {
		return "", err
	}
	return outPath, nil
}

func generatePipLockfile(ctx context.Context, defPath, outPath string, logf LogFunc) (string, error) {
	dir := filepath.Dir(defPath)
	logf("Generating pip lockfile for %s", defPath)
	if err := requireTool("pip-compile"); err != nil {
		return "", err
	}
	if isPythonConfig(defPath) {
		hasDeps, err := hasPythonDependencies(defPath)
		if err != nil {
			return "", err
		}
		if !hasDeps {
			logf("No dependencies found in %s, skipping fake lockfile generation", defPath)
			return "", nil
		}
	}
	if err := runCommand(ctx, dir, logf, "pip-compile", defPath, "--output-file", outPath); err != nil {
		return "", err
	}
	if !fileExists(outPath) {
		return "", fmt.Errorf("pip-compile did not produce %s", outPath)
	}
	if err := os.Chmod(outPath, OwnerFileMode); err != nil {
		return "", err
	}
	return outPath, nil
}

func generateGoLockfile(ctx context.Context, defPath, outPath string, logf LogFunc) (string, error) {
	dir := filepath.Dir(defPath)
	logf("Generating go lockfile for %s", defPath)
	if err := requireTool("go"); err != nil {
		return "", err
	}
	if err := runCommand(ctx, dir, logf, "go", "mod", "tidy"); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), OwnerDirMode); err != nil {
		return "", err
	}
	data, err := os.ReadFile(defPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath, data, OwnerFileMode); err != nil {
		return "", err
	}
	return outPath, nil
}

func isPythonConfig(path string) bool {
	base := filepath.Base(path)
	switch base {
	case setupCfgName, setupIniName, poetryProjectName:
		return true
	default:
		return false
	}
}

func hasPythonDependencies(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	switch filepath.Base(path) {
	case setupCfgName, setupIniName:
		return hasSetupConfigDeps(string(data)), nil
	case poetryProjectName:
		return hasPyprojectDeps(data)
	default:
		return false, nil
	}
}

func hasSetupConfigDeps(content string) bool {
	inOptions := false
	found := false
	lines := strings.Split(content, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.ToLower(strings.Trim(line, "[]"))
			inOptions = section == "options"
			continue
		}
		if !inOptions {
			continue
		}
		if strings.HasPrefix(strings.ToLower(line), "install_requires") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
				return true
			}
			found = true
			continue
		}
		if found && (strings.HasPrefix(raw, " ") || strings.HasPrefix(raw, "\t")) {
			if strings.TrimSpace(line) != "" {
				return true
			}
			continue
		}
		if found && !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") {
			found = false
		}
	}
	return false
}

func hasPyprojectDeps(data []byte) (bool, error) {
	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return false, err
	}
	if project, ok := raw["project"].(map[string]interface{}); ok {
		if deps, ok := project["dependencies"].([]interface{}); ok && len(deps) > 0 {
			return true, nil
		}
		if optional, ok := project["optional-dependencies"].(map[string]interface{}); ok {
			for _, val := range optional {
				if list, ok := val.([]interface{}); ok && len(list) > 0 {
					return true, nil
				}
			}
		}
	}
	if tool, ok := raw["tool"].(map[string]interface{}); ok {
		if poetry, ok := tool["poetry"].(map[string]interface{}); ok {
			if deps, ok := poetry["dependencies"].(map[string]interface{}); ok {
				for name := range deps {
					if strings.ToLower(name) != "python" {
						return true, nil
					}
				}
			}
			if groups, ok := poetry["group"].(map[string]interface{}); ok {
				for _, groupVal := range groups {
					if group, ok := groupVal.(map[string]interface{}); ok {
						if deps, ok := group["dependencies"].(map[string]interface{}); ok && len(deps) > 0 {
							return true, nil
						}
					}
				}
			}
		}
	}
	return false, nil
}

func isPoetryPyproject(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var raw map[string]interface{}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return false
	}
	if tool, ok := raw["tool"].(map[string]interface{}); ok {
		if _, ok := tool["poetry"]; ok {
			return true
		}
	}
	return false
}

func requireTool(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s not found in PATH", name)
	}
	return nil
}

func runCommand(ctx context.Context, dir string, logf LogFunc, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	commandLine := strings.Join(append([]string{name}, args...), " ")
	started := time.Now()
	logf("Running external tool: %s (cwd: %s)", commandLine, dir)

	ticker := time.NewTicker(20 * time.Second)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ticker.C:
				logf("External tool still running (%s elapsed)", formatDuration(time.Since(started)))
			case <-done:
				return
			}
		}
	}()

	output, err := cmd.CombinedOutput()
	ticker.Stop()
	close(done)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ProcessState != nil {
			exitCode = exitErr.ProcessState.ExitCode()
		} else {
			exitCode = -1
		}
	}
	logf("External tool completed (exit %d, %s elapsed): %s", exitCode, formatDuration(time.Since(started)), commandLine)
	if err != nil {
		if len(output) > 0 {
			return fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(string(output)))
		}
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
}

func formatDuration(d time.Duration) string {
	ms := d.Milliseconds() % 1000
	sec := int(d.Seconds()) % 60
	min := int(d.Minutes())
	return fmt.Sprintf("%dm%ds%03dms", min, sec, ms)
}
