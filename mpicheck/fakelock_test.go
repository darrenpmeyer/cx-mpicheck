// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupExistingLockfileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "package-lock.json")
	outPath := filepath.Join(dir, "cx.npm.mpicheck.lock")
	original := []byte(`{"original":true}`)
	if err := os.WriteFile(lockPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	backupPath, restore, err := backupExistingLockfile(lockPath, outPath)
	if err != nil {
		t.Fatalf("backup error: %v", err)
	}
	if backupPath == "" {
		t.Fatal("expected non-empty backup path")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lockPath to be empty after backup; stat err=%v", err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	restore()
	got, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read after restore: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("restored content mismatch: got %q want %q", got, original)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("backup should be gone after restore; stat err=%v", err)
	}

	// Second restore is a no-op (backup already gone).
	restore()
}

func TestBackupExistingLockfileNoOriginal(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "package-lock.json")
	outPath := filepath.Join(dir, "cx.npm.mpicheck.lock")

	backupPath, restore, err := backupExistingLockfile(lockPath, outPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if backupPath != "" {
		t.Fatalf("expected empty backup path when original is absent, got %q", backupPath)
	}
	restore() // must not panic
}

func TestBackupExistingLockfileSamePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "package-lock.json")
	if err := os.WriteFile(p, []byte(`x`), 0o644); err != nil {
		t.Fatal(err)
	}

	backupPath, _, err := backupExistingLockfile(p, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if backupPath != "" {
		t.Fatalf("expected no backup when lockPath == outPath, got %q", backupPath)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("original should be untouched when paths match: %v", err)
	}
}

func TestValidateGeneratedLockfile(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "missing.lock")
	if err := validateGeneratedLockfile(missing, "npm"); err == nil {
		t.Fatal("expected error for missing file")
	}

	empty := filepath.Join(dir, "empty.lock")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateGeneratedLockfile(empty, "npm"); err == nil {
		t.Fatal("expected error for empty file")
	}

	asDir := filepath.Join(dir, "as-dir.lock")
	if err := os.Mkdir(asDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateGeneratedLockfile(asDir, "npm"); err == nil {
		t.Fatal("expected error when target is a directory")
	}

	good := filepath.Join(dir, "good.lock")
	if err := os.WriteFile(good, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateGeneratedLockfile(good, "npm"); err != nil {
		t.Fatalf("unexpected error for valid file: %v", err)
	}
}
