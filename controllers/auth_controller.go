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

type LoginInput struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var input LoginInput
	var user models.User

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.DB.Where("email = ?", input.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong Email or Password"})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong Email or Password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Login Succesfully",
		"data": gin.H{
			"name":   user.FullName,
			"email":  user.Email,
			"gender": user.Gender,
		},
	})
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
