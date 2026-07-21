package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dreamoutbox/go-pos/config"
	"github.com/dreamoutbox/go-pos/models"
	"github.com/dreamoutbox/go-pos/utils"
	"github.com/gin-gonic/gin"
)

type ProductFormInput struct {
	Name        string  `form:"name" json:"name" validate:"required"`
	SKU         string  `form:"sku" json:"sku"`
	Price       float64 `form:"price" json:"price" validate:"required,gt=0"`
	Cost        float64 `form:"cost" json:"cost" validate:"gte=0"`
	Description string  `form:"description" json:"description"`
}

func isJSONRequest(c *gin.Context) bool {
	return c.GetHeader("Accept") == "application/json" || c.GetHeader("X-Requested-With") == "XMLHttpRequest"
}

func ListProducts(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	showDeleted := c.Query("show_deleted") == "true"

	var products []models.Product
	query := config.DB
	if showDeleted {
		query = query.Unscoped()
	}
	if err := query.Where("shop_id = ?", shopID).Order("id desc").Find(&products).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "error/500.html", gin.H{"error": err.Error()})
		return
	}

	c.HTML(http.StatusOK, "product/list.html", gin.H{
		"products":    products,
		"showDeleted": showDeleted,
		"user":        c.MustGet("user"),
		"shop":        c.MustGet("shop"),
	})
}

func ShowProduct(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/products")
		return
	}

	var product models.Product
	if err := config.DB.Unscoped().Where("id = ? AND shop_id = ?", id, shopID).First(&product).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Product not found"})
		return
	}

	c.HTML(http.StatusOK, "product/detail.html", gin.H{
		"product": product,
		"user":    c.MustGet("user"),
		"shop":    c.MustGet("shop"),
	})
}

func NewProductForm(c *gin.Context) {
	c.HTML(http.StatusOK, "product/form.html", gin.H{
		"isEdit": false,
		"user":   c.MustGet("user"),
		"shop":   c.MustGet("shop"),
	})
}

func CreateProduct(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)

	var input ProductFormInput
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "product/form.html", gin.H{
			"error":  "Invalid input form.",
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	if err := utils.Validate.Struct(input); err != nil {
		errors := utils.FormatValidationError(err)
		c.HTML(http.StatusUnprocessableEntity, "product/form.html", gin.H{
			"errors": errors,
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	// Handle Image Upload if any
	var imagePath string
	file, err := c.FormFile("image")
	if err == nil {
		// Ensure upload directory exists
		if err := os.MkdirAll(config.AppConfig.UploadDir, 0755); err != nil {
			c.HTML(http.StatusInternalServerError, "product/form.html", gin.H{
				"error":  "Failed to create upload directory",
				"input":  input,
				"isEdit": false,
				"user":   c.MustGet("user"),
				"shop":   c.MustGet("shop"),
			})
			return
		}

		filename := fmt.Sprintf("p_%d_%d%s", shopID, time.Now().UnixNano(), filepath.Ext(file.Filename))
		savePath := filepath.Join(config.AppConfig.UploadDir, filename)
		if err := c.SaveUploadedFile(file, savePath); err != nil {
			c.HTML(http.StatusInternalServerError, "product/form.html", gin.H{
				"error":  "Failed to save uploaded image",
				"input":  input,
				"isEdit": false,
				"user":   c.MustGet("user"),
				"shop":   c.MustGet("shop"),
			})
			return
		}
		imagePath = "/uploads/" + filename
	}

	newProduct := models.Product{
		ShopID:      shopID,
		Name:        input.Name,
		SKU:         input.SKU,
		Price:       input.Price,
		Cost:        input.Cost,
		ImagePath:   imagePath,
		Description: input.Description,
		Stock:       0, // initial stock is zero, added via stock management
	}

	if err := config.DB.Create(&newProduct).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "product/form.html", gin.H{
			"error":  "Failed to create product: " + err.Error(),
			"input":  input,
			"isEdit": false,
			"user":   c.MustGet("user"),
			"shop":   c.MustGet("shop"),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/products")
}

func EditProductForm(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/products")
		return
	}

	var product models.Product
	if err := config.DB.Unscoped().Where("id = ? AND shop_id = ?", id, shopID).First(&product).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Product not found"})
		return
	}

	c.HTML(http.StatusOK, "product/form.html", gin.H{
		"isEdit":  true,
		"product": product,
		"msg":     c.Query("msg"),
		"user":    c.MustGet("user"),
		"shop":    c.MustGet("shop"),
	})
}

func UpdateProduct(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/products")
		return
	}

	var product models.Product
	if err := config.DB.Unscoped().Where("id = ? AND shop_id = ?", id, shopID).First(&product).Error; err != nil {
		c.HTML(http.StatusNotFound, "error/404.html", gin.H{"error": "Product not found"})
		return
	}

	var input ProductFormInput
	if err := c.ShouldBind(&input); err != nil {
		c.HTML(http.StatusBadRequest, "product/form.html", gin.H{
			"error":   "Invalid input form.",
			"isEdit":  true,
			"product": product,
			"user":    c.MustGet("user"),
			"shop":    c.MustGet("shop"),
		})
		return
	}

	if err := utils.Validate.Struct(input); err != nil {
		errors := utils.FormatValidationError(err)
		c.HTML(http.StatusUnprocessableEntity, "product/form.html", gin.H{
			"errors":  errors,
			"input":   input,
			"isEdit":  true,
			"product": product,
			"user":    c.MustGet("user"),
			"shop":    c.MustGet("shop"),
		})
		return
	}

	// Handle Image Upload if any
	file, err := c.FormFile("image")
	if err == nil {
		// Ensure upload directory exists
		if err := os.MkdirAll(config.AppConfig.UploadDir, 0755); err == nil {
			filename := fmt.Sprintf("p_%d_%d%s", shopID, time.Now().UnixNano(), filepath.Ext(file.Filename))
			savePath := filepath.Join(config.AppConfig.UploadDir, filename)
			if err := c.SaveUploadedFile(file, savePath); err == nil {
				// delete old image if exists
				if product.ImagePath != "" {
					oldFilename := filepath.Base(product.ImagePath)
					oldPath := filepath.Join(config.AppConfig.UploadDir, oldFilename)
					_ = os.Remove(oldPath)
				}
				product.ImagePath = "/uploads/" + filename
			}
		}
	}

	product.Name = input.Name
	product.SKU = input.SKU
	product.Price = input.Price
	product.Cost = input.Cost
	product.Description = input.Description

	if err := config.DB.Unscoped().Save(&product).Error; err != nil {
		c.HTML(http.StatusInternalServerError, "product/form.html", gin.H{
			"error":   "Failed to update product: " + err.Error(),
			"isEdit":  true,
			"product": product,
			"user":    c.MustGet("user"),
			"shop":    c.MustGet("shop"),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/products/%d", product.ID))
}

func DeleteProduct(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if isJSONRequest(c) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		} else {
			c.Redirect(http.StatusSeeOther, "/products")
		}
		return
	}

	var product models.Product
	if err := config.DB.Where("id = ? AND shop_id = ?", id, shopID).First(&product).Error; err != nil {
		if isJSONRequest(c) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		} else {
			c.Redirect(http.StatusSeeOther, "/products")
		}
		return
	}

	// Soft delete product (updates deleted_at column to NOW())
	if err := config.DB.Delete(&product).Error; err != nil {
		if isJSONRequest(c) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
		} else {
			c.Redirect(http.StatusSeeOther, "/products?error=Failed+to+delete+product")
		}
		return
	}

	if isJSONRequest(c) {
		c.JSON(http.StatusOK, gin.H{"message": "Product deleted successfully"})
	} else {
		c.Redirect(http.StatusSeeOther, "/products?msg=Product+deleted+successfully")
	}
}

func RestoreProduct(c *gin.Context) {
	shopID := c.MustGet("shopID").(uint)
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		if isJSONRequest(c) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		} else {
			c.Redirect(http.StatusSeeOther, "/products")
		}
		return
	}

	var product models.Product
	if err := config.DB.Unscoped().Where("id = ? AND shop_id = ?", id, shopID).First(&product).Error; err != nil {
		if isJSONRequest(c) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		} else {
			c.Redirect(http.StatusSeeOther, "/products")
		}
		return
	}

	// Restore product by setting deleted_at to NULL
	if err := config.DB.Unscoped().Model(&product).Update("deleted_at", nil).Error; err != nil {
		if isJSONRequest(c) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore product"})
		} else {
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/products/%d/edit?error=Failed+to+restore+product", id))
		}
		return
	}

	if isJSONRequest(c) {
		c.JSON(http.StatusOK, gin.H{"message": "Product restored successfully"})
	} else {
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/products/%d/edit?msg=Product+restored+successfully", product.ID))
	}
}
