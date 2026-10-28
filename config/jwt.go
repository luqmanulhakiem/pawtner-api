package config

import (
	"log"
	"os"
)

func GetJWTSecretKey() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Println("Peringatan: JWT_SECRET tidak ditemukan di environment, menggunakan default.")
		secret = "default-secret-of-jwt"
	}
	return []byte(secret)
}
