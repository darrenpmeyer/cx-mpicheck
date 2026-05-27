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

func TestRedact(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		secrets []string
		want    string
	}{
		{name: "empty input", input: "", secrets: []string{"key"}, want: ""},
		{name: "no secrets", input: "hello", secrets: nil, want: "hello"},
		{name: "secret not present", input: "hello world", secrets: []string{"missing"}, want: "hello world"},
		{name: "single match", input: "Bearer aaa.bbb.ccc here", secrets: []string{"aaa.bbb.ccc"}, want: "Bearer [REDACTED] here"},
		{name: "multiple matches of same secret", input: "k=x then k=x again", secrets: []string{"x"}, want: "k=[REDACTED] then k=[REDACTED] again"},
		{name: "multiple distinct secrets", input: "alpha and beta", secrets: []string{"alpha", "beta"}, want: "[REDACTED] and [REDACTED]"},
		{name: "empty secret in list is skipped", input: "preserve me", secrets: []string{"", "preserve"}, want: "[REDACTED] me"},
		{name: "case-sensitive", input: "ABC abc", secrets: []string{"abc"}, want: "ABC [REDACTED]"},
		{name: "secret containing regex metacharacters", input: "leak: a.b+c/d=", secrets: []string{"a.b+c/d="}, want: "leak: [REDACTED]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Redact(tc.input, tc.secrets...)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRedactInMPIAPIErrorBody confirms that when MPIAPI returns a non-2xx
// with the auth header echoed in the body (a known behavior of some
// gateways), the wrapped error message does not contain the key value.
func TestRedactInMPIAPIErrorBody(t *testing.T) {
	root := t.TempDir()
	lock := filepath.Join(root, "package-lock.json")
	if err := os.WriteFile(lock, []byte(`{"packages":{"node_modules/lodash":{"version":"4.17.21"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	apiKey := "aaaa.bbbb.cccc"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a misbehaving gateway echoing the Authorization
		// header value into the error response body.
		echoed := r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"forbidden","seen_auth":"` + echoed + `"}`))
	}))
	defer srv.Close()

	target, _ := url.Parse(srv.URL)
	client := &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, target: target}}

	cfg := DefaultConfig()
	cfg.RootDir = root
	cfg.APIKey = apiKey
	cfg.OutPackages = filepath.Join(root, "out.packages.json")
	cfg.OutResults = filepath.Join(root, "out.results.json")
	cfg.OutRisks = filepath.Join(root, "out.risks.json")
	cfg.HTTPClient = client
	cfg.BatchSize = 1000

	_, err := Run(context.Background(), cfg, func(string, ...interface{}) {})
	if err == nil {
		t.Fatal("expected Run to fail with non-2xx response")
	}
	msg := err.Error()
	if strings.Contains(msg, apiKey) {
		t.Fatalf("error message leaked the API key: %q", msg)
	}
	if !strings.Contains(msg, RedactMarker) {
		t.Fatalf("expected redact marker in error message, got: %q", msg)
	}
}

// Silence unused-import linters across this small test file.
var _ = json.Marshal
