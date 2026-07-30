package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/gin-gonic/gin"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("session_token")
		if err != nil || tokenStr == "" {
			// Fallback check for legacy "token" cookie
			tokenStr, err = c.Cookie("token")
			if err != nil || tokenStr == "" {
				respondUnauthorized(c)
				return
			}
		}

		var session models.Session
		if err := config.DB.Preload("User.Shop").
			Where("token = ? AND expires_at > ?", tokenStr, time.Now()).
			First(&session).Error; err != nil {
			respondUnauthorized(c)
			return
		}

		user := session.User

		// Fetch active shop by session.ActiveShopID — may differ from user.ShopID when superuser switches
		var activeShop models.Shop
		if err := config.DB.First(&activeShop, session.ActiveShopID).Error; err != nil {
			respondUnauthorized(c)
			return
		}

		// Inject user and shop info into context
		c.Set("userID", user.ID)
		c.Set("shopID", activeShop.ID)
		c.Set("role", user.Role)
		c.Set("userName", user.Name)
		c.Set("user", user)
		c.Set("shop", activeShop)
		c.Set("sessionToken", session.Token)

		c.Next()
	}
}

// SuperuserRequired allows only the superuser role (global shop management).
func SuperuserRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role")
		if !exists || roleVal.(string) != "superuser" {
			if isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Superuser access required"})
			} else {
				c.HTML(http.StatusForbidden, "error/403.html", gin.H{
					"error": "Forbidden: Superuser access required",
				})
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

// ShopOwnerRequired allows superuser and shop_owner roles.
func ShopOwnerRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role")
		if !exists {
			c.Abort()
			return
		}
		role := roleVal.(string)
		if role != "superuser" && role != "shop_owner" {
			if isAPIRequest(c) {
				c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Shop owner or superuser access required"})
			} else {
				c.HTML(http.StatusForbidden, "error/403.html", gin.H{
					"error": "Forbidden: Shop owner or superuser access required",
				})
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

// AdminRequired is kept as an alias for ShopOwnerRequired for backward compatibility.
func AdminRequired() gin.HandlerFunc {
	return ShopOwnerRequired()
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
