// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

type rewriteTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (r *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = r.target.Scheme
	req.URL.Host = r.target.Host
	return r.base.RoundTrip(req)
}

func TestRunWithMockMPIAPI(t *testing.T) {
	root := t.TempDir()
	lock := filepath.Join(root, "package-lock.json")
	if err := os.WriteFile(lock, []byte(`{"packages":{"node_modules/lodash":{"version":"4.17.21"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var reqPkgs []map[string]string
		if err := json.NewDecoder(r.Body).Decode(&reqPkgs); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		resp := []map[string]interface{}{}
		for _, pkg := range reqPkgs {
			resp = append(resp, map[string]interface{}{
				"name":    pkg["name"],
				"type":    pkg["type"],
				"version": pkg["version"],
				"risks":   []interface{}{map[string]interface{}{"score": 9, "title": "risk"}},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	target, _ := url.Parse(srv.URL)
	client := &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: target}}

	cfg := DefaultConfig()
	cfg.RootDir = root
	cfg.APIKey = "test-key"
	cfg.OutPackages = filepath.Join(root, "out.packages.json")
	cfg.OutResults = filepath.Join(root, "out.results.json")
	cfg.OutRisks = filepath.Join(root, "out.risks.json")
	cfg.HTTPClient = client
	cfg.BatchSize = 1000

	_, err := Run(context.Background(), cfg, func(string, ...interface{}) {})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	data, err := os.ReadFile(cfg.OutRisks)
	if err != nil {
		t.Fatalf("read risks error: %v", err)
	}
	var risks []RiskRecord
	if err := json.Unmarshal(data, &risks); err != nil {
		t.Fatalf("unmarshal risks error: %v", err)
	}
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk record, got %d", len(risks))
	}
	if len(risks[0].Lockfiles) != 1 || risks[0].Lockfiles[0] != "package-lock.json" {
		t.Fatalf("expected relative lockfile path, got %v", risks[0].Lockfiles)
	}
}

func TestFilterLockfilesAlways(t *testing.T) {
	lockfiles := []LockfileRef{
		{Path: "/tmp/real/package-lock.json", Ecosystem: EcosystemNPM},
		{Path: "/tmp/real/go.mod", Ecosystem: EcosystemGo},
		{Path: "/tmp/fake/cx.npm.mpicheck.lock", Ecosystem: EcosystemNPM},
	}
	generated := []string{"/tmp/fake/cx.npm.mpicheck.lock"}
	filtered := filterLockfilesAlways(func(string, ...interface{}) {}, lockfiles, generated)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 lockfiles, got %d", len(filtered))
	}
}
