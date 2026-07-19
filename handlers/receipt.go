package handlers

import (
	"net/http"
	"strconv"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/gin-gonic/gin"
)

func RenderReceipt(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/orders")
		return
	}

	var order models.Order
	if err := config.DB.Where("id = ? AND shop_id = ?", id, shopID).
		Preload("OrderItems").
		Preload("User").
		Preload("Shop").
		First(&order).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Order not found"})
		return
	}

	c.HTML(http.StatusOK, "receipt/receipt.html", gin.H{
		"order": order,
		"shop":  order.Shop,
	})
}
