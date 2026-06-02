package controllers

import (
	"net/http"
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
	CaseSensitive bool   `json:"case_sensitive"`
	Enabled       *bool  `json:"enabled"`
	Description   string `json:"description"`
}

func ListKeywordRules(c *gin.Context) {
	var rows []models.KeywordRule
	if err := database.GetDB().Order("created_at DESC").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取关键字规则失败"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func CreateKeywordRule(c *gin.Context) {
	var req keywordRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	req.Pattern = strings.TrimSpace(req.Pattern)
	req.MatchType = normalizedRuleType(req.MatchType)
	if err := rules.Validate(req.Pattern, req.MatchType, req.CaseSensitive); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
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
		CaseSensitive: req.CaseSensitive,
		Enabled:       enabled,
		Description:   strings.TrimSpace(req.Description),
	}
	if row.Name == "" {
		row.Name = row.Pattern
	}
	if err := database.GetDB().Create(&row).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "创建关键字规则失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "创建成功", "rule": row})
}

func UpdateKeywordRule(c *gin.Context) {
	var req keywordRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	db := database.GetDB()
	var row models.KeywordRule
	if err := db.First(&row, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "关键字规则不存在"})
		return
	}

	if strings.TrimSpace(req.Pattern) != "" {
		row.Pattern = strings.TrimSpace(req.Pattern)
	}
	if strings.TrimSpace(req.MatchType) != "" {
		row.MatchType = normalizedRuleType(req.MatchType)
	}
	row.CaseSensitive = req.CaseSensitive
	if err := rules.Validate(row.Pattern, row.MatchType, row.CaseSensitive); err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusOK, gin.H{"error": "更新关键字规则失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "rule": row})
}

func DeleteKeywordRule(c *gin.Context) {
	db := database.GetDB()
	var row models.KeywordRule
	if err := db.First(&row, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "关键字规则不存在"})
		return
	}
	if err := db.Delete(&row).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "删除关键字规则失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func PreviewKeywordRules(c *gin.Context) {
	var req struct {
		Text          string `json:"text"`
		Name          string `json:"name"`
		Pattern       string `json:"pattern"`
		MatchType     string `json:"match_type"`
		CaseSensitive bool   `json:"case_sensitive"`
		UseEnabled    bool   `json:"use_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var compiled []rules.CompiledRule
	var compileErrors []error
	if strings.TrimSpace(req.Pattern) != "" {
		rule := models.KeywordRule{
			Name:          req.Name,
			Pattern:       req.Pattern,
			MatchType:     normalizedRuleType(req.MatchType),
			CaseSensitive: req.CaseSensitive,
			Enabled:       true,
		}
		one, err := rules.Compile(rule)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"error": err.Error()})
			return
		}
		compiled = append(compiled, one)
	}
	if req.UseEnabled || strings.TrimSpace(req.Pattern) == "" {
		var rows []models.KeywordRule
		if err := database.GetDB().Where("enabled = ?", true).Order("created_at ASC").Find(&rows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取规则失败"})
			return
		}
		more, errs := rules.CompileMany(rows, "")
		compiled = append(compiled, more...)
		compileErrors = append(compileErrors, errs...)
	}

	c.JSON(http.StatusOK, gin.H{
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

func stringifyErrors(errs []error) []string {
	values := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			values = append(values, err.Error())
		}
	}
	return values
}
