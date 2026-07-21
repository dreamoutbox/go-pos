package models

import (
	"time"

	"gorm.io/gorm"
)

// DocumentSequence tracks monthly running numbers per shop and prefix.
// Prefix examples: "ORD", "RFD", "CDN", "DBN"
// YearMonth format: "202607"
type DocumentSequence struct {
	ID        uint   `gorm:"primaryKey"`
	ShopID    uint   `gorm:"not null;index:idx_shop_prefix_ym,unique"`
	Prefix    string `gorm:"not null;size:10;index:idx_shop_prefix_ym,unique"`
	YearMonth string `gorm:"not null;size:6;index:idx_shop_prefix_ym,unique"` // e.g. "202607"
	LastValue int    `gorm:"not null;default:0"`
	UpdatedAt time.Time
}

// Refund represents a monetary & optional stock refund against an Order
type Refund struct {
	gorm.Model
	Code        string       `gorm:"not null;index" json:"code"` // RFD-202607-00012
	ShopID      uint         `gorm:"not null;index" json:"shop_id"`
	Shop        Shop         `json:"shop" gorm:"foreignKey:ShopID"`
	OrderID     uint         `gorm:"not null;index" json:"order_id"`
	Order       Order        `json:"order" gorm:"foreignKey:OrderID"`
	UserID      uint         `gorm:"not null" json:"user_id"`
	User        User         `json:"user" gorm:"foreignKey:UserID"`
	Subtotal    float64      `json:"subtotal"`
	TaxAmount   float64      `json:"tax_amount"`
	Total       float64      `json:"total"`
	Reason      string       `json:"reason"`
	RefundItems []RefundItem `json:"refund_items" gorm:"foreignKey:RefundID;constraint:OnDelete:CASCADE"`
}

type RefundItem struct {
	gorm.Model
	RefundID      uint    `gorm:"not null;index" json:"refund_id"`
	OrderItemID   uint    `gorm:"not null" json:"order_item_id"`
	ProductID     uint    `gorm:"not null" json:"product_id"`
	Product       Product `json:"product" gorm:"foreignKey:ProductID"`
	Name          string  `gorm:"not null" json:"name"`
	Price         float64 `gorm:"not null" json:"price"`
	Quantity      int     `gorm:"not null" json:"quantity"`
	Subtotal      float64 `gorm:"not null" json:"subtotal"`
	ReturnToStock bool    `gorm:"default:false" json:"return_to_stock"`
}

// CreditNote represents a formal tax credit adjustment document against an Order
type CreditNote struct {
	gorm.Model
	Code            string           `gorm:"not null;index" json:"code"` // CDN-202607-00020
	ShopID          uint             `gorm:"not null;index" json:"shop_id"`
	Shop            Shop             `json:"shop" gorm:"foreignKey:ShopID"`
	OrderID         uint             `gorm:"not null;index" json:"order_id"`
	Order           Order            `json:"order" gorm:"foreignKey:OrderID"`
	UserID          uint             `gorm:"not null" json:"user_id"`
	User            User             `json:"user" gorm:"foreignKey:UserID"`
	Subtotal        float64          `json:"subtotal"`
	TaxAmount       float64          `json:"tax_amount"`
	Total           float64          `json:"total"`
	Reason          string           `json:"reason"`
	CreditNoteItems []CreditNoteItem `json:"credit_note_items" gorm:"foreignKey:CreditNoteID;constraint:OnDelete:CASCADE"`
}

type CreditNoteItem struct {
	gorm.Model
	CreditNoteID  uint    `gorm:"not null;index" json:"credit_note_id"`
	OrderItemID   uint    `gorm:"not null" json:"order_item_id"`
	ProductID     uint    `gorm:"not null" json:"product_id"`
	Product       Product `json:"product" gorm:"foreignKey:ProductID"`
	Name          string  `gorm:"not null" json:"name"`
	Price         float64 `gorm:"not null" json:"price"`
	Quantity      int     `gorm:"not null" json:"quantity"`
	Subtotal      float64 `gorm:"not null" json:"subtotal"`
	ReturnToStock bool    `gorm:"default:false" json:"return_to_stock"`
}

// DebitNote represents an additional charge/fee/item document against an Order
type DebitNote struct {
	gorm.Model
	Code           string          `gorm:"not null;index" json:"code"` // DBN-202607-00030
	ShopID         uint            `gorm:"not null;index" json:"shop_id"`
	Shop           Shop            `json:"shop" gorm:"foreignKey:ShopID"`
	OrderID        uint            `gorm:"not null;index" json:"order_id"`
	Order          Order           `json:"order" gorm:"foreignKey:OrderID"`
	UserID         uint            `gorm:"not null" json:"user_id"`
	User           User            `json:"user" gorm:"foreignKey:UserID"`
	Subtotal       float64         `json:"subtotal"`
	TaxAmount      float64         `json:"tax_amount"`
	Total          float64         `json:"total"`
	Reason         string          `json:"reason"`
	DebitNoteItems []DebitNoteItem `json:"debit_note_items" gorm:"foreignKey:DebitNoteID;constraint:OnDelete:CASCADE"`
}

type DebitNoteItem struct {
	gorm.Model
	DebitNoteID uint     `gorm:"not null;index" json:"debit_note_id"`
	ProductID   *uint    `json:"product_id"`
	Product     *Product `json:"product" gorm:"foreignKey:ProductID"`
	Name        string   `gorm:"not null" json:"name"`
	Price       float64  `gorm:"not null" json:"price"`
	Quantity    int      `gorm:"not null" json:"quantity"`
	Subtotal    float64  `gorm:"not null" json:"subtotal"`
	DeductStock bool     `gorm:"default:false" json:"deduct_stock"`
}
