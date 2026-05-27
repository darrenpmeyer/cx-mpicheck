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
//
// All paths (root, explicit lockfile paths, excludes) are canonicalized
// via filepath.EvalSymlinks where possible, so an --exclude entry that
// points at a symlink correctly suppresses the symlink's target during
// the walk. WalkDir itself still does not follow symlinks, so an
// in-tree symlink does not cause the walk to leave the resolved root.
func DiscoverLockfiles(root string, explicit []string, mode IncludeMode, exclude []string, handlers []LockfileHandler) ([]LockfileRef, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	rootAbs = resolveSymlinks(rootAbs)

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
		pathAbs = resolveSymlinks(pathAbs)
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
		if isFakeLockfileArtifact(path) {
			// Skip cx-mpicheck-generated fake lockfiles during the
			// recursive walk. The handler Match functions still claim
			// them (so they remain valid as explicit --lockfile/
			// --fake-lockfile inputs and as auto-added generation
			// outputs), but a stale artifact from a prior run must
			// not be merged with a real lockfile in the same
			// directory — that would double-count packages whose
			// resolved versions diverged between the real and the
			// regenerated lockfiles.
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
		resolved = append(resolved, resolveSymlinks(abs))
	}
	return resolved, nil
}

// isFakeLockfileArtifact reports whether path looks like a
// cx-mpicheck-generated fake lockfile (e.g. cx.npm.mpicheck.lock,
// cx.pip.mpicheck.lock). These files are tool output, not user-
// authored lockfiles. They remain valid as explicit --lockfile or
// --fake-lockfile inputs (the handler Match functions still claim
// the names), but the recursive walk skips them so a stale artifact
// from a prior run cannot be merged with a real lockfile in the
// same directory.
func isFakeLockfileArtifact(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "cx.") && strings.HasSuffix(base, ".mpicheck.lock")
}

// resolveSymlinks returns the canonical path with symlinks resolved.
// If resolution fails — most commonly because the path doesn't exist
// on disk — the input is returned unchanged. This lets callers pass
// speculative paths (e.g. an --exclude entry that may not exist in
// the tree) without aborting the whole discovery, while still letting
// real symlinks be matched against walk results.
func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
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
