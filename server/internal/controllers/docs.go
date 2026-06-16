package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/docs"
)

func GetAPIDocs(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(docs.SwaggerHTML))
}

func GetOpenAPISpec(c *gin.Context) {
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.OpenAPIJSON))
}
