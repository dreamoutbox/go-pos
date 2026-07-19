package handlers

import (
	"net/http"
	"time"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/gin-gonic/gin"
)

func Dashboard(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	// Fetch today's orders count and revenue
	todayStart := time.Now().Truncate(24 * time.Hour)
	var orders []models.Order
	if err := config.DB.Where("shop_id = ? AND status = ? AND created_at >= ?", shopID, "paid", todayStart).Find(&orders).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	var revenue float64
	for _, order := range orders {
		revenue += order.Total
	}

	// Fetch low stock items
	var lowStockProducts []models.Product
	if err := config.DB.Where("shop_id = ? AND stock < ?", shopID, 10).Find(&lowStockProducts).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	// Fetch quick-sell products (top products or just general list)
	var quickProducts []models.Product
	if err := config.DB.Where("shop_id = ? AND stock > ?", shopID, 0).Limit(8).Find(&quickProducts).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "dashboard/index.html", gin.H{
		"salesCount": len(orders),
		"revenue":    revenue,
		"lowStock":   lowStockProducts,
		"products":   quickProducts,
		"user":       c.MustGet("user"),
		"shop":       c.MustGet("shop"),
	})
}
