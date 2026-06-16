package controllers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type deleteConfirmationRequest struct {
	ConfirmID   any `json:"confirm_id"`
	ConfirmText any `json:"confirm_text"`
}

func requireDeleteConfirmation(c *gin.Context, expectedValues ...string) bool {
	id := strings.TrimSpace(c.Param("id"))
	confirmID := strings.TrimSpace(c.Query("confirm_id"))
	confirmText := strings.TrimSpace(c.Query("confirm_text"))
	if confirmID == "" || confirmText == "" {
		var req deleteConfirmationRequest
		if err := c.ShouldBindJSON(&req); err == nil {
			if confirmID == "" {
				confirmID = confirmationString(req.ConfirmID)
			}
			if confirmText == "" {
				confirmText = confirmationString(req.ConfirmText)
			}
		}
	}
	if confirmID != id {
		respondError(c, http.StatusPreconditionRequired, "删除确认失败：confirm_id 必须与路径 ID 一致")
		return false
	}
	if confirmText == "" {
		respondError(c, http.StatusPreconditionRequired, "删除确认失败：缺少 confirm_text")
		return false
	}
	for _, value := range append(expectedValues, id, "DELETE") {
		if strings.TrimSpace(value) != "" && confirmText == strings.TrimSpace(value) {
			return true
		}
	}
	respondError(c, http.StatusPreconditionRequired, fmt.Sprintf("删除确认失败：请输入 %s 或 DELETE", firstExpectedValue(expectedValues, id)))
	return false
}

func confirmationString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func firstExpectedValue(values []string, fallback string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return fallback
}
