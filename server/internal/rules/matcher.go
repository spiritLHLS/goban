package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spiritlhl/goban/internal/models"
)

const (
	MatchTypePlain = "plain"
	MatchTypeRegex = "regex"
)

type CompiledRule struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Pattern       string `json:"pattern"`
	MatchType     string `json:"match_type"`
	CaseSensitive bool   `json:"case_sensitive"`
	regex         *regexp.Regexp
}

type MatchResult struct {
	RuleID    uint   `json:"rule_id"`
	RuleName  string `json:"rule_name"`
	Pattern   string `json:"pattern"`
	MatchType string `json:"match_type"`
	Matched   string `json:"matched"`
}

func Validate(pattern, matchType string, caseSensitive bool) error {
	if strings.TrimSpace(pattern) == "" {
		return fmt.Errorf("匹配内容不能为空")
	}
	if normalizeMatchType(matchType) == MatchTypeRegex {
		_, err := regexp.Compile(regexPattern(pattern, caseSensitive))
		if err != nil {
			return fmt.Errorf("正则表达式无效: %w", err)
		}
	}
	return nil
}

func Compile(rule models.KeywordRule) (CompiledRule, error) {
	compiled := CompiledRule{
		ID:            rule.ID,
		Name:          rule.Name,
		Pattern:       strings.TrimSpace(rule.Pattern),
		MatchType:     normalizeMatchType(rule.MatchType),
		CaseSensitive: rule.CaseSensitive,
	}
	if compiled.Name == "" {
		compiled.Name = compiled.Pattern
	}
	if err := Validate(compiled.Pattern, compiled.MatchType, compiled.CaseSensitive); err != nil {
		return CompiledRule{}, err
	}
	if compiled.MatchType == MatchTypeRegex {
		re, err := regexp.Compile(regexPattern(compiled.Pattern, compiled.CaseSensitive))
		if err != nil {
			return CompiledRule{}, err
		}
		compiled.regex = re
	}
	return compiled, nil
}

func CompileMany(rows []models.KeywordRule, adHocKeywords string) ([]CompiledRule, []error) {
	compiled := make([]CompiledRule, 0, len(rows))
	var errs []error

	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		rule, err := Compile(row)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", row.Name, err))
			continue
		}
		compiled = append(compiled, rule)
	}

	for _, keyword := range ParseAdHocKeywords(adHocKeywords) {
		compiled = append(compiled, CompiledRule{
			Name:      keyword,
			Pattern:   keyword,
			MatchType: MatchTypePlain,
		})
	}

	return compiled, errs
}

func MatchText(text string, compiled []CompiledRule) *MatchResult {
	for _, rule := range compiled {
		if matched := rule.Match(text); matched != "" {
			return &MatchResult{
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				Pattern:   rule.Pattern,
				MatchType: rule.MatchType,
				Matched:   matched,
			}
		}
	}
	return nil
}

func MatchAll(text string, compiled []CompiledRule) []MatchResult {
	matches := make([]MatchResult, 0)
	for _, rule := range compiled {
		if matched := rule.Match(text); matched != "" {
			matches = append(matches, MatchResult{
				RuleID:    rule.ID,
				RuleName:  rule.Name,
				Pattern:   rule.Pattern,
				MatchType: rule.MatchType,
				Matched:   matched,
			})
		}
	}
	return matches
}

func (r CompiledRule) Match(text string) string {
	if r.MatchType == MatchTypeRegex {
		if r.regex == nil {
			return ""
		}
		return r.regex.FindString(text)
	}

	source := text
	pattern := r.Pattern
	if !r.CaseSensitive {
		source = strings.ToLower(source)
		pattern = strings.ToLower(pattern)
	}
	if strings.Contains(source, pattern) {
		return r.Pattern
	}
	return ""
}

func ParseRuleIDs(raw string) []uint {
	ids := make([]uint, 0)
	seen := map[uint]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id64, err := strconv.ParseUint(part, 10, 64)
		if err != nil || id64 == 0 {
			continue
		}
		id := uint(id64)
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids
}

func FormatRuleIDs(ids []uint) string {
	parts := make([]string, 0, len(ids))
	seen := map[uint]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
		seen[id] = true
	}
	return strings.Join(parts, ",")
}

func ParseAdHocKeywords(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ';'
	})
	keywords := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		keyword := strings.TrimSpace(field)
		if keyword == "" || seen[keyword] {
			continue
		}
		keywords = append(keywords, keyword)
		seen[keyword] = true
	}
	return keywords
}

func MarkMatchedAt(rule *models.KeywordRule, now time.Time) {
	if rule != nil && rule.ID > 0 {
		rule.LastMatchedAt = &now
	}
}

func normalizeMatchType(matchType string) string {
	if strings.EqualFold(matchType, MatchTypeRegex) {
		return MatchTypeRegex
	}
	return MatchTypePlain
}

func regexPattern(pattern string, caseSensitive bool) string {
	if caseSensitive {
		return pattern
	}
	return "(?i)" + pattern
}
