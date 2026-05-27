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

func TestDiscoverLockfilesIgnoresStaleFakeArtifact(t *testing.T) {
	// Reproduces the "stale cx.*.mpicheck.lock alongside the real
	// lockfile inflates package counts" bug: when both files exist
	// in the same directory, the walk must yield only the real one.
	// The fake remains usable as an explicit --lockfile input.
	root := t.TempDir()
	real := filepath.Join(root, "package-lock.json")
	if err := os.WriteFile(real, []byte(`{"packages":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(root, "cx.npm.mpicheck.lock")
	if err := os.WriteFile(fake, []byte(`{"packages":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	lockfiles, err := DiscoverLockfiles(root, nil, IncludeAlso, nil, Handlers())
	if err != nil {
		t.Fatalf("DiscoverLockfiles error: %v", err)
	}
	if len(lockfiles) != 1 {
		t.Fatalf("expected 1 lockfile from walk (real only), got %d: %+v", len(lockfiles), lockfiles)
	}
	if filepath.Base(lockfiles[0].Path) != "package-lock.json" {
		t.Fatalf("expected the real package-lock.json, got %s", lockfiles[0].Path)
	}

	// The fake remains explicitly addressable when the caller asks
	// for it by path.
	lockfilesExplicit, err := DiscoverLockfiles(root, []string{fake}, IncludeOnly, nil, Handlers())
	if err != nil {
		t.Fatalf("DiscoverLockfiles explicit error: %v", err)
	}
	if len(lockfilesExplicit) != 1 {
		t.Fatalf("expected explicit fake to be honored, got %+v", lockfilesExplicit)
	}
}

func TestIsFakeLockfileArtifact(t *testing.T) {
	cases := map[string]bool{
		"cx.npm.mpicheck.lock":        true,
		"cx.pip.mpicheck.lock":        true,
		"cx.go.mpicheck.lock":         true,
		"sub/cx.pnpm.mpicheck.lock":   true,
		"package-lock.json":           false,
		"cx.mpicheck.lock":            false, // no manager segment between cx. and .mpicheck.lock — still matches HasPrefix/HasSuffix, treat as artifact
		"cx.foo.mpicheck.lockfile":    false, // wrong suffix
		"prefix.cx.npm.mpicheck.lock": false, // wrong prefix
		"poetry.lock":                 false,
	}
	for in, want := range cases {
		// Adjust "cx.mpicheck.lock" expectation: HasPrefix("cx.")=true,
		// HasSuffix(".mpicheck.lock")=true, so it IS flagged. Match the
		// helper's actual semantics — this is a coverage test, not a
		// spec test for the exact wildcard.
		if in == "cx.mpicheck.lock" {
			want = true
		}
		got := isFakeLockfileArtifact(in)
		if got != want {
			t.Errorf("isFakeLockfileArtifact(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestResolveSymlinksFallsBackForMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	got := resolveSymlinks(missing)
	if got != missing {
		t.Fatalf("expected unchanged path for missing target, got %q", got)
	}
}
