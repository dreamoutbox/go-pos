package handlers

import (
	"net/http"
	"strconv"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/gin-gonic/gin"
)

type AddStockInput struct {
	Quantity int    `form:"quantity" json:"quantity" validate:"required,gt=0"`
	Note     string `form:"note" json:"note"`
}

type EditStockInput struct {
	Stock int    `json:"stock" validate:"gte=0"`
	Note  string `json:"note"`
}

func StockList(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	var products []models.Product
	if err := config.DB.Where("shop_id = ?", shopID).Find(&products).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "stock/list.html", gin.H{
		"products": products,
		"user":     c.MustGet("user"),
		"shop":     c.MustGet("shop"),
	})
}

func AddStockForm(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	productIDStr := c.Param("productID")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/stock")
		return
	}

	var product models.Product
	if err := config.DB.Where("id = ? AND shop_id = ?", productID, shopID).First(&product).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Product not found"})
		return
	}

	c.HTML(http.StatusOK, "stock/add.html", gin.H{
		"product": product,
		"user":    c.MustGet("user"),
		"shop":    c.MustGet("shop"),
	})
}

func AddStock(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	userID := c.MustGet("userID").(uint)
	productIDStr := c.Param("productID")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/stock")
		return
	}

	var product models.Product
	if err := config.DB.Where("id = ? AND shop_id = ?", productID, shopID).First(&product).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Product not found"})
		return
	}

	var input AddStockInput
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "stock/add.html", gin.H{
			"error":   "Invalid input.",
			"product": product,
			"user":    c.MustGet("user"),
			"shop":    c.MustGet("shop"),
		})
		return
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Lock the product row for update
	if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&product, product.ID).Error; err != nil {
		tx.Rollback()
		c.HTML(http.StatusInternalServerError, "stock/add.html", gin.H{
			"error":   "Failed to lock product details for update.",
			"product": product,
			"user":    c.MustGet("user"),
			"shop":    c.MustGet("shop"),
		})
		return
	}

	product.Stock += input.Quantity
	if err := tx.Save(&product).Error; err != nil {
		tx.Rollback()
		c.HTML(http.StatusInternalServerError, "stock/add.html", gin.H{
			"error":   "Failed to update stock: " + err.Error(),
			"product": product,
			"user":    c.MustGet("user"),
			"shop":    c.MustGet("shop"),
		})
		return
	}

	history := models.StockHistory{
		ProductID: product.ID,
		UserID:    userID,
		Quantity:  input.Quantity,
		Type:      "add",
		Note:      input.Note,
	}
	if err := tx.Create(&history).Error; err != nil {
		tx.Rollback()
		c.HTML(http.StatusInternalServerError, "stock/add.html", gin.H{
			"error":   "Failed to write stock audit history.",
			"product": product,
			"user":    c.MustGet("user"),
			"shop":    c.MustGet("shop"),
		})
		return
	}

	tx.Commit()
	c.Redirect(http.StatusSeeOther, "/stock")
}

func EditStock(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	userID := c.MustGet("userID").(uint)
	productIDStr := c.Param("productID")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}

	var input EditStockInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON input"})
		return
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var product models.Product
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ? AND shop_id = ?", productID, shopID).First(&product).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	delta := input.Stock - product.Stock
	product.Stock = input.Stock

	if err := tx.Save(&product).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update stock"})
		return
	}

	history := models.StockHistory{
		ProductID: product.ID,
		UserID:    userID,
		Quantity:  delta,
		Type:      "adjustment",
		Note:      input.Note,
	}
	if err := tx.Create(&history).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write audit history"})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{
		"message":   "Stock updated successfully",
		"new_stock": product.Stock,
	})
}

func StockHistory(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	var histories []models.StockHistory
	// Join with products table to filter by shop_id and preload Product & User
	err := config.DB.
		Joins("JOIN products ON stock_histories.product_id = products.id").
		Where("products.shop_id = ?", shopID).
		Preload("Product").
		Preload("User").
		Order("stock_histories.created_at DESC").
		Find(&histories).Error

	if err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "stock/history.html", gin.H{
		"histories": histories,
		"user":      c.MustGet("user"),
		"shop":      c.MustGet("shop"),
	})
}
