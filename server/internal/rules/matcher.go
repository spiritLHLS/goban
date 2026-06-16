package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spiritlhl/goban/internal/models"
	"golang.org/x/text/width"
)

const (
	MatchTypePlain = "plain"
	MatchTypeRegex = "regex"

	MatchLogicSingle = "single"
	MatchLogicAny    = "or"
	MatchLogicAll    = "and"
)

type CompiledRule struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Pattern       string `json:"pattern"`
	MatchType     string `json:"match_type"`
	MatchLogic    string `json:"match_logic"`
	CaseSensitive bool   `json:"case_sensitive"`
	terms         []string
	regexes       []*regexp.Regexp
}

type MatchResult struct {
	RuleID     uint   `json:"rule_id"`
	RuleName   string `json:"rule_name"`
	Pattern    string `json:"pattern"`
	MatchType  string `json:"match_type"`
	MatchLogic string `json:"match_logic"`
	Matched    string `json:"matched"`
}

func Validate(pattern, matchType string, caseSensitive bool, matchLogicValue ...string) error {
	if strings.TrimSpace(pattern) == "" {
		return fmt.Errorf("匹配内容不能为空")
	}
	logic := MatchLogicSingle
	if len(matchLogicValue) > 0 {
		logic = normalizeMatchLogic(matchLogicValue[0])
	}
	terms := ruleTerms(pattern, logic)
	if len(terms) == 0 {
		return fmt.Errorf("匹配内容不能为空")
	}
	if normalizeMatchType(matchType) == MatchTypeRegex {
		for _, term := range terms {
			_, err := regexp.Compile(regexPattern(term, caseSensitive))
			if err != nil {
				return fmt.Errorf("正则表达式无效: %w", err)
			}
		}
	}
	if logic == MatchLogicAll && len(terms) < 2 {
		return fmt.Errorf("全部匹配至少需要两个条件")
	}
	return nil
}

func Compile(rule models.KeywordRule) (CompiledRule, error) {
	compiled := CompiledRule{
		ID:            rule.ID,
		Name:          rule.Name,
		Pattern:       strings.TrimSpace(rule.Pattern),
		MatchType:     normalizeMatchType(rule.MatchType),
		MatchLogic:    normalizeMatchLogic(rule.MatchLogic),
		CaseSensitive: rule.CaseSensitive,
	}
	if compiled.Name == "" {
		compiled.Name = compiled.Pattern
	}
	if err := Validate(compiled.Pattern, compiled.MatchType, compiled.CaseSensitive, compiled.MatchLogic); err != nil {
		return CompiledRule{}, err
	}
	compiled.terms = ruleTerms(compiled.Pattern, compiled.MatchLogic)
	if compiled.MatchType == MatchTypeRegex {
		compiled.regexes = make([]*regexp.Regexp, 0, len(compiled.terms))
		for _, term := range compiled.terms {
			re, err := regexp.Compile(regexPattern(term, compiled.CaseSensitive))
			if err != nil {
				return CompiledRule{}, err
			}
			compiled.regexes = append(compiled.regexes, re)
		}
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
			Name:       keyword,
			Pattern:    keyword,
			MatchType:  MatchTypePlain,
			MatchLogic: MatchLogicSingle,
			terms:      []string{keyword},
		})
	}

	return compiled, errs
}

func MatchText(text string, compiled []CompiledRule) *MatchResult {
	for _, rule := range compiled {
		if matched := rule.Match(text); matched != "" {
			return &MatchResult{
				RuleID:     rule.ID,
				RuleName:   rule.Name,
				Pattern:    rule.Pattern,
				MatchType:  rule.MatchType,
				MatchLogic: rule.MatchLogic,
				Matched:    matched,
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
				RuleID:     rule.ID,
				RuleName:   rule.Name,
				Pattern:    rule.Pattern,
				MatchType:  rule.MatchType,
				MatchLogic: rule.MatchLogic,
				Matched:    matched,
			})
		}
	}
	return matches
}

func (r CompiledRule) Match(text string) string {
	if r.MatchType == MatchTypeRegex {
		return r.matchRegex(text)
	}

	return r.matchPlain(text)
}

func (r CompiledRule) matchRegex(text string) string {
	if len(r.regexes) == 0 {
		return ""
	}
	matches := make([]string, 0, len(r.regexes))
	for _, re := range r.regexes {
		if re == nil {
			continue
		}
		matched := re.FindString(text)
		if matched == "" && r.MatchLogic == MatchLogicAll {
			return ""
		}
		if matched != "" {
			matches = append(matches, matched)
			if r.MatchLogic != MatchLogicAll {
				return matched
			}
		}
	}
	return strings.Join(matches, ", ")
}

func (r CompiledRule) matchPlain(text string) string {
	terms := r.terms
	if len(terms) == 0 {
		terms = ruleTerms(r.Pattern, r.MatchLogic)
	}
	source := text
	normalize := func(value string) string {
		value = width.Fold.String(value)
		if !r.CaseSensitive {
			value = strings.ToLower(value)
		}
		return value
	}
	if !r.CaseSensitive {
		source = normalize(source)
	} else {
		source = width.Fold.String(source)
	}
	matches := make([]string, 0, len(terms))
	for _, term := range terms {
		pattern := normalize(term)
		matched := pattern != "" && strings.Contains(source, pattern)
		if !matched && r.MatchLogic == MatchLogicAll {
			return ""
		}
		if matched {
			matches = append(matches, term)
			if r.MatchLogic != MatchLogicAll {
				return term
			}
		}
	}
	return strings.Join(matches, ", ")
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

func normalizeMatchLogic(matchLogic string) string {
	switch strings.ToLower(strings.TrimSpace(matchLogic)) {
	case MatchLogicAny:
		return MatchLogicAny
	case MatchLogicAll:
		return MatchLogicAll
	default:
		return MatchLogicSingle
	}
}

func regexPattern(pattern string, caseSensitive bool) string {
	if caseSensitive {
		return pattern
	}
	return "(?i)" + pattern
}

func ruleTerms(pattern, matchLogic string) []string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}
	if normalizeMatchLogic(matchLogic) == MatchLogicSingle {
		return []string{pattern}
	}
	fields := strings.FieldsFunc(pattern, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == ';' || r == '，' || r == '；'
	})
	terms := make([]string, 0, len(fields))
	seen := map[string]bool{}
	for _, field := range fields {
		term := strings.TrimSpace(field)
		if term == "" || seen[term] {
			continue
		}
		terms = append(terms, term)
		seen[term] = true
	}
	return terms
}
