// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import "strings"

// RedactMarker is what Redact substitutes in place of each found secret.
const RedactMarker = "[REDACTED]"

// Redact returns s with every occurrence of every non-empty secret
// replaced by RedactMarker. Empty secrets are skipped so a caller can
// safely pass values that may not be set without redacting every empty
// substring of s.
//
// Matching is exact-substring and case-sensitive, applied in the order
// secrets are given. This catches verbatim leaks (e.g. an upstream
// gateway that echoes the Authorization header into the response body)
// but does NOT catch partial leaks — truncated prefixes, base64-decoded
// or re-encoded forms, or values split across log lines. Callers
// concerned about those should redact at the source instead.
func Redact(s string, secrets ...string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		s = strings.ReplaceAll(s, secret, RedactMarker)
	}
	return s
}
