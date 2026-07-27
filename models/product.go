package models

import (
	"gorm.io/gorm"
)

type Product struct {
	gorm.Model
	ShopID      uint      `gorm:"not null;index" json:"shop_id"`
	Shop        Shop      `json:"shop" gorm:"foreignKey:ShopID"`
	CategoryID  *uint     `gorm:"index" json:"category_id"`
	Category    *Category `json:"category" gorm:"foreignKey:CategoryID"`
	Name        string    `gorm:"not null" json:"name" validate:"required"`
	SKU         string    `gorm:"index" json:"sku"`
	Price       float64   `gorm:"not null" json:"price" validate:"required,gt=0"`
	Cost        float64   `gorm:"default:0" json:"cost" validate:"gte=0"`
	ImagePath   string    `json:"image_path"`
	Description string    `json:"description"`
	Stock       int       `gorm:"default:0" json:"stock"` // current stock quantity
}

func (p Product) GetCategoryID() uint {
	if p.CategoryID != nil {
		return *p.CategoryID
	}
	return 0
}

type StockHistory struct {
	gorm.Model
	ProductID uint    `gorm:"not null;index" json:"product_id"`
	Product   Product `json:"product" gorm:"foreignKey:ProductID"`
	UserID    uint    `gorm:"not null" json:"user_id"`
	User      User    `json:"user" gorm:"foreignKey:UserID"`
	Quantity  int     `gorm:"not null" json:"quantity"` // positive = add, negative = subtract
	Type      string  `gorm:"not null" json:"type"`     // "add", "sale", "adjustment"
	Note      string  `json:"note"`
}
