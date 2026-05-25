package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func registerInfra(r *gin.Engine, api *gin.RouterGroup, d Deps) {
	r.GET("/.well-known/oauth-authorization-server", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"issuer":                                "",
			"authorization_endpoint":                "",
			"token_endpoint":                        "",
			"response_types_supported":              []string{},
			"token_endpoint_auth_methods_supported": []string{},
		})
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"type":   "api-server",
		})
	})
	r.HEAD("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	if d.Mode == "debug" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not Found",
			"message": "The requested resource was not found. In separated architecture, static files are served by Nginx.",
		})
	})
}
