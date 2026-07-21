package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/dreamoutbox/go-pos/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CartItemInput struct {
	ProductID uint `json:"product_id" validate:"required"`
	Quantity  int  `json:"quantity" validate:"required,gt=0"`
}

type CheckoutInput struct {
	Items []CartItemInput `json:"items" validate:"required,min=1"`
}

func ListOrders(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	var orders []models.Order
	if err := config.DB.Where("shop_id = ?", shopID).Preload("User").Order("created_at DESC").Find(&orders).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "order/list.html", gin.H{
		"orders": orders,
		"user":   c.MustGet("user"),
		"shop":   c.MustGet("shop"),
	})
}

func NewOrderPage(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	var products []models.Product
	if err := config.DB.Where("shop_id = ? AND stock > 0", shopID).Find(&products).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "order/new.html", gin.H{
		"products": products,
		"user":     c.MustGet("user"),
		"shop":     c.MustGet("shop"),
	})
}

func CreateOrder(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	userID := c.MustGet("userID").(uint)

	var input CheckoutInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Fetch current Shop config for Tax parameters
	var shop models.Shop
	if err := config.DB.First(&shop, shopID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch shop settings"})
		return
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var orderItems []models.OrderItem
	var calculatedSubtotal float64
	var calculatedTotal float64

	// Process each cart item inside a transaction
	for _, item := range input.Items {
		var product models.Product
		// Lock product row for update to prevent race conditions during parallel checkouts
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ? AND shop_id = ?", item.ProductID, shopID).First(&product).Error; err != nil {
			tx.Rollback()
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "Product not found: ID " + strconv.Itoa(int(item.ProductID))})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database lock error"})
			}
			return
		}

		if product.Stock < item.Quantity {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient stock for product: " + product.Name})
			return
		}

		// Deduct stock
		product.Stock -= item.Quantity
		if err := tx.Save(&product).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deduct product stock"})
			return
		}

		// Write stock audit history
		history := models.StockHistory{
			ProductID: product.ID,
			UserID:    userID,
			Quantity:  -item.Quantity,
			Type:      "sale",
			Note:      "Order Checkout",
		}
		if err := tx.Create(&history).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write stock audit log"})
			return
		}

		itemPrice := product.Price
		itemSubtotal := itemPrice * float64(item.Quantity)

		orderItems = append(orderItems, models.OrderItem{
			ProductID: product.ID,
			Name:      product.Name,
			Price:     itemPrice,
			Cost:      product.Cost,
			Quantity:  item.Quantity,
			Subtotal:  itemSubtotal,
		})

		calculatedTotal += itemSubtotal
	}

	var taxAmount float64
	var subtotal float64

	if shop.TaxEnabled {
		if shop.TaxIncluded {
			// Total already includes VAT 7%
			// BasePrice = Total / 1.07
			// VAT = Total - BasePrice
			calculatedSubtotal = calculatedTotal / (1.0 + (shop.TaxRate / 100.0))
			taxAmount = calculatedTotal - calculatedSubtotal
			subtotal = calculatedSubtotal
		} else {
			// VAT is added on top
			subtotal = calculatedTotal
			taxAmount = subtotal * (shop.TaxRate / 100.0)
			calculatedTotal = subtotal + taxAmount
		}
	} else {
		subtotal = calculatedTotal
		taxAmount = 0.0
	}

	orderCode, err := utils.GenerateDocumentCode(tx, shopID, "ORD", time.Now())
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate order running number"})
		return
	}

	order := models.Order{
		Code:       orderCode,
		ShopID:     shopID,
		UserID:     userID,
		Status:     "pending",
		Subtotal:   subtotal,
		TaxAmount:  taxAmount,
		Total:      calculatedTotal,
		OrderItems: orderItems,
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order record"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Order created successfully",
		"order_id": order.ID,
	})
}

func ShowOrder(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/orders")
		return
	}

	var order models.Order
	if err := config.DB.Where("id = ? AND shop_id = ?", id, shopID).
		Preload("OrderItems.Product", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).
		Preload("User").
		First(&order).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Order not found"})
		return
	}

	var refunds []models.Refund
	config.DB.Where("order_id = ?", order.ID).Preload("User").Find(&refunds)

	var creditNotes []models.CreditNote
	config.DB.Where("order_id = ?", order.ID).Preload("User").Find(&creditNotes)

	var debitNotes []models.DebitNote
	config.DB.Where("order_id = ?", order.ID).Preload("User").Find(&debitNotes)

	c.HTML(http.StatusOK, "order/detail.html", gin.H{
		"order":       order,
		"refunds":     refunds,
		"creditNotes": creditNotes,
		"debitNotes":  debitNotes,
		"user":        c.MustGet("user"),
		"shop":        c.MustGet("shop"),
	})
}

func PayOrder(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var order models.Order
	if err := config.DB.Where("id = ? AND shop_id = ?", id, shopID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	if order.Status == "paid" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order is already paid"})
		return
	}

	now := time.Now()
	order.Status = "paid"
	order.PaidAt = &now

	if err := config.DB.Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark order as paid"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order paid successfully"})
}
