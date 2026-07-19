package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func MethodOverride() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost {
			// Check form parameter
			method := c.PostForm("_method")
			if method == "" {
				// Also check query parameter
				method = c.Query("_method")
			}

			if method != "" {
				method = strings.ToUpper(method)
				if method == "PATCH" || method == "PUT" || method == "DELETE" {
					c.Request.Method = method
				}
			}
		}
		c.Next()
	}
}
