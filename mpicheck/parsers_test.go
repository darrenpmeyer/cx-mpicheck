// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import (
	"encoding/json"
	"testing"
)

func TestParsePackageLockJSONPackages(t *testing.T) {
	data := []byte(`{
		"packages": {
			"": {"name": "root", "version": "0.0.0"},
			"node_modules/lodash": {"version": "4.17.21"},
			"node_modules/@scope/pkg": {"version": "1.2.3"}
		}
	}`)
	pkgs, err := parsePackageLockJSON("package-lock.json", data)
	if err != nil {
		t.Fatalf("parsePackageLockJSON error: %v", err)
	}
	assertHasPackage(t, pkgs, "lodash", "4.17.21", EcosystemNPM)
	assertHasPackage(t, pkgs, "@scope/pkg", "1.2.3", EcosystemNPM)
}

func TestParsePackageLockJSONDependencies(t *testing.T) {
	data := []byte(`{
		"dependencies": {
			"react": {"version": "18.2.0", "dependencies": {"loose-envify": {"version": "1.4.0"}}}
		}
	}`)
	pkgs, err := parsePackageLockJSON("package-lock.json", data)
	if err != nil {
		t.Fatalf("parsePackageLockJSON error: %v", err)
	}
	assertHasPackage(t, pkgs, "react", "18.2.0", EcosystemNPM)
	assertHasPackage(t, pkgs, "loose-envify", "1.4.0", EcosystemNPM)
}

func TestParsePnpmLockYAML(t *testing.T) {
	data := []byte("packages:\n  /lodash/4.17.21: {}\n  /@scope/pkg/1.2.3: {}\n")
	pkgs, err := parsePnpmLockYAML("pnpm-lock.yaml", data)
	if err != nil {
		t.Fatalf("parsePnpmLockYAML error: %v", err)
	}
	assertHasPackage(t, pkgs, "lodash", "4.17.21", EcosystemNPM)
	assertHasPackage(t, pkgs, "@scope/pkg", "1.2.3", EcosystemNPM)
}

func TestParseRequirementsTXT(t *testing.T) {
	data := []byte("# comment\nrequests==2.31.0\n-r other.txt\n-e git+https://example.com/pkg\nFlask==2.3.2; python_version>='3.8'\n")
	pkgs, err := parseRequirementsTXT("requirements.txt", data)
	if err != nil {
		t.Fatalf("parseRequirementsTXT error: %v", err)
	}
	assertHasPackage(t, pkgs, "requests", "2.31.0", EcosystemPyPI)
	assertHasPackage(t, pkgs, "flask", "2.3.2", EcosystemPyPI)
}

func TestParsePipfileLock(t *testing.T) {
	data := []byte(`{"default":{"requests":{"version":"==2.31.0"}},"develop":{"pytest":{"version":"==7.4.0"}}}`)
	pkgs, err := parsePipfileLock("Pipfile.lock", data)
	if err != nil {
		t.Fatalf("parsePipfileLock error: %v", err)
	}
	assertHasPackage(t, pkgs, "requests", "2.31.0", EcosystemPyPI)
	assertHasPackage(t, pkgs, "pytest", "7.4.0", EcosystemPyPI)
}

func TestParsePoetryLock(t *testing.T) {
	data := []byte("[[package]]\nname = \"requests\"\nversion = \"2.31.0\"\n\n[[package]]\nname = \"pytest\"\nversion = \"7.4.0\"\n")
	pkgs, err := parsePoetryLock("poetry.lock", data)
	if err != nil {
		t.Fatalf("parsePoetryLock error: %v", err)
	}
	assertHasPackage(t, pkgs, "requests", "2.31.0", EcosystemPyPI)
	assertHasPackage(t, pkgs, "pytest", "7.4.0", EcosystemPyPI)
}

func TestParseGoMod(t *testing.T) {
	data := []byte("module example.com/app\n\nrequire (\n\texample.com/lib v1.2.3\n)\n")
	pkgs, err := parseGoMod("go.mod", data)
	if err != nil {
		t.Fatalf("parseGoMod error: %v", err)
	}
	assertHasPackage(t, pkgs, "example.com/lib", "v1.2.3", EcosystemGo)
}

func TestNormalizePyPIName(t *testing.T) {
	if got := normalizePyPIName("My_Pkg.Name"); got != "my-pkg-name" {
		t.Fatalf("normalizePyPIName got %s", got)
	}
}

func assertHasPackage(t *testing.T, pkgs []PackageVersion, name string, version string, eco Ecosystem) {
	t.Helper()
	for _, pkg := range pkgs {
		if pkg.Name == name && pkg.Version == version && pkg.Ecosystem == eco {
			return
		}
	}
	payload, _ := json.Marshal(pkgs)
	t.Fatalf("expected package %s@%s (%s), got %s", name, version, eco, string(payload))
}
