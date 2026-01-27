// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import (
	"bufio"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

func parsePackageLockJSON(_ string, data []byte) ([]PackageVersion, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	if packagesRaw, ok := raw["packages"]; ok {
		return parseNpmPackagesMap(packagesRaw)
	}

	if depsRaw, ok := raw["dependencies"]; ok {
		return parseNpmDependenciesMap(depsRaw)
	}

	return nil, fmt.Errorf("package-lock.json missing packages or dependencies")
}

func parseNpmPackagesMap(raw interface{}) ([]PackageVersion, error) {
	packages := []PackageVersion{}
	pkgMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected packages format")
	}
	for path, entryRaw := range pkgMap {
		if path == "" {
			continue
		}
		entry, ok := entryRaw.(map[string]interface{})
		if !ok {
			continue
		}
		version, _ := entry["version"].(string)
		if version == "" {
			continue
		}
		name, _ := entry["name"].(string)
		if name == "" {
			name = nameFromNodeModulesPath(path)
		}
		if name == "" {
			continue
		}
		packages = append(packages, PackageVersion{
			Ecosystem: EcosystemNPM,
			Name:      name,
			Version:   version,
		})
	}
	return packages, nil
}

func parseNpmDependenciesMap(raw interface{}) ([]PackageVersion, error) {
	packages := []PackageVersion{}
	depsMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected dependencies format")
	}
	var walk func(map[string]interface{})
	walk = func(node map[string]interface{}) {
		for name, depRaw := range node {
			dep, ok := depRaw.(map[string]interface{})
			if !ok {
				continue
			}
			version, _ := dep["version"].(string)
			if version != "" {
				packages = append(packages, PackageVersion{
					Ecosystem: EcosystemNPM,
					Name:      name,
					Version:   version,
				})
			}
			if nested, ok := dep["dependencies"].(map[string]interface{}); ok {
				walk(nested)
			}
		}
	}
	walk(depsMap)
	return packages, nil
}

func nameFromNodeModulesPath(path string) string {
	parts := strings.Split(path, "node_modules")
	if len(parts) < 2 {
		return ""
	}
	last := parts[len(parts)-1]
	last = strings.TrimPrefix(last, "/")
	last = strings.TrimPrefix(last, "\\")
	if last == "" {
		return ""
	}
	return last
}

func parsePnpmLockYAML(_ string, data []byte) ([]PackageVersion, error) {
	var raw struct {
		Packages map[string]interface{} `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	packages := []PackageVersion{}
	for key := range raw.Packages {
		name, version := parsePnpmKey(key)
		if name == "" || version == "" {
			continue
		}
		packages = append(packages, PackageVersion{
			Ecosystem: EcosystemNPM,
			Name:      name,
			Version:   version,
		})
	}
	return packages, nil
}

func parsePnpmKey(key string) (string, string) {
	key = strings.TrimPrefix(key, "/")
	key = strings.TrimSpace(key)
	if key == "" {
		return "", ""
	}
	key = strings.SplitN(key, "(", 2)[0]
	segments := strings.Split(key, "/")
	if len(segments) < 2 {
		return "", ""
	}
	version := segments[len(segments)-1]
	if strings.HasPrefix(segments[0], "@") {
		if len(segments) < 3 {
			return "", ""
		}
		name := strings.Join(segments[:2], "/")
		return name, version
	}
	return segments[0], version
}

var requirementLine = regexp.MustCompile(`^\s*([A-Za-z0-9_.-]+)\s*([=]{2,3})\s*([^\s;]+)`) // name==version

func parseRequirementsTXT(_ string, data []byte) ([]PackageVersion, error) {
	packages := []PackageVersion{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "-r") || strings.HasPrefix(line, "--requirement") {
			continue
		}
		if strings.HasPrefix(line, "-e") || strings.HasPrefix(line, "--editable") {
			continue
		}
		if strings.Contains(line, "@") && strings.Contains(line, "://") {
			continue
		}
		matches := requirementLine.FindStringSubmatch(line)
		if len(matches) < 4 {
			continue
		}
		name := normalizePyPIName(matches[1])
		version := matches[3]
		packages = append(packages, PackageVersion{Ecosystem: EcosystemPyPI, Name: name, Version: version})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return packages, nil
}

func parsePipfileLock(_ string, data []byte) ([]PackageVersion, error) {
	var raw struct {
		Default map[string]struct {
			Version string `json:"version"`
		} `json:"default"`
		Develop map[string]struct {
			Version string `json:"version"`
		} `json:"develop"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	packages := []PackageVersion{}
	add := func(name, version string) {
		version = strings.TrimPrefix(version, "==")
		if version == "" {
			return
		}
		packages = append(packages, PackageVersion{Ecosystem: EcosystemPyPI, Name: normalizePyPIName(name), Version: version})
	}
	for name, pkg := range raw.Default {
		add(name, pkg.Version)
	}
	for name, pkg := range raw.Develop {
		add(name, pkg.Version)
	}
	return packages, nil
}

func parsePoetryLock(_ string, data []byte) ([]PackageVersion, error) {
	var raw struct {
		Packages []struct {
			Name    string `toml:"name"`
			Version string `toml:"version"`
		} `toml:"package"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	packages := []PackageVersion{}
	for _, pkg := range raw.Packages {
		if pkg.Name == "" || pkg.Version == "" {
			continue
		}
		packages = append(packages, PackageVersion{Ecosystem: EcosystemPyPI, Name: normalizePyPIName(pkg.Name), Version: pkg.Version})
	}
	return packages, nil
}

func normalizePyPIName(name string) string {
	name = strings.ToLower(name)
	replacer := regexp.MustCompile(`[-_.]+`)
	return replacer.ReplaceAllString(name, "-")
}

func parseGoMod(path string, data []byte) ([]PackageVersion, error) {
	file, err := modfile.Parse(path, data, nil)
	if err != nil {
		return nil, err
	}
	packages := []PackageVersion{}
	for _, req := range file.Require {
		if req == nil || req.Mod.Path == "" || req.Mod.Version == "" {
			continue
		}
		packages = append(packages, PackageVersion{
			Ecosystem: EcosystemGo,
			Name:      req.Mod.Path,
			Version:   req.Mod.Version,
		})
	}
	return packages, nil
}
