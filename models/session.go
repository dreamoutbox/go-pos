package models

import (
	"time"

	"gorm.io/gorm"
)

type Session struct {
	gorm.Model
	Token        string    `gorm:"uniqueIndex;not null" json:"token"`
	UserID       uint      `gorm:"not null;index" json:"user_id"`
	User         User      `json:"user" gorm:"foreignKey:UserID"`
	ActiveShopID uint      `gorm:"not null" json:"active_shop_id"`
	ExpiresAt    time.Time `gorm:"not null;index" json:"expires_at"`
}
