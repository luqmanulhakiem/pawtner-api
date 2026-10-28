package controllers

import (
	"errors"
	"net/http"
	"pawtner-api/config"
	"pawtner-api/helpers"
	"pawtner-api/models"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const moodDateLayout = "2006-01-02"

type CreateDailyMoodInput struct {
	Date       string `json:"date" binding:"required"`   // YYYY-MM-DD
	Period     string `json:"period" binding:"required"` // morning | night
	MoodStatus string `json:"mood_status" binding:"required"`
	Note       string `json:"note"`
}

type UpdateDailyMoodInput struct {
	MoodStatus string `json:"mood_status" binding:"required"`
	Note       string `json:"note"`
}

// currentUserID mengambil ID user dari klaim JWT yang disimpan middleware.
func currentUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get("currentUserId")
	if !exists {
		return 0, false
	}
	// Angka di JWT MapClaims di-decode sebagai float64
	id, ok := value.(float64)
	if !ok {
		return 0, false
	}
	return uint(id), true
}

func CreateDailyMood(c *gin.Context) {
	var input CreateDailyMoodInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := time.Parse(moodDateLayout, input.Date); err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid date format, use YYYY-MM-DD")
		return
	}

	if !models.IsValidMoodPeriod(input.Period) {
		helpers.Error(c, http.StatusBadRequest, "Period must be 'morning' or 'night'")
		return
	}

	db := config.DB.WithContext(c.Request.Context())

	// Maksimal 1 mood per periode per hari
	_, err := models.FindDailyMoodByPeriod(db, userId, input.Date, input.Period)
	if err == nil {
		helpers.Error(c, http.StatusConflict, "Mood for this date and period already recorded")
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		helpers.Error(c, http.StatusInternalServerError, "Failed to check existing mood")
		return
	}

	mood := models.DailyMood{
		UserId:     userId,
		Date:       input.Date,
		Period:     input.Period,
		MoodStatus: input.MoodStatus,
		Note:       input.Note,
	}

	if err := models.CreateDailyMood(db, &mood); err != nil {
		helpers.Error(c, http.StatusInternalServerError, "Failed to save daily mood")
		return
	}

	helpers.Success(c, http.StatusCreated, "Daily mood recorded successfully", mood)
}

// GetDailyMoods menampilkan mood milik user. Opsional filter ?date=YYYY-MM-DD.
func GetDailyMoods(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	date := c.Query("date")
	if date != "" {
		if _, err := time.Parse(moodDateLayout, date); err != nil {
			helpers.Error(c, http.StatusBadRequest, "Invalid date format, use YYYY-MM-DD")
			return
		}
	}

	db := config.DB.WithContext(c.Request.Context())

	moods, err := models.GetDailyMoods(db, userId, date)
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, "Failed to fetch daily moods")
		return
	}

	helpers.Success(c, http.StatusOK, "Daily moods fetched successfully", moods)
}

func GetDailyMood(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid mood id")
		return
	}

	db := config.DB.WithContext(c.Request.Context())

	mood, err := models.FindDailyMoodByID(db, uint(id), userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			helpers.Error(c, http.StatusNotFound, "Daily mood not found")
			return
		}
		helpers.Error(c, http.StatusInternalServerError, "Failed to fetch daily mood")
		return
	}

	helpers.Success(c, http.StatusOK, "Daily mood fetched successfully", mood)
}

// UpdateDailyMood hanya mengubah mood_status dan note; tanggal dan periode tetap.
func UpdateDailyMood(c *gin.Context) {
	var input UpdateDailyMoodInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid mood id")
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	db := config.DB.WithContext(c.Request.Context())

	mood, err := models.FindDailyMoodByID(db, uint(id), userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			helpers.Error(c, http.StatusNotFound, "Daily mood not found")
			return
		}
		helpers.Error(c, http.StatusInternalServerError, "Failed to fetch daily mood")
		return
	}

	mood.MoodStatus = input.MoodStatus
	mood.Note = input.Note

	if err := models.UpdateDailyMood(db, &mood); err != nil {
		helpers.Error(c, http.StatusInternalServerError, "Failed to update daily mood")
		return
	}

	helpers.Success(c, http.StatusOK, "Daily mood updated successfully", mood)
}
