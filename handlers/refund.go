package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/dreamoutbox/go-pos/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RefundItemInput struct {
	OrderItemID   uint `json:"order_item_id" form:"order_item_id"`
	Quantity      int  `json:"quantity" form:"quantity"`
	ReturnToStock bool `json:"return_to_stock" form:"return_to_stock"`
}

type CreateRefundInput struct {
	Reason string            `json:"reason" form:"reason"`
	Items  []RefundItemInput `json:"items" form:"items"`
}

func ListRefunds(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	var refunds []models.Refund
	if err := config.DB.Where("shop_id = ?", shopID).
		Preload("Order").
		Preload("User").
		Order("created_at DESC").
		Find(&refunds).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "refund/list.html", gin.H{
		"refunds": refunds,
		"user":    c.MustGet("user"),
		"shop":    c.MustGet("shop"),
	})
}

func NewRefundForm(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	orderIDStr := c.Param("id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/orders")
		return
	}

	var order models.Order
	if err := config.DB.Where("id = ? AND shop_id = ?", orderID, shopID).
		Preload("OrderItems.Product").
		First(&order).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Order not found"})
		return
	}

	if order.Status != "paid" && order.Status != "partially_refunded" {
		c.HTML(http.StatusBadRequest, "error/400.html", gin.H{"error": "Order must be paid before issuing a refund"})
		return
	}

	// Calculate already refunded quantities for each OrderItem
	refundedQtyMap := make(map[uint]int)
	var existingRefundItems []models.RefundItem
	config.DB.Joins("JOIN refunds ON refunds.id = refund_items.refund_id").
		Where("refunds.order_id = ?", order.ID).
		Find(&existingRefundItems)

	for _, item := range existingRefundItems {
		refundedQtyMap[item.OrderItemID] += item.Quantity
	}

	c.HTML(http.StatusOK, "refund/form.html", gin.H{
		"order":          order,
		"refundedQtyMap": refundedQtyMap,
		"user":           c.MustGet("user"),
		"shop":           c.MustGet("shop"),
	})
}

func CreateRefund(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	userID := c.MustGet("userID").(uint)
	orderIDStr := c.Param("id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var input CreateRefundInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return
	}

	if len(input.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one item must be selected for refund"})
		return
	}

	var shop models.Shop
	if err := config.DB.First(&shop, shopID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load shop settings"})
		return
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var order models.Order
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND shop_id = ?", orderID, shopID).
		Preload("OrderItems").
		First(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Calculate already refunded quantities
	refundedQtyMap := make(map[uint]int)
	var existingRefundItems []models.RefundItem
	tx.Joins("JOIN refunds ON refunds.id = refund_items.refund_id").
		Where("refunds.order_id = ?", order.ID).
		Find(&existingRefundItems)

	for _, item := range existingRefundItems {
		refundedQtyMap[item.OrderItemID] += item.Quantity
	}

	orderItemMap := make(map[uint]models.OrderItem)
	for _, item := range order.OrderItems {
		orderItemMap[item.ID] = item
	}

	var refundItems []models.RefundItem
	var calculatedTotal float64

	for _, itemInput := range input.Items {
		if itemInput.Quantity <= 0 {
			continue
		}

		orderItem, exists := orderItemMap[itemInput.OrderItemID]
		if !exists {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order item ID"})
			return
		}

		alreadyRefunded := refundedQtyMap[orderItem.ID]
		remainingQty := orderItem.Quantity - alreadyRefunded

		if itemInput.Quantity > remainingQty {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Cannot refund %d of '%s'. Only %d remaining.", itemInput.Quantity, orderItem.Name, remainingQty),
			})
			return
		}

		itemSubtotal := orderItem.Price * float64(itemInput.Quantity)
		calculatedTotal += itemSubtotal

		refundItems = append(refundItems, models.RefundItem{
			OrderItemID:   orderItem.ID,
			ProductID:     orderItem.ProductID,
			Name:          orderItem.Name,
			Price:         orderItem.Price,
			Quantity:      itemInput.Quantity,
			Subtotal:      itemSubtotal,
			ReturnToStock: itemInput.ReturnToStock,
		})

		// Return to inventory stock if toggled
		if itemInput.ReturnToStock {
			if err := tx.Model(&models.Product{}).
				Where("id = ?", orderItem.ProductID).
				Update("stock", gorm.Expr("stock + ?", itemInput.Quantity)).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to return product to stock"})
				return
			}

			// Write stock audit log
			history := models.StockHistory{
				ProductID: orderItem.ProductID,
				UserID:    userID,
				Quantity:  itemInput.Quantity,
				Type:      "adjustment",
				Note:      fmt.Sprintf("Refund Return (Order #%s)", order.Code),
			}
			if err := tx.Create(&history).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record stock audit log"})
				return
			}
		}
	}

	if len(refundItems) == 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid items to refund"})
		return
	}

	// Calculate tax amounts
	var taxAmount float64
	var subtotal float64
	if shop.TaxEnabled {
		if shop.TaxIncluded {
			subtotal = calculatedTotal / (1.0 + (shop.TaxRate / 100.0))
			taxAmount = calculatedTotal - subtotal
		} else {
			subtotal = calculatedTotal
			taxAmount = subtotal * (shop.TaxRate / 100.0)
			calculatedTotal = subtotal + taxAmount
		}
	} else {
		subtotal = calculatedTotal
		taxAmount = 0.0
	}

	refundCode, err := utils.GenerateDocumentCode(tx, shopID, "RFD", time.Now())
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate RFD running number"})
		return
	}

	refund := models.Refund{
		Code:        refundCode,
		ShopID:      shopID,
		OrderID:     order.ID,
		UserID:      userID,
		Subtotal:    subtotal,
		TaxAmount:   taxAmount,
		Total:       calculatedTotal,
		Reason:      input.Reason,
		RefundItems: refundItems,
	}

	if err := tx.Create(&refund).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save refund record"})
		return
	}

	// Check if all items in order are fully refunded
	allFullyRefunded := true
	for _, item := range order.OrderItems {
		totalRefunded := refundedQtyMap[item.ID]
		for _, rItem := range refundItems {
			if rItem.OrderItemID == item.ID {
				totalRefunded += rItem.Quantity
			}
		}
		if totalRefunded < item.Quantity {
			allFullyRefunded = false
			break
		}
	}

	newStatus := "partially_refunded"
	if allFullyRefunded {
		newStatus = "refunded"
	}

	if err := tx.Model(&order).Update("status", newStatus).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Refund processed successfully",
		"refund_id": refund.ID,
		"code":      refund.Code,
	})
}

func ShowRefund(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/refunds")
		return
	}

	var refund models.Refund
	if err := config.DB.Where("id = ? AND shop_id = ?", id, shopID).
		Preload("Order").
		Preload("User").
		Preload("Shop").
		Preload("RefundItems.Product").
		Preload("RefundItems.OrderItem").
		First(&refund).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Refund record not found"})
		return
	}

	c.HTML(http.StatusOK, "refund/detail.html", gin.H{
		"refund": refund,
		"user":   c.MustGet("user"),
		"shop":   c.MustGet("shop"),
	})
}
