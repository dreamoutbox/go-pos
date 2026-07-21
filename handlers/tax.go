package handlers

import (
	"net/http"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/dreamoutbox/go-pos/utils"
	"github.com/gin-gonic/gin"
)

type TaxSettingsInput struct {
	TaxEnabled  bool    `form:"tax_enabled" json:"tax_enabled"`
	TaxIncluded bool    `form:"tax_included" json:"tax_included"`
	TaxName     string  `form:"tax_name" json:"tax_name"`
	TaxAddress  string  `form:"tax_address" json:"tax_address"`
	TaxID       string  `form:"tax_id" json:"tax_id"`
	TaxRate     float64 `form:"tax_rate" json:"tax_rate" validate:"required,gt=0"`
}

func TaxSettingsForm(c *gin.Context) {
	c.HTML(http.StatusOK, "tax/settings.html", gin.H{
		"user": c.MustGet("user"),
		"shop": c.MustGet("shop"),
	})
}

func UpdateTaxSettings(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	// Fetch shop record to update
	var shop models.Shop
	if err := config.DB.First(&shop, shopID).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": "Failed to fetch shop details"})
		return
	}

	var input TaxSettingsInput
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "tax/settings.html", gin.H{
			"error": "Invalid form input format.",
			"user":  c.MustGet("user"),
			"shop":  shop,
		})
		return
	}

	// We set default values if tax_rate is missing, or validate
	if input.TaxRate <= 0 {
		input.TaxRate = 7.0
	}

	if err := utils.Validate.Struct(input); err != nil {
		errors := utils.FormatValidationError(err)
		c.HTML(http.StatusUnprocessableEntity, "tax/settings.html", gin.H{
			"errors": errors,
			"user":   c.MustGet("user"),
			"shop":   shop,
		})
		return
	}

	// Since checkbox forms don't send values when unchecked, check gin Context's PostForm values
	// or rely on ShouldBind handling default false values properly. ShouldBind handles boolean checkboxes
	// if they are named correctly and the input uses 'form:"tax_enabled"'.
	// But in standard form submissions:
	// - If checkbox is unchecked, it is not sent. ShouldBind keeps GORM's struct field as false.
	// - If it is checked, it sends "on" or "true", ShouldBind parses as true.
	// Let's manually double check checkbox bindings or use standard form posts where we map them:
	shop.TaxEnabled = c.PostForm("tax_enabled") == "true" || c.PostForm("tax_enabled") == "on"
	shop.TaxIncluded = c.PostForm("tax_included") == "true" || c.PostForm("tax_included") == "on"
	shop.TaxName = input.TaxName
	shop.TaxAddress = input.TaxAddress
	shop.TaxID = input.TaxID
	shop.TaxRate = input.TaxRate

	if err := config.DB.Save(&shop).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "tax/settings.html", gin.H{
			"error": "Failed to save settings: " + err.Error(),
			"user":  c.MustGet("user"),
			"shop":  shop,
		})
		return
	}

	// Update shop context for current request template rendering
	c.Set("shop", shop)

	c.Redirect(http.StatusSeeOther, "/shop/settings?taxMsg=VAT+settings+updated+successfully")
}
