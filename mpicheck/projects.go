// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import (
	"io/fs"
	"path/filepath"
)

// DiscoverProjectFiles finds supported project definition files under root.
//
// Root and excludes are canonicalized via filepath.EvalSymlinks so an
// --exclude entry pointing at a symlink correctly suppresses the
// symlink's target during the walk. See DiscoverLockfiles for details.
func DiscoverProjectFiles(root string, exclude []string) ([]string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	rootAbs = resolveSymlinks(rootAbs)
	excludeAbs, err := resolvePaths(rootAbs, exclude)
	if err != nil {
		return nil, err
	}
	projects := map[string]struct{}{}
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
		if isProjectFile(path) {
			projects[path] = struct{}{}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(projects))
	for p := range projects {
		paths = append(paths, p)
	}
	return paths, nil
}

func isProjectFile(path string) bool {
	switch filepath.Base(path) {
	case "package.json", "pnpm-workspace.yaml", "pnpm-workspace.yml", "Pipfile", "pyproject.toml", "setup.cfg", "setup.ini", "requirements.in", "requirements.txt", "go.mod":
		return true
	default:
		return false
	}
}
