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

type DebitNoteItemInput struct {
	ProductID   *uint   `json:"product_id" form:"product_id"`
	Name        string  `json:"name" form:"name"`
	Price       float64 `json:"price" form:"price"`
	Quantity    int     `json:"quantity" form:"quantity"`
	DeductStock bool    `json:"deduct_stock" form:"deduct_stock"`
}

type CreateDebitNoteInput struct {
	Reason string               `json:"reason" form:"reason"`
	Items  []DebitNoteItemInput `json:"items" form:"items"`
}

func ListDebitNotes(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	var debitNotes []models.DebitNote
	if err := config.DB.Where("shop_id = ?", shopID).
		Preload("Order").
		Preload("User").
		Order("created_at DESC").
		Find(&debitNotes).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "debit_note/list.html", gin.H{
		"debitNotes": debitNotes,
		"user":       c.MustGet("user"),
		"shop":       c.MustGet("shop"),
	})
}

func NewDebitNoteForm(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	orderIDStr := c.Param("id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/orders")
		return
	}

	var order models.Order
	if err := config.DB.Where("id = ? AND shop_id = ?", orderID, shopID).
		Preload("OrderItems").
		First(&order).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Order not found"})
		return
	}

	var products []models.Product
	config.DB.Unscoped().Where("shop_id = ?", shopID).Find(&products)

	c.HTML(http.StatusOK, "debit_note/form.html", gin.H{
		"order":    order,
		"products": products,
		"user":     c.MustGet("user"),
		"shop":     c.MustGet("shop"),
	})
}

func CreateDebitNote(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	userID := c.MustGet("userID").(uint)
	orderIDStr := c.Param("id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var input CreateDebitNoteInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input format"})
		return
	}

	if len(input.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least one item or adjustment fee must be added to Debit Note"})
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
		First(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	var dbnItems []models.DebitNoteItem
	var calculatedTotal float64

	for _, itemInput := range input.Items {
		if itemInput.Quantity <= 0 || itemInput.Price < 0 {
			continue
		}

		itemName := itemInput.Name
		var prodID *uint = itemInput.ProductID

		if prodID != nil && *prodID > 0 {
			var product models.Product
			if err := tx.First(&product, *prodID).Error; err == nil {
				if itemName == "" {
					itemName = product.Name
				}
				if itemInput.DeductStock {
					if product.Stock < itemInput.Quantity {
						tx.Rollback()
						c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Insufficient stock for '%s'", product.Name)})
						return
					}
					// Deduct stock
					if err := tx.Model(&product).Update("stock", gorm.Expr("stock - ?", itemInput.Quantity)).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deduct product stock"})
						return
					}

					history := models.StockHistory{
						ProductID: product.ID,
						UserID:    userID,
						Quantity:  -itemInput.Quantity,
						Type:      "sale",
						Note:      fmt.Sprintf("Debit Note Charge (Order #%s)", order.Code),
					}
					if err := tx.Create(&history).Error; err != nil {
						tx.Rollback()
						c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record stock audit log"})
						return
					}
				}
			}
		}

		if itemName == "" {
			itemName = "Debit Adjustment Fee"
		}

		itemSubtotal := itemInput.Price * float64(itemInput.Quantity)
		calculatedTotal += itemSubtotal

		dbnItems = append(dbnItems, models.DebitNoteItem{
			ProductID:   prodID,
			Name:        itemName,
			Price:       itemInput.Price,
			Quantity:    itemInput.Quantity,
			Subtotal:    itemSubtotal,
			DeductStock: itemInput.DeductStock,
		})
	}

	if len(dbnItems) == 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid items added for Debit Note"})
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

	dbnCode, err := utils.GenerateDocumentCode(tx, shopID, "DBN", time.Now())
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate DBN running number"})
		return
	}

	debitNote := models.DebitNote{
		Code:           dbnCode,
		ShopID:         shopID,
		OrderID:        order.ID,
		UserID:         userID,
		Subtotal:       subtotal,
		TaxAmount:      taxAmount,
		Total:          calculatedTotal,
		Reason:         input.Reason,
		DebitNoteItems: dbnItems,
	}

	if err := tx.Create(&debitNote).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save debit note record"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Debit Note issued successfully",
		"debit_note_id": debitNote.ID,
		"code":          debitNote.Code,
	})
}

func ShowDebitNote(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/debit_notes")
		return
	}

	var debitNote models.DebitNote
	if err := config.DB.Where("id = ? AND shop_id = ?", id, shopID).
		Preload("Order").
		Preload("User").
		Preload("Shop").
		Preload("DebitNoteItems.Product", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).
		First(&debitNote).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Debit Note record not found"})
		return
	}

	c.HTML(http.StatusOK, "debit_note/detail.html", gin.H{
		"debitNote": debitNote,
		"user":      c.MustGet("user"),
		"shop":      c.MustGet("shop"),
	})
}

func PrintDebitNote(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/debit_notes")
		return
	}

	var debitNote models.DebitNote
	if err := config.DB.Where("id = ? AND shop_id = ?", id, shopID).
		Preload("Order").
		Preload("User").
		Preload("Shop").
		Preload("DebitNoteItems.Product", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).
		First(&debitNote).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Debit Note record not found"})
		return
	}

	c.HTML(http.StatusOK, "debit_note/print.html", gin.H{
		"debitNote": debitNote,
		"shop":      debitNote.Shop,
	})
}
