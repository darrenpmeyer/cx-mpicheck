// SPDX-License-Identifier: AGPL-3.0-only
package mpicheck

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// RiskPolicy controls which risks cause a non-zero exit.
type RiskPolicy struct {
	ScoreRules    []ScoreRule
	TitleMatchers []string
}

// ScoreRule matches numeric scores.
type ScoreRule struct {
	Min          *float64
	Max          *float64
	MinInclusive bool
	MaxInclusive bool
}

// IsDefault returns true when the policy does not filter risks.
func (p RiskPolicy) IsDefault() bool {
	return len(p.ScoreRules) == 0 && len(p.TitleMatchers) == 0
}

// BuildRiskPolicy parses score expressions and normalizes title matchers.
func BuildRiskPolicy(scoreExprs, titleMatchers []string) (RiskPolicy, error) {
	policy := RiskPolicy{}
	for _, expr := range scoreExprs {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		rule, err := ParseScoreRule(expr)
		if err != nil {
			return RiskPolicy{}, err
		}
		policy.ScoreRules = append(policy.ScoreRules, rule)
	}
	for _, title := range titleMatchers {
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		policy.TitleMatchers = append(policy.TitleMatchers, strings.ToLower(title))
	}
	return policy, nil
}

// ParseScoreRule parses expressions like ">5", "5-9", "<=10", or "10".
func ParseScoreRule(expr string) (ScoreRule, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ScoreRule{}, fmt.Errorf("empty score rule")
	}

	if strings.Contains(expr, "-") || strings.Contains(expr, "..") {
		rangeExpr := strings.ReplaceAll(expr, "..", "-")
		parts := strings.SplitN(rangeExpr, "-", 2)
		if len(parts) != 2 {
			return ScoreRule{}, fmt.Errorf("invalid score range: %s", expr)
		}
		min, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			return ScoreRule{}, fmt.Errorf("invalid score range: %s", expr)
		}
		max, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return ScoreRule{}, fmt.Errorf("invalid score range: %s", expr)
		}
		return ScoreRule{Min: &min, Max: &max, MinInclusive: true, MaxInclusive: true}, nil
	}

	operators := []string{">=", "<=", ">", "<", "==", "="}
	for _, op := range operators {
		if strings.HasPrefix(expr, op) {
			valStr := strings.TrimSpace(strings.TrimPrefix(expr, op))
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				return ScoreRule{}, fmt.Errorf("invalid score rule: %s", expr)
			}
			switch op {
			case ">=":
				return ScoreRule{Min: &val, MinInclusive: true}, nil
			case ">":
				return ScoreRule{Min: &val, MinInclusive: false}, nil
			case "<=":
				return ScoreRule{Max: &val, MaxInclusive: true}, nil
			case "<":
				return ScoreRule{Max: &val, MaxInclusive: false}, nil
			case "==", "=":
				return ScoreRule{Min: &val, Max: &val, MinInclusive: true, MaxInclusive: true}, nil
			}
		}
	}

	val, err := strconv.ParseFloat(expr, 64)
	if err != nil {
		return ScoreRule{}, fmt.Errorf("invalid score rule: %s", expr)
	}
	return ScoreRule{Min: &val, Max: &val, MinInclusive: true, MaxInclusive: true}, nil
}

// LoadRiskFile reads a cx.mpiapi-risks.json file.
func LoadRiskFile(path string) ([]RiskRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var records []RiskRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// EvaluatePolicy counts total risks and how many match the policy.
func EvaluatePolicy(records []RiskRecord, policy RiskPolicy) (totalRisks, matchedRisks int) {
	_, totalRisks, matchedRisks, _ = FilterRiskRecords(records, policy)
	return totalRisks, matchedRisks
}

// FilterRiskRecords returns records filtered to only include matching risks.
// It also returns counts for total risks, matched risks, and matched packages.
func FilterRiskRecords(records []RiskRecord, policy RiskPolicy) (filtered []RiskRecord, totalRisks, matchedRisks, matchedPackages int) {
	filtered = make([]RiskRecord, 0, len(records))
	for _, rec := range records {
		totalRisks += len(rec.Risks)
		if policy.IsDefault() {
			matchedRisks += len(rec.Risks)
			matchedPackages++
			filtered = append(filtered, rec)
			continue
		}
		matched := make([]interface{}, 0, len(rec.Risks))
		for _, risk := range rec.Risks {
			if matchRisk(policy, risk) {
				matched = append(matched, risk)
				matchedRisks++
			}
		}
		if len(matched) > 0 {
			matchedPackages++
			rec.Risks = matched
			filtered = append(filtered, rec)
		}
	}
	return filtered, totalRisks, matchedRisks, matchedPackages
}

func matchRisk(policy RiskPolicy, risk interface{}) bool {
	asMap, ok := risk.(map[string]interface{})
	if !ok {
		return false
	}

	if len(policy.ScoreRules) > 0 {
		if score, ok := extractScore(asMap); ok {
			for _, rule := range policy.ScoreRules {
				if rule.Matches(score) {
					return true
				}
			}
		}
	}

	if len(policy.TitleMatchers) > 0 {
		if title, ok := extractTitle(asMap); ok {
			lower := strings.ToLower(title)
			for _, matcher := range policy.TitleMatchers {
				if strings.Contains(lower, matcher) {
					return true
				}
			}
		}
	}

	return false
}

// Matches returns true if the score matches the rule.
func (r ScoreRule) Matches(score float64) bool {
	if r.Min != nil {
		if r.MinInclusive {
			if score < *r.Min {
				return false
			}
		} else if score <= *r.Min {
			return false
		}
	}
	if r.Max != nil {
		if r.MaxInclusive {
			if score > *r.Max {
				return false
			}
		} else if score >= *r.Max {
			return false
		}
	}
	return true
}

func extractScore(risk map[string]interface{}) (float64, bool) {
	keys := []string{"score", "cvss", "cvssScore", "severityScore"}
	for _, key := range keys {
		if val, ok := risk[key]; ok {
			switch v := val.(type) {
			case float64:
				return v, true
			case int:
				return float64(v), true
			case int64:
				return float64(v), true
			case json.Number:
				parsed, err := v.Float64()
				if err == nil {
					return parsed, true
				}
			case string:
				parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
				if err == nil {
					return parsed, true
				}
			}
		}
	}
	return 0, false
}

func extractTitle(risk map[string]interface{}) (string, bool) {
	keys := []string{"title", "name"}
	for _, key := range keys {
		if val, ok := risk[key]; ok {
			if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
				return s, true
			}
		}
	}
	return "", false
}
