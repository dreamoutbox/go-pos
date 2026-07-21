package handlers

import (
	"net/http"
	"strconv"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/dreamoutbox/go-pos/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateUserInput struct {
	Name     string `form:"name" json:"name" validate:"required,min=2"`
	Email    string `form:"email" json:"email" validate:"required,email"`
	Password string `form:"password" json:"password" validate:"required,min=4"`
	Role     string `form:"role" json:"role" validate:"required,oneof=shop_owner cashier"`
	ShopID   uint   `form:"shop_id" json:"shop_id"`
}

type UpdateUserInput struct {
	Name   string `form:"name" json:"name" validate:"required,min=2"`
	Email  string `form:"email" json:"email" validate:"required,email"`
	Role   string `form:"role" json:"role" validate:"required,oneof=shop_owner cashier"`
	ShopID uint   `form:"shop_id" json:"shop_id"`
}

type ChangePasswordInput struct {
	Password string `form:"password" json:"password" validate:"required,min=4"`
}

func ListUsers(c *gin.Context) {
	role := c.MustGet("role").(string)
	shopID := c.MustGet("shopID").(uint)

	var users []models.User
	var query *gorm.DB
	if role == "superuser" {
		// Superuser sees all users across all shops
		query = config.DB.Preload("Shop").Find(&users)
	} else {
		// Shop owner sees only users in their shop
		query = config.DB.Where("shop_id = ?", shopID).Find(&users)
	}
	if err := query.Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	var shops []models.Shop
	if role == "superuser" {
		config.DB.Find(&shops)
	}

	c.HTML(http.StatusOK, "user/list.html", gin.H{
		"users":             users,
		"shops":             shops,
		"hideViewingBanner": true,
		"user":              c.MustGet("user"),
		"shop":              c.MustGet("shop"),
	})
}

func NewUserForm(c *gin.Context) {
	role := c.MustGet("role").(string)
	var shops []models.Shop
	if role == "superuser" {
		config.DB.Find(&shops)
	}
	c.HTML(http.StatusOK, "user/form.html", gin.H{
		"isEdit": false,
		"shops":  shops,
		"user":   c.MustGet("user"),
		"shop":   c.MustGet("shop"),
	})
}

func CreateUser(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	var input CreateUserInput
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "user/form.html", gin.H{
			"error":  "Invalid form input",
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	if err := utils.Validate.Struct(input); err != nil {
		errors := utils.FormatValidationError(err)
		c.HTML(http.StatusUnprocessableEntity, "user/form.html", gin.H{
			"errors": errors,
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	// Check if email already registered
	var existing models.User
	if err := config.DB.Where("email = ?", input.Email).First(&existing).Error; err == nil {
		c.HTML(http.StatusConflict, "user/form.html", gin.H{
			"error":  "Email is already in use.",
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	// Determine which shop to assign
	assignedShopID := shopID
	if c.MustGet("role").(string) == "superuser" && input.ShopID > 0 {
		assignedShopID = input.ShopID
	}

	newUser := models.User{
		ShopID: assignedShopID,
		Name:   input.Name,
		Email:  input.Email,
		Role:   input.Role,
	}
	if err := newUser.SetPassword(input.Password); err != nil {
		c.HTML(http.StatusInternalServerError, "user/form.html", gin.H{
			"error":  "Failed to process password",
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	if err := config.DB.Create(&newUser).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "user/form.html", gin.H{
			"error":  "Failed to create user: " + err.Error(),
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/users")
}

func EditUserForm(c *gin.Context) {
	role := c.MustGet("role").(string)
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/users")
		return
	}

	var targetUser models.User
	var dbErr error
	if role == "superuser" {
		dbErr = config.DB.First(&targetUser, id).Error
	} else {
		dbErr = config.DB.Where("id = ? AND shop_id = ?", id, shopID).First(&targetUser).Error
	}
	if dbErr != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "User not found"})
		return
	}

	var shops []models.Shop
	if role == "superuser" {
		config.DB.Find(&shops)
	}

	c.HTML(http.StatusOK, "user/form.html", gin.H{
		"isEdit":     true,
		"targetUser": targetUser,
		"shops":      shops,
		"user":       c.MustGet("user"),
		"shop":       c.MustGet("shop"),
	})
}

func UpdateUser(c *gin.Context) {
	role := c.MustGet("role").(string)
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/users")
		return
	}

	var targetUser models.User
	var dbErr error
	if role == "superuser" {
		dbErr = config.DB.First(&targetUser, id).Error
	} else {
		dbErr = config.DB.Where("id = ? AND shop_id = ?", id, shopID).First(&targetUser).Error
	}
	if dbErr != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "User not found"})
		return
	}

	// Load shops for re-render on error
	var shops []models.Shop
	if role == "superuser" {
		config.DB.Find(&shops)
	}

	var input UpdateUserInput
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "user/form.html", gin.H{
			"error":      "Invalid form input",
			"isEdit":     true,
			"targetUser": targetUser,
			"shops":      shops,
			"user":       c.MustGet("user"),
			"shop":       c.MustGet("shop"),
		})
		return
	}

	if err := utils.Validate.Struct(input); err != nil {
		errors := utils.FormatValidationError(err)
		c.HTML(http.StatusUnprocessableEntity, "user/form.html", gin.H{
			"errors":     errors,
			"input":      input,
			"isEdit":     true,
			"targetUser": targetUser,
			"shops":      shops,
			"user":       c.MustGet("user"),
			"shop":       c.MustGet("shop"),
		})
		return
	}

	// Update fields
	targetUser.Name = input.Name
	targetUser.Email = input.Email
	targetUser.Role = input.Role
	if role == "superuser" && input.ShopID > 0 {
		targetUser.ShopID = input.ShopID
	}

	if err := config.DB.Save(&targetUser).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "user/form.html", gin.H{
			"error":      "Failed to update user: " + err.Error(),
			"input":      input,
			"isEdit":     true,
			"targetUser": targetUser,
			"shops":      shops,
			"user":       c.MustGet("user"),
			"shop":       c.MustGet("shop"),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/users")
}

func ChangePassword(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/users")
		return
	}

	var targetUser models.User
	if err := config.DB.Where("id = ? AND shop_id = ?", id, shopID).First(&targetUser).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "User not found"})
		return
	}

	var input ChangePasswordInput
	if err := c.ShouldBind(&input); err != nil {
		c.Redirect(http.StatusSeeOther, "/users")
		return
	}

	if err := utils.Validate.Struct(input); err != nil {
		c.Redirect(http.StatusSeeOther, "/users?error=Password+must+be+at+least+4+characters")
		return
	}

	if err := targetUser.SetPassword(input.Password); err != nil {
		c.Redirect(http.StatusSeeOther, "/users?error=Failed+to+update+password")
		return
	}

	if err := config.DB.Save(&targetUser).Error; err != nil {
		c.Redirect(http.StatusSeeOther, "/users?error=Failed+to+save+password")
		return
	}

	c.Redirect(http.StatusSeeOther, "/users?msg=Password+updated+successfully")
}
