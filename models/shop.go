package models

import (
	"gorm.io/gorm"
)

type Shop struct {
	gorm.Model
	Name        string  `gorm:"not null" json:"name" validate:"required"`
	Address     string  `json:"address"`
	Phone       string  `json:"phone"`
	TaxEnabled  bool    `gorm:"default:false" json:"tax_enabled"`
	TaxIncluded bool    `gorm:"default:true" json:"tax_included"` // true = prices already include VAT
	TaxName     string  `json:"tax_name"`
	TaxAddress  string  `json:"tax_address"`
	TaxID       string  `json:"tax_id"`
	TaxRate     float64 `gorm:"default:7.0" json:"tax_rate"` // Default Thailand VAT 7%
}
