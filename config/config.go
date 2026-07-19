package config

import (
	"os"
)

type Config struct {
	Port      string
	DBPath    string
	JWTSecret []byte
	UploadDir string
}

var AppConfig Config

func LoadConfig() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/pos.db"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "super-secret-pos-jwt-key-change-me"
	}

	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./data/product"
	}

	AppConfig = Config{
		Port:      port,
		DBPath:    dbPath,
		JWTSecret: []byte(jwtSecret),
		UploadDir: uploadDir,
	}
}
