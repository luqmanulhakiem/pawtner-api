package config

import (
	"fmt"
	"log"
	"pawtner-api/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {
	dsn := "root:qwerty123@unix(/tmp/mysql.sock)/pawtner_db?charset=utf8mb4&parseTime=True&loc=Local"

	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Gagal terhubung ke database:", err)
	}

	if err := database.AutoMigrate(&models.User{}, &models.DailyMood{}); err != nil {
		log.Fatal("Gagal migrasi database:", err)
	}

	DB = database
	fmt.Println("Berhasil terhubung ke database MySQL!")
}
