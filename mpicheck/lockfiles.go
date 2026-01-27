// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// LockfileHandler handles a specific lockfile format.
type LockfileHandler struct {
	Kind      string
	Ecosystem Ecosystem
	Match     func(path string) bool
	Parse     func(path string, data []byte) ([]PackageVersion, error)
}

// LockfileRef identifies a lockfile for parsing.
type LockfileRef struct {
	Path      string
	Kind      string
	Ecosystem Ecosystem
}

// Handlers returns the built-in lockfile handlers.
func Handlers() []LockfileHandler {
	return []LockfileHandler{
		{
			Kind:      "npm-package-lock",
			Ecosystem: EcosystemNPM,
			Match: func(path string) bool {
				base := filepath.Base(path)
				return base == "package-lock.json" || base == "cx.npm.mpicheck.lock"
			},
			Parse: parsePackageLockJSON,
		},
		{
			Kind:      "pnpm-lock",
			Ecosystem: EcosystemNPM,
			Match: func(path string) bool {
				base := filepath.Base(path)
				return base == "pnpm-lock.yaml" || base == "pnpm-lock.yml" || base == "cx.pnpm.mpicheck.lock"
			},
			Parse: parsePnpmLockYAML,
		},
		{
			Kind:      "pip-requirements",
			Ecosystem: EcosystemPyPI,
			Match: func(path string) bool {
				base := filepath.Base(path)
				if base == "cx.pip.mpicheck.lock" {
					return true
				}
				if strings.HasPrefix(base, "requirements") && strings.HasSuffix(base, ".txt") {
					return true
				}
				return base == "requirements.txt" || base == "requirements.lock"
			},
			Parse: parseRequirementsTXT,
		},
		{
			Kind:      "pipenv-lock",
			Ecosystem: EcosystemPyPI,
			Match: func(path string) bool {
				base := filepath.Base(path)
				return base == "Pipfile.lock" || base == "cx.pipenv.mpicheck.lock"
			},
			Parse: parsePipfileLock,
		},
		{
			Kind:      "poetry-lock",
			Ecosystem: EcosystemPyPI,
			Match: func(path string) bool {
				base := filepath.Base(path)
				return base == "poetry.lock" || base == "cx.poetry.mpicheck.lock"
			},
			Parse: parsePoetryLock,
		},
		{
			Kind:      "go-mod",
			Ecosystem: EcosystemGo,
			Match: func(path string) bool {
				base := filepath.Base(path)
				return base == "go.mod" || base == "cx.go.mpicheck.lock"
			},
			Parse: parseGoMod,
		},
	}
}

// DiscoverLockfiles finds lockfiles under root and merges explicit paths.
func DiscoverLockfiles(root string, explicit []string, mode IncludeMode, exclude []string, handlers []LockfileHandler) ([]LockfileRef, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	explicitAbs, err := resolvePaths(rootAbs, explicit)
	if err != nil {
		return nil, err
	}

	excludeAbs, err := resolvePaths(rootAbs, exclude)
	if err != nil {
		return nil, err
	}

	lockfiles := map[string]LockfileRef{}

	addLockfile := func(path string) error {
		pathAbs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if isExcluded(pathAbs, excludeAbs) {
			return nil
		}
		for _, h := range handlers {
			if h.Match(pathAbs) {
				lockfiles[pathAbs] = LockfileRef{Path: pathAbs, Kind: h.Kind, Ecosystem: h.Ecosystem}
				return nil
			}
		}
		return fmt.Errorf("unsupported lockfile: %s", pathAbs)
	}

	if mode == IncludeOnly {
		for _, p := range explicitAbs {
			if err := addLockfile(p); err != nil {
				return nil, err
			}
		}
		return mapToLockfiles(lockfiles), nil
	}

	for _, p := range explicitAbs {
		if err := addLockfile(p); err != nil {
			return nil, err
		}
	}

	if err := filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if isExcluded(path, excludeAbs) {
				return fs.SkipDir
			}
			return nil
		}
		if isExcluded(path, excludeAbs) {
			return nil
		}
		for _, h := range handlers {
			if h.Match(path) {
				lockfiles[path] = LockfileRef{Path: path, Kind: h.Kind, Ecosystem: h.Ecosystem}
				break
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return mapToLockfiles(lockfiles), nil
}

func mapToLockfiles(m map[string]LockfileRef) []LockfileRef {
	lockfiles := make([]LockfileRef, 0, len(m))
	for _, lf := range m {
		lockfiles = append(lockfiles, lf)
	}
	return lockfiles
}

func resolvePaths(root string, paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	resolved := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, abs)
	}
	return resolved, nil
}

func isExcluded(path string, excludes []string) bool {
	for _, ex := range excludes {
		info, err := os.Stat(ex)
		if err == nil && info.IsDir() {
			if isSubpath(ex, path) {
				return true
			}
			continue
		}
		if samePath(ex, path) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func isSubpath(dir, path string) bool {
	dir = filepath.Clean(dir) + string(os.PathSeparator)
	path = filepath.Clean(path)
	return strings.HasPrefix(path, dir)
}
