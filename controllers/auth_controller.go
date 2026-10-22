package controllers

import (
	"net/http"
	"pawtner-api/config"
	"pawtner-api/models"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type RegisterInput struct {
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Gender   string `json:"gender" binding:"required"`
}

func Login(c *gin.Context) {
	//
}

func Register(c *gin.Context) {
	var input RegisterInput

	// Validasi Input
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hash Password before Saved to database
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		FullName:  input.FullName,
		Email:     input.Email,
		Password:  string(hashedPassword),
		Gender:    input.Gender,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result := config.DB.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already registered or something wrong"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registration Successfully",
		"data": gin.H{
			"id":        user.ID,
			"full_name": user.FullName,
			"email":     user.Email,
		},
	})
}
