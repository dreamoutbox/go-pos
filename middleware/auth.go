package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   uint   `json:"user_id"`
	ShopID   uint   `json:"shop_id"`
	Role     string `json:"role"`
	UserName string `json:"user_name"`
	jwt.RegisteredClaims
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("token")
		if err != nil {
			respondUnauthorized(c)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return config.AppConfig.JWTSecret, nil
		})

		if err != nil || !token.Valid {
			respondUnauthorized(c)
			return
		}

		// Inject claims into context
		c.Set("userID", claims.UserID)
		c.Set("shopID", claims.ShopID)
		c.Set("role", claims.Role)
		c.Set("userName", claims.UserName)

		// Fetch and inject User & Shop into context for easy access
		var user models.User
		if err := config.DB.Preload("Shop").First(&user, claims.UserID).Error; err != nil {
			respondUnauthorized(c)
			return
		}
		c.Set("user", user)
		c.Set("shop", user.Shop)

		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role")
		if !exists || roleVal.(string) != "admin" {
			if isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Admin access required"})
			} else {
				c.HTML(http.StatusForbidden, "error/403.html", gin.H{
					"error": "Forbidden: Admin access required",
				})
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

func isAPIRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.Request.URL.Path, "/api/") || c.GetHeader("Accept") == "application/json"
}

func respondUnauthorized(c *gin.Context) {
	if isAPIRequest(c) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Please log in"})
	} else {
		c.Redirect(http.StatusSeeOther, "/login")
	}
	c.Abort()
}
