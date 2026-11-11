package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// cors mengizinkan frontend web (HTML/JS di domain lain) memanggil API dari browser.
// CORS_ALLOWED_ORIGINS: "*" (default) atau daftar dipisah koma, mis.
// "https://preeat.netlify.app,http://localhost:5500". Login memakai header
// Authorization (bukan cookie), jadi kredensial browser tidak dikirim.
func cors() gin.HandlerFunc {
	allowed := map[string]bool{}
	all := true
	if v := strings.TrimSpace(getenv("CORS_ALLOWED_ORIGINS", "*")); v != "*" {
		all = false
		for _, o := range strings.Split(v, ",") {
			allowed[strings.TrimRight(strings.TrimSpace(o), "/")] = true
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (all || allowed[origin]) {
			h := c.Writer.Header()
			if all {
				h.Set("Access-Control-Allow-Origin", "*")
			} else {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
			}
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			h.Set("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
