package handlers

import (
	"net/http"
	"time"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/middleware"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/dreamoutbox/go-pos/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type LoginInput struct {
	Email    string `form:"email" json:"email" validate:"required,email"`
	Password string `form:"password" json:"password" validate:"required"`
}

func ShowLoginPage(c *gin.Context) {
	// If already logged in, redirect to dashboard
	if _, err := c.Cookie("token"); err == nil {
		c.Redirect(http.StatusSeeOther, "/")
		return
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

	// Create JWT token
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &middleware.Claims{
		UserID:   user.ID,
		ShopID:   user.ShopID,
		Role:     user.Role,
		UserName: user.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(config.AppConfig.JWTSecret)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "auth/login.html", gin.H{
			"error": "Failed to generate token.",
			"input": input,
		})
		return
	}

	// Set HTTP-only cookie
	c.SetCookie(
		"token",
		tokenString,
		86400, // 24 hours in seconds
		"/",
		"",
		false, // secure (set to true in production if HTTPS)
		true,  // httpOnly
	)

	c.Redirect(http.StatusSeeOther, "/")
}

func Logout(c *gin.Context) {
	// Clear the token cookie
	c.SetCookie(
		"token",
		"",
		-1,
		"/",
		"",
		false,
		true,
	)
	c.Redirect(http.StatusSeeOther, "/login")
}
