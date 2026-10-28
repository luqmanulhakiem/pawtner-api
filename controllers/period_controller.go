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

type PeriodInput struct {
	StartDate string  `json:"start_date" binding:"required"` // YYYY-MM-DD
	EndDate   *string `json:"end_date"`                      // YYYY-MM-DD, kosong jika masih berlangsung
	Note      string  `json:"note"`
}

func validPeriodDates(input PeriodInput) bool {
	if _, err := time.Parse(moodDateLayout, input.StartDate); err != nil {
		return false
	}
	if input.EndDate != nil {
		if _, err := time.Parse(moodDateLayout, *input.EndDate); err != nil {
			return false
		}
	}
	return true
}

// periodError memetakan error business logic period ke respons HTTP.
func periodError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, models.ErrPeriodFemaleOnly),
		errors.Is(err, models.ErrPeriodMaleOnly):
		helpers.Error(c, http.StatusForbidden, err.Error())
	case errors.Is(err, models.ErrPeriodInvalid):
		helpers.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, models.ErrPeriodOverlap):
		helpers.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, models.ErrPartnerNotActive):
		helpers.Error(c, http.StatusForbidden, "You don't have an active partner")
	case errors.Is(err, gorm.ErrRecordNotFound):
		helpers.Error(c, http.StatusNotFound, "Period not found")
	default:
		helpers.Error(c, http.StatusInternalServerError, fallback)
	}
}

func CreatePeriod(c *gin.Context) {
	var input PeriodInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if !validPeriodDates(input) {
		helpers.Error(c, http.StatusBadRequest, "Invalid date format, use YYYY-MM-DD")
		return
	}

	period := models.Period{UserId: userId, StartDate: input.StartDate, EndDate: input.EndDate, Note: input.Note}
	if err := models.CreatePeriod(config.DB.WithContext(c.Request.Context()), &period); err != nil {
		periodError(c, err, "Failed to save period")
		return
	}

	helpers.Success(c, http.StatusCreated, "Period recorded successfully", period)
}

// GetPeriods menampilkan riwayat periode milik user sendiri.
func GetPeriods(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	periods, err := models.GetPeriods(config.DB.WithContext(c.Request.Context()), userId)
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, "Failed to fetch periods")
		return
	}

	helpers.Success(c, http.StatusOK, "Periods fetched successfully", periods)
}

func GetPeriod(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid period id")
		return
	}

	period, err := models.FindPeriodByID(config.DB.WithContext(c.Request.Context()), uint(id), userId)
	if err != nil {
		periodError(c, err, "Failed to fetch period")
		return
	}

	helpers.Success(c, http.StatusOK, "Period fetched successfully", period)
}

func UpdatePeriod(c *gin.Context) {
	var input PeriodInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid period id")
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if !validPeriodDates(input) {
		helpers.Error(c, http.StatusBadRequest, "Invalid date format, use YYYY-MM-DD")
		return
	}

	db := config.DB.WithContext(c.Request.Context())

	period, err := models.FindPeriodByID(db, uint(id), userId)
	if err != nil {
		periodError(c, err, "Failed to fetch period")
		return
	}

	period.StartDate = input.StartDate
	period.EndDate = input.EndDate
	period.Note = input.Note

	if err := models.UpdatePeriod(db, &period); err != nil {
		periodError(c, err, "Failed to update period")
		return
	}

	helpers.Success(c, http.StatusOK, "Period updated successfully", period)
}

// GetPartnerPeriods menampilkan riwayat periode pasangan aktif (khusus male).
func GetPartnerPeriods(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	periods, err := models.GetActivePartnerPeriods(config.DB.WithContext(c.Request.Context()), userId)
	if err != nil {
		periodError(c, err, "Failed to fetch partner periods")
		return
	}

	helpers.Success(c, http.StatusOK, "Partner periods fetched successfully", periods)
}

// GetPartnerPeriodToday menunjukkan apakah pasangan aktif sedang periode pada tanggal
// hari ini milik user: GET /partners/active/periods/today?date=YYYY-MM-DD (khusus male).
func GetPartnerPeriodToday(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	date := c.Query("date")
	if date == "" {
		helpers.Error(c, http.StatusBadRequest, "Query 'date' is required")
		return
	}
	if _, err := time.Parse(moodDateLayout, date); err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid date format, use YYYY-MM-DD")
		return
	}

	status, err := models.GetActivePartnerPeriodStatus(config.DB.WithContext(c.Request.Context()), userId, date)
	if err != nil {
		periodError(c, err, "Failed to fetch partner period status")
		return
	}

	helpers.Success(c, http.StatusOK, "Partner period status fetched successfully", status)
}

// GetPeriodInsight menampilkan fase siklus dan deskripsi kesehatan untuk user female:
// GET /periods/insight?date=YYYY-MM-DD
func GetPeriodInsight(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	date := c.Query("date")
	if date == "" {
		helpers.Error(c, http.StatusBadRequest, "Query 'date' is required")
		return
	}
	if _, err := time.Parse(moodDateLayout, date); err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid date format, use YYYY-MM-DD")
		return
	}

	insight, err := models.GetPeriodInsight(config.DB.WithContext(c.Request.Context()), userId, date)
	if err != nil {
		periodError(c, err, "Failed to fetch period insight")
		return
	}

	helpers.Success(c, http.StatusOK, "Period insight fetched successfully", insight)
}
