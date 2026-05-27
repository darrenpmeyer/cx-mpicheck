// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiscoverLockfilesIncludeOnly(t *testing.T) {
	root := t.TempDir()
	lock := filepath.Join(root, "package-lock.json")
	if err := os.WriteFile(lock, []byte(`{"packages":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	lockfiles, err := DiscoverLockfiles(root, []string{lock}, IncludeOnly, nil, Handlers())
	if err != nil {
		t.Fatalf("DiscoverLockfiles error: %v", err)
	}
	if len(lockfiles) != 1 {
		t.Fatalf("expected 1 lockfile, got %d", len(lockfiles))
	}
}

func TestDiscoverLockfilesExclude(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(sub, "package-lock.json")
	if err := os.WriteFile(lock, []byte(`{"packages":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	lockfiles, err := DiscoverLockfiles(root, nil, IncludeAlso, []string{sub}, Handlers())
	if err != nil {
		t.Fatalf("DiscoverLockfiles error: %v", err)
	}
	if len(lockfiles) != 0 {
		t.Fatalf("expected 0 lockfiles, got %d", len(lockfiles))
	}
}

func TestDiscoverLockfilesUnsupported(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "unknown.lock")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := DiscoverLockfiles(root, []string{file}, IncludeOnly, nil, Handlers())
	if err == nil {
		t.Fatalf("expected error for unsupported lockfile")
	}
}

func TestDiscoverLockfilesExcludeThroughSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation on Windows requires elevated privileges; not part of normal CI")
	}
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "package-lock.json"), []byte(`{"packages":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	vendor := filepath.Join(root, "vendor")
	if err := os.Symlink(realDir, vendor); err != nil {
		t.Skipf("symlink creation not permitted: %v", err)
	}

	// Without symlink resolution, --exclude vendor would not match
	// /<root>/real/package-lock.json (which is the path WalkDir yields).
	// With resolveSymlinks applied to the exclude entry, the exclusion
	// resolves to /<root>/real and the lockfile is correctly suppressed.
	lockfiles, err := DiscoverLockfiles(root, nil, IncludeAlso, []string{"vendor"}, Handlers())
	if err != nil {
		t.Fatalf("DiscoverLockfiles error: %v", err)
	}
	if len(lockfiles) != 0 {
		t.Fatalf("expected 0 lockfiles (vendor symlink should exclude its target), got %d: %+v", len(lockfiles), lockfiles)
	}
}

func TestResolveSymlinksFallsBackForMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	got := resolveSymlinks(missing)
	if got != missing {
		t.Fatalf("expected unchanged path for missing target, got %q", got)
	}
}
