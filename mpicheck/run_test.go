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
	"strings"
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
	cfg.APIKey = "aaaa.bbbb.cccc"
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

func TestValidateAPIKey(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", wantErr: true},
		{name: "whitespace only", input: "   \n\t  ", wantErr: true},
		{name: "well-formed JWT", input: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature123", want: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature123"},
		{name: "JWT with hyphens and underscores", input: "abc-def_ghi.payload.sig-123_xyz", want: "abc-def_ghi.payload.sig-123_xyz"},
		{name: "JWT with surrounding whitespace gets trimmed", input: "  aaa.bbb.ccc\n", want: "aaa.bbb.ccc"},
		{name: "JWT with trailing padding", input: "aaa==.bbb=.ccc", want: "aaa==.bbb=.ccc"},
		{name: "two segments fails", input: "aaa.bbb", wantErr: true},
		{name: "four segments fails", input: "aaa.bbb.ccc.ddd", wantErr: true},
		{name: "empty segment fails", input: "aaa..ccc", wantErr: true},
		{name: "spaces inside fail", input: "aaa.bb b.ccc", wantErr: true},
		{name: "non-base64 chars fail", input: "aaa.bbb!.ccc", wantErr: true},
		{name: "standard-base64 '+' rejected (JWT is URL-safe)", input: "aa+a.bbb.ccc", wantErr: true},
		{name: "standard-base64 '/' rejected (JWT is URL-safe)", input: "aaa.b/bb.ccc", wantErr: true},
		{name: "newline inside fails", input: "aaa.bbb\n.ccc", wantErr: true},
		{name: "too long", input: strings.Repeat("a", 2045) + ".b.c", wantErr: true},
		{name: "right at limit", input: strings.Repeat("a", 2044) + ".b.c", want: strings.Repeat("a", 2044) + ".b.c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateAPIKey(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (got=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateAPIKeyErrorDoesNotEchoKey(t *testing.T) {
	sentinel := "SUPERSECRETKEYDONOTLEAK"
	_, err := validateAPIKey(sentinel + "!!!") // invalid char triggers format error
	if err == nil {
		t.Fatal("expected validation error")
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("error message echoed the key value: %q", err.Error())
	}
}

func TestRunValidatesAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RootDir = t.TempDir()
	cfg.APIKey = "not-a-jwt"

	_, err := Run(context.Background(), cfg, func(string, ...interface{}) {})
	if err == nil {
		t.Fatal("expected Run to reject a malformed API key")
	}
	typed, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if typed.Code != errCodeConfig {
		t.Fatalf("expected errCodeConfig (%d), got %d", errCodeConfig, typed.Code)
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
