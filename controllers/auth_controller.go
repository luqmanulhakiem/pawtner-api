package controllers

import (
	"net/http"
	"pawtner-api/config"
	"pawtner-api/helpers"
	"pawtner-api/models"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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

// GenerateAccessToken membuat access token berumur pendek
func GenerateAccessToken(user models.User) (string, error) {
	accessClaims := jwt.MapClaims{
		"id":    user.ID,
		"email": user.Email,
		"exp":   time.Now().Add(time.Minute * 15).Unix(), // 15 Menit
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	return tokenObj.SignedString(config.GetJWTSecretKey())
}

// GenerateRefreshToken membuat refresh token berumur panjang
func GenerateRefreshToken(user models.User) (string, error) {
	refreshClaims := jwt.MapClaims{
		"id":  user.ID,
		"exp": time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 Hari
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	return tokenObj.SignedString(config.GetJWTSecretKey())
}

func Login(c *gin.Context) {
	var input LoginInput
	var user models.User

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := config.DB.WithContext(c.Request.Context()).Where("email = ?", input.Email).First(&user).Error; err != nil {
		helpers.Error(c, http.StatusUnauthorized, "Wrong Email or Password")
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
	if err != nil {
		helpers.Error(c, http.StatusUnauthorized, "Wrong Email or Password")
		return
	}

	accessToken, err := GenerateAccessToken(user)
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, "Could not generate access token")
		return
	}

	// Panggil fungsi terpisah untuk membuat Refresh Token
	refreshToken, err := GenerateRefreshToken(user)
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, "Could not generate refresh token")
		return
	}

	// Simpan Refresh Token ke Database
	user.RefreshToken = &refreshToken
	if err := config.DB.WithContext(c.Request.Context()).Save(&user).Error; err != nil {
		helpers.Error(c, http.StatusInternalServerError, "Could not save refresh token")
		return
	}

	helpers.Success(c, http.StatusOK, "Login Successfully", gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": gin.H{
			"name":        user.FullName,
			"email":       user.Email,
			"gender":      user.Gender,
			"unique_code": user.UniqueCode,
		},
	})
}

func Register(c *gin.Context) {
	var input RegisterInput

	// Validasi Input
	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Gender dinormalisasi ke huruf kecil dan harus male/female
	gender := strings.ToLower(strings.TrimSpace(input.Gender))
	if gender != models.GenderMale && gender != models.GenderFemale {
		helpers.Error(c, http.StatusBadRequest, "Gender must be 'male' or 'female'")
		return
	}

	// Hash Password before Saved to database
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	user := models.User{
		FullName:  input.FullName,
		Email:     input.Email,
		Password:  string(hashedPassword),
		Gender:    gender,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	result := config.DB.WithContext(c.Request.Context()).Create(&user)
	if result.Error != nil {
		helpers.Error(c, http.StatusBadRequest, "Email already registered or something wrong")
		return
	}

	helpers.Success(c, http.StatusCreated, "Registration Successfully", gin.H{
		"id":          user.ID,
		"full_name":   user.FullName,
		"email":       user.Email,
		"unique_code": user.UniqueCode,
	})
}
