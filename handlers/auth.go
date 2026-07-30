package handlers

import (
	"net/http"
	"time"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/dreamoutbox/go-pos/utils"
	"github.com/gin-gonic/gin"
)

type LoginInput struct {
	Email    string `form:"email" json:"email" validate:"required,email"`
	Password string `form:"password" json:"password" validate:"required"`
}

func ShowLoginPage(c *gin.Context) {
	// If already logged in with a valid session, redirect to dashboard
	if tokenStr, err := c.Cookie("session_token"); err == nil && tokenStr != "" {
		var session models.Session
		if err := config.DB.Where("token = ? AND expires_at > ?", tokenStr, time.Now()).First(&session).Error; err == nil {
			c.Redirect(http.StatusSeeOther, "/")
			return
		}
	}
	c.HTML(http.StatusOK, "auth/login.html", gin.H{})
}

func Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "auth/login.html", gin.H{
			"error": "Invalid input format.",
		})
		return
	}

	// Validate input
	if err := utils.Validate.Struct(input); err != nil {
		errors := utils.FormatValidationError(err)
		c.HTML(http.StatusUnprocessableEntity, "auth/login.html", gin.H{
			"errors": errors,
			"input":  input,
		})
		return
	}

	var user models.User
	if err := config.DB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		c.HTML(http.StatusUnauthorized, "auth/login.html", gin.H{
			"error": "Invalid email or password.",
			"input": input,
		})
		return
	}

	if !user.CheckPassword(input.Password) {
		c.HTML(http.StatusUnauthorized, "auth/login.html", gin.H{
			"error": "Invalid email or password.",
			"input": input,
		})
		return
	}

	// Generate session token
	tokenString, err := utils.GenerateSessionToken()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "auth/login.html", gin.H{
			"error": "Failed to create user session.",
			"input": input,
		})
		return
	}

	// Create session record in database (valid for 7 days)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	session := models.Session{
		Token:        tokenString,
		UserID:       user.ID,
		ActiveShopID: user.ShopID,
		ExpiresAt:    expiresAt,
	}

	if err := config.DB.Create(&session).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "auth/login.html", gin.H{
			"error": "Failed to persist user session.",
			"input": input,
		})
		return
	}

	// Set HTTP-only session cookie
	c.SetCookie(
		"session_token",
		tokenString,
		86400*7, // 7 days in seconds
		"/",
		"",
		false, // secure (set to true in production if HTTPS)
		true,  // httpOnly
	)

	c.Redirect(http.StatusSeeOther, "/")
}

func Logout(c *gin.Context) {
	if tokenStr, err := c.Cookie("session_token"); err == nil && tokenStr != "" {
		config.DB.Where("token = ?", tokenStr).Delete(&models.Session{})
	}

	// Clear the session_token cookie
	c.SetCookie(
		"session_token",
		"",
		-1,
		"/",
		"",
		false,
		true,
	)
	c.Redirect(http.StatusSeeOther, "/login")
}
