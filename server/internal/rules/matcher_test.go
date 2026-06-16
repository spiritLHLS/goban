package rules

import (
	"testing"

	"github.com/spiritlhl/goban/internal/models"
)

func TestPlainMatchHandlesChineseEmojiAndCase(t *testing.T) {
	compiled, err := Compile(models.KeywordRule{
		Name:      "unicode",
		Pattern:   "这个😊视频",
		MatchType: MatchTypePlain,
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if got := compiled.Match("我觉得这个😊视频很好"); got != "这个😊视频" {
		t.Fatalf("expected unicode match, got %q", got)
	}

	compiled, err = Compile(models.KeywordRule{
		Name:      "case",
		Pattern:   "Video",
		MatchType: MatchTypePlain,
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	if got := compiled.Match("这个video真好"); got != "Video" {
		t.Fatalf("expected case-insensitive match, got %q", got)
	}
	if got := compiled.Match("这个ｖｉｄｅｏ真好"); got != "Video" {
		t.Fatalf("expected full-width ASCII match, got %q", got)
	}
}

func TestRegexMatchAndValidation(t *testing.T) {
	compiled, err := Compile(models.KeywordRule{
		Name:      "wechat",
		Pattern:   `V信\d+`,
		MatchType: MatchTypeRegex,
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if got := compiled.Match("广告+V信123"); got != "V信123" {
		t.Fatalf("expected regex match, got %q", got)
	}

	if err := Validate("(", MatchTypeRegex, false); err == nil {
		t.Fatal("expected invalid regex to fail validation")
	}
}

func TestAdHocKeywordParsingDeduplicates(t *testing.T) {
	got := ParseAdHocKeywords("广告, 广告\n引流;诈骗")
	want := []string{"广告", "引流", "诈骗"}
	if len(got) != len(want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %#v, got %#v", want, got)
		}
	}
}

func TestPlainAnyAndAllMatchLogic(t *testing.T) {
	anyRule, err := Compile(models.KeywordRule{
		Name:       "any",
		Pattern:    "广告,引流",
		MatchType:  MatchTypePlain,
		MatchLogic: MatchLogicAny,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("Compile any failed: %v", err)
	}
	if got := anyRule.Match("这里有广告"); got != "广告" {
		t.Fatalf("expected any match, got %q", got)
	}

	allRule, err := Compile(models.KeywordRule{
		Name:       "all",
		Pattern:    "广告;V信",
		MatchType:  MatchTypePlain,
		MatchLogic: MatchLogicAll,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("Compile all failed: %v", err)
	}
	if got := allRule.Match("广告加V信123"); got != "广告, V信" {
		t.Fatalf("expected all terms, got %q", got)
	}
	if got := allRule.Match("只有广告"); got != "" {
		t.Fatalf("expected all match to require every term, got %q", got)
	}
}

func TestRegexAllMatchLogic(t *testing.T) {
	compiled, err := Compile(models.KeywordRule{
		Name:       "regex-all",
		Pattern:    `广告,\d{3}`,
		MatchType:  MatchTypeRegex,
		MatchLogic: MatchLogicAll,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	if got := compiled.Match("广告联系123"); got != "广告, 123" {
		t.Fatalf("expected regex all match, got %q", got)
	}
	if got := compiled.Match("广告无号码"); got != "" {
		t.Fatalf("expected regex all to require every expression, got %q", got)
	}
}

func TestAllMatchRequiresMultipleTerms(t *testing.T) {
	if err := Validate("广告", MatchTypePlain, false, MatchLogicAll); err == nil {
		t.Fatal("expected all match with one term to fail")
	}
}
