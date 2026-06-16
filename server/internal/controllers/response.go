package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const apiSuccessCode = 0

type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func respondOK(c *gin.Context, data interface{}) {
	respondMessage(c, http.StatusOK, "ok", data)
}

func respondCreated(c *gin.Context, message string, data interface{}) {
	respondMessage(c, http.StatusOK, message, data)
}

func respondMessage(c *gin.Context, status int, message string, data interface{}) {
	if message == "" {
		message = "ok"
	}
	c.JSON(status, apiResponse{
		Code:    apiSuccessCode,
		Message: message,
		Data:    data,
	})
}

func respondError(c *gin.Context, status int, message string) {
	if status < 400 {
		status = http.StatusInternalServerError
	}
	c.AbortWithStatusJSON(status, apiResponse{
		Code:    status,
		Message: message,
		Error:   message,
	})
}
