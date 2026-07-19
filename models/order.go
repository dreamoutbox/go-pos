package models

import (
	"time"

	"gorm.io/gorm"
)

type Order struct {
	gorm.Model
	ShopID      uint        `gorm:"not null;index" json:"shop_id"`
	Shop        Shop        `json:"shop" gorm:"foreignKey:ShopID"`
	UserID      uint        `gorm:"not null" json:"user_id"` // Cashier
	User        User        `json:"user" gorm:"foreignKey:UserID"`
	Status      string      `gorm:"default:'pending'" json:"status"` // "pending" | "paid"
	Subtotal    float64     `json:"subtotal"`
	TaxAmount   float64     `json:"tax_amount"`
	Total       float64     `json:"total"`
	PaidAt      *time.Time  `json:"paid_at"`
	OrderItems  []OrderItem `json:"order_items" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
}

type OrderItem struct {
	gorm.Model
	OrderID   uint    `gorm:"not null;index" json:"order_id"`
	ProductID uint    `gorm:"not null" json:"product_id"`
	Product   Product `json:"product" gorm:"foreignKey:ProductID"`
	Name      string  `gorm:"not null" json:"name"` // snapshot name
	Price     float64 `gorm:"not null" json:"price"` // snapshot price
	Cost      float64 `json:"cost"`                 // snapshot cost
	Quantity  int     `gorm:"not null" json:"quantity"`
	Subtotal  float64 `gorm:"not null" json:"subtotal"`
}
