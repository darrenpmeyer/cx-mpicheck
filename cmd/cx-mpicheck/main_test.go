// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/cx-mpicheck", "--version")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "CHECKMARX_MPIAPI_KEY=dummy")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v (%s)", err, string(out))
	}
	if len(bytes.TrimSpace(out)) == 0 {
		t.Fatalf("expected version output")
	}
}

func TestMissingAPIKeyMessage(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/cx-mpicheck")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "CHECKMARX_MPIAPI_KEY=")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error due to missing key")
	}
	if !strings.Contains(string(out), "CHECKMARX_MPIAPI_KEY is required") {
		t.Fatalf("expected missing key message, got: %s", string(out))
	}
}

func TestPrettyBanner(t *testing.T) {
	root := t.TempDir()
	risks := filepath.Join(root, "cx.mpiapi-risks.json")
	if err := os.WriteFile(risks, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/cx-mpicheck", "--pretty", "--out-risks", risks)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "CHECKMARX_MPIAPI_KEY=dummy")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %v (%s)", err, string(out))
	}
	if !strings.Contains(string(out), "Malicious package checker") {
		t.Fatalf("expected banner in output")
	}
}

func TestResolveFlagValidation(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/cx-mpicheck", "--resolve", "bogus")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "CHECKMARX_MPIAPI_KEY=dummy")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected failure for invalid resolve mode")
	}
	if !strings.Contains(string(out), "Invalid resolve mode") {
		t.Fatalf("expected invalid resolve mode message, got: %s", string(out))
	}
}

func TestFakeLockfileOutFlag(t *testing.T) {
	root := t.TempDir()
	packageJSON := filepath.Join(root, "package.json")
	if err := os.WriteFile(packageJSON, []byte(`{"name":"x","version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/cx-mpicheck", "--fake-lockfile", packageJSON, "--fake-lockfile-out", root, "--resolve", "never", "--version")
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "CHECKMARX_MPIAPI_KEY=dummy")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("unexpected error: %v (%s)", err, string(out))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

func TestSplitList(t *testing.T) {
	items := splitList("a,b; c")
	if len(items) != 3 || items[0] != "a" || items[1] != "b" || items[2] != "c" {
		t.Fatalf("unexpected split list: %v", items)
	}
}

func TestParseBool(t *testing.T) {
	if v, err := parseBool("yes"); err != nil || !v {
		t.Fatalf("expected yes to be true")
	}
	if v, err := parseBool("0"); err != nil || v {
		t.Fatalf("expected 0 to be false")
	}
}

func TestColorNoop(t *testing.T) {
	msg := color("x", "unknown")
	if msg != "x" {
		t.Fatalf("expected no color, got %s", msg)
	}
}

func TestDecorateMessagePolicy(t *testing.T) {
	msg := decorateMessage("Packages matched policy: 2 (total risks: 3)")
	if msg == "Packages matched policy: 2 (total risks: 3)" {
		t.Fatalf("expected decorated message")
	}
}
