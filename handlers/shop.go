package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/dreamoutbox/go-pos/utils"
	"github.com/gin-gonic/gin"
)

type ShopFormInput struct {
	Name    string `form:"name" json:"name" validate:"required"`
	Address string `form:"address" json:"address"`
	Phone   string `form:"phone" json:"phone"`
}

type CreateShopInput struct {
	Name          string `form:"name" json:"name" validate:"required"`
	Address       string `form:"address" json:"address"`
	Phone         string `form:"phone" json:"phone"`
	AdminEmail    string `form:"admin_email" json:"admin_email" validate:"required,email"`
	AdminPassword string `form:"admin_password" json:"admin_password" validate:"required,min=4"`
	AdminName     string `form:"admin_name" json:"admin_name" validate:"required,min=2"`
}

func ListShops(c *gin.Context) {
	var shops []models.Shop
	if err := config.DB.Find(&shops).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "shop/list.html", gin.H{
		"shops":             shops,
		"hideViewingBanner": true,
		"user":              c.MustGet("user"),
		"shop":              c.MustGet("shop"),
	})
}

func NewShopForm(c *gin.Context) {
	c.HTML(http.StatusOK, "shop/form.html", gin.H{
		"isEdit": false,
		"user":   c.MustGet("user"),
		"shop":   c.MustGet("shop"),
	})
}

func CreateShop(c *gin.Context) {
	var input CreateShopInput
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "shop/form.html", gin.H{
			"error":  "Invalid form input.",
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	if err := utils.Validate.Struct(input); err != nil {
		errors := utils.FormatValidationError(err)
		c.HTML(http.StatusUnprocessableEntity, "shop/form.html", gin.H{
			"errors": errors,
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	// Verify email not in use
	var existingUser models.User
	if err := config.DB.Where("email = ?", input.AdminEmail).First(&existingUser).Error; err == nil {
		c.HTML(http.StatusConflict, "shop/form.html", gin.H{
			"error":  "Admin email is already in use by another user account.",
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	newShop := models.Shop{
		Name:        input.Name,
		Address:     input.Address,
		Phone:       input.Phone,
		TaxEnabled:  false,
		TaxIncluded: true,
		TaxRate:     7.0,
	}

	if err := tx.Create(&newShop).Error; err != nil {
		tx.Rollback()
		c.HTML(http.StatusInternalServerError, "shop/form.html", gin.H{
			"error":  "Failed to create shop record: " + err.Error(),
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	newAdmin := models.User{
		ShopID: newShop.ID,
		Email:  input.AdminEmail,
		Name:   input.AdminName,
		Role:   "shop_owner",
	}
	if err := newAdmin.SetPassword(input.AdminPassword); err != nil {
		tx.Rollback()
		c.HTML(http.StatusInternalServerError, "shop/form.html", gin.H{
			"error":  "Failed to encrypt password",
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	if err := tx.Create(&newAdmin).Error; err != nil {
		tx.Rollback()
		c.HTML(http.StatusInternalServerError, "shop/form.html", gin.H{
			"error":  "Failed to create admin user: " + err.Error(),
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	tx.Commit()
	c.Redirect(http.StatusSeeOther, "/shops")
}

func EditShopForm(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/shops")
		return
	}

	var targetShop models.Shop
	if err := config.DB.First(&targetShop, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Shop not found"})
		return
	}

	c.HTML(http.StatusOK, "shop/settings.html", gin.H{
		"targetShop": targetShop,
		"formAction": fmt.Sprintf("/shops/%d", targetShop.ID),
		"pageTitle":  fmt.Sprintf("Edit: %s", targetShop.Name),
		"backURL":    "/shops",
		"user":       c.MustGet("user"),
		"shop":       c.MustGet("shop"),
	})
}

func UpdateShop(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/shops")
		return
	}

	var targetShop models.Shop
	if err := config.DB.First(&targetShop, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Shop not found"})
		return
	}

	var input ShopFormInput
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "shop/form.html", gin.H{
			"error":      "Invalid form input.",
			"isEdit":     true,
			"targetShop": targetShop,
			"user":       c.MustGet("user"),
			"shop":       c.MustGet("shop"),
		})
		return
	}

	if err := utils.Validate.Struct(input); err != nil {
		errors := utils.FormatValidationError(err)
		c.HTML(http.StatusUnprocessableEntity, "shop/form.html", gin.H{
			"errors":     errors,
			"input":      input,
			"isEdit":     true,
			"targetShop": targetShop,
			"user":       c.MustGet("user"),
			"shop":       c.MustGet("shop"),
		})
		return
	}

	targetShop.Name = input.Name
	targetShop.Address = input.Address
	targetShop.Phone = input.Phone

	if err := config.DB.Save(&targetShop).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "shop/settings.html", gin.H{
			"error":      "Failed to update shop details: " + err.Error(),
			"formAction": fmt.Sprintf("/shops/%d", targetShop.ID),
			"pageTitle":  fmt.Sprintf("Edit: %s", targetShop.Name),
			"backURL":    "/shops",
			"targetShop": targetShop,
			"user":       c.MustGet("user"),
			"shop":       c.MustGet("shop"),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/shops")
}

// SwitchShop lets the superuser switch their active shop context without re-logging in.
func SwitchShop(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/shops")
		return
	}

	var targetShop models.Shop
	if err := config.DB.First(&targetShop, id).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Shop not found"})
		return
	}

	if tokenVal, exists := c.Get("sessionToken"); exists {
		config.DB.Model(&models.Session{}).Where("token = ?", tokenVal.(string)).Update("active_shop_id", uint(id))
	}

	c.Redirect(http.StatusSeeOther, "/")
}

// MyShopSettingsForm shows the shop owner a form to edit their own shop's basic info.
func MyShopSettingsForm(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	var targetShop models.Shop
	if err := config.DB.First(&targetShop, shopID).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": "Failed to load shop"})
		return
	}
	c.HTML(http.StatusOK, "shop/settings.html", gin.H{
		"targetShop": targetShop,
		"msg":        c.Query("msg"),
		"taxMsg":     c.Query("taxMsg"),
		"user":       c.MustGet("user"),
		"shop":       targetShop,
	})
}

// UpdateMyShopSettings saves name/phone/address for the current user's own shop.
func UpdateMyShopSettings(c *gin.Context) {
	shop := c.MustGet("shop").(models.Shop)

	var input ShopFormInput
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "shop/settings.html", gin.H{
			"error":      "Invalid form input.",
			"targetShop": shop,
			"user":       c.MustGet("user"),
			"shop":       shop,
		})
		return
	}

	if err := utils.Validate.Struct(input); err != nil {
		errors := utils.FormatValidationError(err)
		c.HTML(http.StatusUnprocessableEntity, "shop/settings.html", gin.H{
			"errors":     errors,
			"input":      input,
			"targetShop": shop,
			"user":       c.MustGet("user"),
			"shop":       shop,
		})
		return
	}

	shop.Name = input.Name
	shop.Address = input.Address
	shop.Phone = input.Phone

	if err := config.DB.Save(&shop).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "shop/settings.html", gin.H{
			"error":      "Failed to update shop details: " + err.Error(),
			"targetShop": shop,
			"user":       c.MustGet("user"),
			"shop":       shop,
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/shop/settings?msg=Shop+info+updated+successfully")
}
