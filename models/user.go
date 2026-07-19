package models

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	ShopID       uint   `gorm:"not null;index" json:"shop_id"`
	Shop         Shop   `json:"shop" gorm:"foreignKey:ShopID"`
	Email        string `gorm:"uniqueIndex;not null" json:"email" validate:"required,email"`
	PasswordHash string `gorm:"not null" json:"-"`
	Name         string `gorm:"not null" json:"name" validate:"required,min=2"`
	Role         string `gorm:"default:'staff'" json:"role" validate:"required,oneof=admin staff"`
}

func (u *User) SetPassword(password string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hashed)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}
