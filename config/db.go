package config

import (
	"path/filepath"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	// Ensure data directory exists
	dir := filepath.Dir(AppConfig.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		panic("failed to create data directory: " + err.Error())
	}

	var err error
	DB, err = gorm.Open(sqlite.Open(AppConfig.DBPath), &gorm.Config{})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}
}
