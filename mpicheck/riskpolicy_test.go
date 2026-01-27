// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import "testing"

func TestScoreRuleMatches(t *testing.T) {
	rule, err := ParseScoreRule(">5")
	if err != nil {
		t.Fatal(err)
	}
	if !rule.Matches(6) || rule.Matches(5) {
		t.Fatalf("unexpected match results")
	}

	rangeRule, err := ParseScoreRule("5-9")
	if err != nil {
		t.Fatal(err)
	}
	if !rangeRule.Matches(7) || rangeRule.Matches(10) {
		t.Fatalf("unexpected range match results")
	}

	exactRule, err := ParseScoreRule("10")
	if err != nil {
		t.Fatal(err)
	}
	if !exactRule.Matches(10) || exactRule.Matches(9) {
		t.Fatalf("unexpected exact match results")
	}
}

func TestFilterRiskRecords(t *testing.T) {
	records := []RiskRecord{
		{PURL: "pkg:npm/a@1.0.0", Risks: []interface{}{map[string]interface{}{"score": 9, "title": "RCE"}}},
		{PURL: "pkg:npm/b@1.0.0", Risks: []interface{}{map[string]interface{}{"score": 4, "title": "low"}}},
	}
	policy, err := BuildRiskPolicy([]string{">=8"}, []string{"rce"})
	if err != nil {
		t.Fatal(err)
	}
	filtered, total, matched, pkgs := FilterRiskRecords(records, policy)
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if matched != 1 {
		t.Fatalf("expected matched 1, got %d", matched)
	}
	if pkgs != 1 || len(filtered) != 1 {
		t.Fatalf("expected 1 package matched")
	}
}
