package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/models"
	"github.com/spiritlhl/goban/internal/rules"
)

type keywordRuleRequest struct {
	Name          string `json:"name"`
	Pattern       string `json:"pattern" binding:"required"`
	MatchType     string `json:"match_type"`
	MatchLogic    string `json:"match_logic"`
	CaseSensitive bool   `json:"case_sensitive"`
	Enabled       *bool  `json:"enabled"`
	Description   string `json:"description"`
}

const (
	maxKeywordRuleName        = 80
	maxKeywordRulePattern     = 1000
	maxKeywordRuleDescription = 500
	maxKeywordPreviewText     = 8000
)

func ListKeywordRules(c *gin.Context) {
	var rows []models.KeywordRule
	if err := database.GetDB().Order("created_at DESC").Find(&rows).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "获取关键字规则失败")
		return
	}
	respondOK(c, rows)
}

func CreateKeywordRule(c *gin.Context) {
	var req keywordRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	req.Pattern = strings.TrimSpace(req.Pattern)
	req.MatchType = normalizedRuleType(req.MatchType)
	req.MatchLogic = normalizedRuleLogic(req.MatchLogic)
	if err := validateKeywordRuleInput(req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := rules.Validate(req.Pattern, req.MatchType, req.CaseSensitive, req.MatchLogic); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	row := models.KeywordRule{
		Name:          strings.TrimSpace(req.Name),
		Pattern:       req.Pattern,
		MatchType:     req.MatchType,
		MatchLogic:    req.MatchLogic,
		CaseSensitive: req.CaseSensitive,
		Enabled:       enabled,
		Description:   strings.TrimSpace(req.Description),
	}
	if row.Name == "" {
		row.Name = row.Pattern
	}
	if err := database.GetDB().Create(&row).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "创建关键字规则失败: "+err.Error())
		return
	}
	respondCreated(c, "创建成功", gin.H{"message": "创建成功", "rule": row})
}

func UpdateKeywordRule(c *gin.Context) {
	var req keywordRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	db := database.GetDB()
	var row models.KeywordRule
	if err := db.First(&row, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "关键字规则不存在")
		return
	}

	if strings.TrimSpace(req.Pattern) != "" {
		row.Pattern = strings.TrimSpace(req.Pattern)
	}
	if strings.TrimSpace(req.MatchType) != "" {
		row.MatchType = normalizedRuleType(req.MatchType)
	}
	if strings.TrimSpace(req.MatchLogic) != "" {
		row.MatchLogic = normalizedRuleLogic(req.MatchLogic)
	}
	row.CaseSensitive = req.CaseSensitive
	if err := validateKeywordRuleInput(keywordRuleRequest{
		Name:        firstNonEmpty(req.Name, row.Name),
		Pattern:     row.Pattern,
		MatchType:   row.MatchType,
		MatchLogic:  row.MatchLogic,
		Description: req.Description,
	}); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := rules.Validate(row.Pattern, row.MatchType, row.CaseSensitive, row.MatchLogic); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) != "" {
		row.Name = strings.TrimSpace(req.Name)
	}
	if row.Name == "" {
		row.Name = row.Pattern
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	row.Description = strings.TrimSpace(req.Description)

	if err := db.Save(&row).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "更新关键字规则失败: "+err.Error())
		return
	}
	respondCreated(c, "更新成功", gin.H{"message": "更新成功", "rule": row})
}

func DeleteKeywordRule(c *gin.Context) {
	db := database.GetDB()
	var row models.KeywordRule
	if err := db.First(&row, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "关键字规则不存在")
		return
	}
	if !requireDeleteConfirmation(c, row.Name, strconv.FormatUint(uint64(row.ID), 10)) {
		return
	}
	if err := db.Delete(&row).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "删除关键字规则失败: "+err.Error())
		return
	}
	var remaining int64
	if err := db.Model(&models.KeywordRule{}).Where("id = ?", row.ID).Count(&remaining).Error; err != nil || remaining != 0 {
		respondError(c, http.StatusInternalServerError, "删除结果校验失败")
		return
	}
	respondCreated(c, "删除成功", gin.H{"message": "删除成功", "deleted_id": row.ID})
}

func PreviewKeywordRules(c *gin.Context) {
	var req struct {
		Text          string `json:"text"`
		Name          string `json:"name"`
		Pattern       string `json:"pattern"`
		MatchType     string `json:"match_type"`
		MatchLogic    string `json:"match_logic"`
		CaseSensitive bool   `json:"case_sensitive"`
		UseEnabled    bool   `json:"use_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	if runeLen(strings.TrimSpace(req.Text)) > maxKeywordPreviewText {
		respondError(c, http.StatusBadRequest, "预览文本过长")
		return
	}

	var compiled []rules.CompiledRule
	var compileErrors []error
	if strings.TrimSpace(req.Pattern) != "" {
		if runeLen(strings.TrimSpace(req.Pattern)) > maxKeywordRulePattern {
			respondError(c, http.StatusBadRequest, "匹配内容过长")
			return
		}
		rule := models.KeywordRule{
			Name:          req.Name,
			Pattern:       req.Pattern,
			MatchType:     normalizedRuleType(req.MatchType),
			MatchLogic:    normalizedRuleLogic(req.MatchLogic),
			CaseSensitive: req.CaseSensitive,
			Enabled:       true,
		}
		one, err := rules.Compile(rule)
		if err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		compiled = append(compiled, one)
	}
	if req.UseEnabled || strings.TrimSpace(req.Pattern) == "" {
		var rows []models.KeywordRule
		if err := database.GetDB().Where("enabled = ?", true).Order("created_at ASC").Find(&rows).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "获取规则失败")
			return
		}
		more, errs := rules.CompileMany(rows, "")
		compiled = append(compiled, more...)
		compileErrors = append(compileErrors, errs...)
	}

	respondOK(c, gin.H{
		"matches":        rules.MatchAll(req.Text, compiled),
		"compile_errors": stringifyErrors(compileErrors),
	})
}

func normalizedRuleType(matchType string) string {
	if strings.EqualFold(matchType, rules.MatchTypeRegex) {
		return rules.MatchTypeRegex
	}
	return rules.MatchTypePlain
}

func validateKeywordRuleInput(req keywordRuleRequest) error {
	if runeLen(strings.TrimSpace(req.Name)) > maxKeywordRuleName {
		return fmt.Errorf("规则名称不能超过 %d 个字符", maxKeywordRuleName)
	}
	if runeLen(strings.TrimSpace(req.Pattern)) > maxKeywordRulePattern {
		return fmt.Errorf("匹配内容不能超过 %d 个字符", maxKeywordRulePattern)
	}
	if runeLen(strings.TrimSpace(req.Description)) > maxKeywordRuleDescription {
		return fmt.Errorf("备注不能超过 %d 个字符", maxKeywordRuleDescription)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizedRuleLogic(matchLogic string) string {
	switch strings.ToLower(strings.TrimSpace(matchLogic)) {
	case rules.MatchLogicAny:
		return rules.MatchLogicAny
	case rules.MatchLogicAll:
		return rules.MatchLogicAll
	default:
		return rules.MatchLogicSingle
	}
}

func stringifyErrors(errs []error) []string {
	values := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			values = append(values, err.Error())
		}
	}
	return values
}
