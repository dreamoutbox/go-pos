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

type CreditNoteItemInput struct {
	OrderItemID   uint `json:"order_item_id" form:"order_item_id"`
	Quantity      int  `json:"quantity" form:"quantity"`
	ReturnToStock bool `json:"return_to_stock" form:"return_to_stock"`
}

type CreateCreditNoteInput struct {
	Reason string                `json:"reason" form:"reason"`
	Items  []CreditNoteItemInput `json:"items" form:"items"`
}

func ListCreditNotes(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	var creditNotes []models.CreditNote
	if err := config.DB.Where("shop_id = ?", shopID).
		Preload("Order").
		Preload("User").
		Order("created_at DESC").
		Find(&creditNotes).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "credit_note/list.html", gin.H{
		"creditNotes": creditNotes,
		"user":        c.MustGet("user"),
		"shop":        c.MustGet("shop"),
	})
}

func NewCreditNoteForm(c *gin.Context) {
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

	c.HTML(http.StatusOK, "credit_note/form.html", gin.H{
		"order": order,
		"user":  c.MustGet("user"),
		"shop":  c.MustGet("shop"),
	})
}

func CreateCreditNote(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	userID := c.MustGet("userID").(uint)
	orderIDStr := c.Param("id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var input CreateCreditNoteInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return
	}

	if len(input.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one item must be selected for Credit Note"})
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

	orderItemMap := make(map[uint]models.OrderItem)
	for _, item := range order.OrderItems {
		orderItemMap[item.ID] = item
	}

	var cnItems []models.CreditNoteItem
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

		itemSubtotal := orderItem.Price * float64(itemInput.Quantity)
		calculatedTotal += itemSubtotal

		cnItems = append(cnItems, models.CreditNoteItem{
			OrderItemID:   orderItem.ID,
			ProductID:     orderItem.ProductID,
			Name:          orderItem.Name,
			Price:         orderItem.Price,
			Quantity:      itemInput.Quantity,
			Subtotal:      itemSubtotal,
			ReturnToStock: itemInput.ReturnToStock,
		})

		if itemInput.ReturnToStock {
			if err := tx.Model(&models.Product{}).
				Where("id = ?", orderItem.ProductID).
				Update("stock", gorm.Expr("stock + ?", itemInput.Quantity)).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to return product to stock"})
				return
			}

			history := models.StockHistory{
				ProductID: orderItem.ProductID,
				UserID:    userID,
				Quantity:  itemInput.Quantity,
				Type:      "adjustment",
				Note:      fmt.Sprintf("Credit Note Return (Order #%s)", order.Code),
			}
			if err := tx.Create(&history).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record stock audit log"})
				return
			}
		}
	}

	if len(cnItems) == 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid items selected for Credit Note"})
		return
	}

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

	cnCode, err := utils.GenerateDocumentCode(tx, shopID, "CDN", time.Now())
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate CDN running number"})
		return
	}

	creditNote := models.CreditNote{
		Code:            cnCode,
		ShopID:          shopID,
		OrderID:         order.ID,
		UserID:          userID,
		Subtotal:        subtotal,
		TaxAmount:       taxAmount,
		Total:           calculatedTotal,
		Reason:          input.Reason,
		CreditNoteItems: cnItems,
	}

	if err := tx.Create(&creditNote).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save credit note record"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Credit Note issued successfully",
		"credit_note_id": creditNote.ID,
		"code":           creditNote.Code,
	})
}

func ShowCreditNote(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/credit_notes")
		return
	}

	var creditNote models.CreditNote
	if err := config.DB.Where("id = ? AND shop_id = ?", id, shopID).
		Preload("Order").
		Preload("User").
		Preload("Shop").
		Preload("CreditNoteItems.Product").
		Preload("CreditNoteItems.OrderItem").
		First(&creditNote).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Credit Note record not found"})
		return
	}

	c.HTML(http.StatusOK, "credit_note/detail.html", gin.H{
		"creditNote": creditNote,
		"user":       c.MustGet("user"),
		"shop":       c.MustGet("shop"),
	})
}

func PrintCreditNote(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/credit_notes")
		return
	}

	var creditNote models.CreditNote
	if err := config.DB.Where("id = ? AND shop_id = ?", id, shopID).
		Preload("Order").
		Preload("User").
		Preload("Shop").
		Preload("CreditNoteItems.Product").
		Preload("CreditNoteItems.OrderItem").
		First(&creditNote).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Credit Note record not found"})
		return
	}

	c.HTML(http.StatusOK, "credit_note/print.html", gin.H{
		"creditNote": creditNote,
		"shop":       creditNote.Shop,
	})
}
