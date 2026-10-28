package controllers

import (
	"errors"
	"net/http"
	"pawtner-api/config"
	"pawtner-api/helpers"
	"pawtner-api/models"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type InvitePartnerInput struct {
	UniqueCode string `json:"unique_code" binding:"required"`
}

func normalizeUniqueCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// partnerError memetakan error business logic partner ke respons HTTP.
func partnerError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		helpers.Error(c, http.StatusNotFound, "Partner data not found")
	case errors.Is(err, models.ErrPartnerForbidden),
		errors.Is(err, models.ErrPartnerMaleOnlyInvite):
		helpers.Error(c, http.StatusForbidden, err.Error())
	case errors.Is(err, models.ErrPartnerSelfInvite),
		errors.Is(err, models.ErrPartnerInvalidGender),
		errors.Is(err, models.ErrPartnerSameGender):
		helpers.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, models.ErrPartnerInviterTaken),
		errors.Is(err, models.ErrPartnerInviteeTaken),
		errors.Is(err, models.ErrPartnerPendingExists),
		errors.Is(err, models.ErrPartnerNotPending),
		errors.Is(err, models.ErrPartnerNotActive):
		helpers.Error(c, http.StatusConflict, err.Error())
	default:
		helpers.Error(c, http.StatusInternalServerError, fallback)
	}
}

// SearchPartner mencari user berdasarkan unique code: GET /partners/search?code=XXXX
func SearchPartner(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	code := normalizeUniqueCode(c.Query("code"))
	if code == "" {
		helpers.Error(c, http.StatusBadRequest, "Query 'code' is required")
		return
	}

	user, err := models.FindUserByUniqueCode(config.DB.WithContext(c.Request.Context()), code)
	if err != nil || user.ID == userId {
		if err == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			helpers.Error(c, http.StatusNotFound, "User not found")
			return
		}
		helpers.Error(c, http.StatusInternalServerError, "Failed to search user")
		return
	}

	helpers.Success(c, http.StatusOK, "User found", gin.H{
		"full_name":   user.FullName,
		"gender":      user.Gender,
		"unique_code": user.UniqueCode,
	})
}

func InvitePartner(c *gin.Context) {
	var input InvitePartnerInput

	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		helpers.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	partner, err := models.InvitePartner(config.DB.WithContext(c.Request.Context()), userId, normalizeUniqueCode(input.UniqueCode))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			helpers.Error(c, http.StatusNotFound, "User with this unique code not found")
			return
		}
		partnerError(c, err, "Failed to send partner invitation")
		return
	}

	helpers.Success(c, http.StatusCreated, "Partner invitation sent successfully", partner)
}

// GetPartnerRequests menampilkan undangan pending yang masuk ke user.
func GetPartnerRequests(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	requests, err := models.GetIncomingPartnerRequests(config.DB.WithContext(c.Request.Context()), userId)
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, "Failed to fetch partner requests")
		return
	}

	helpers.Success(c, http.StatusOK, "Partner requests fetched successfully", requests)
}

// GetSentPartnerRequests menampilkan undangan yang dikirim user beserta statusnya.
// Opsional filter ?status=pending|active|rejected|disabled.
func GetSentPartnerRequests(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	status := c.Query("status")
	if status != "" && !models.IsValidPartnerStatus(status) {
		helpers.Error(c, http.StatusBadRequest, "Status must be 'pending', 'active', 'rejected', or 'disabled'")
		return
	}

	requests, err := models.GetSentPartnerRequests(config.DB.WithContext(c.Request.Context()), userId, status)
	if err != nil {
		helpers.Error(c, http.StatusInternalServerError, "Failed to fetch sent partner requests")
		return
	}

	helpers.Success(c, http.StatusOK, "Sent partner requests fetched successfully", requests)
}

// GetActivePartner menampilkan partner yang sedang terkoneksi (aktif).
func GetActivePartner(c *gin.Context) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	partner, err := models.FindActivePartner(config.DB.WithContext(c.Request.Context()), userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			helpers.Error(c, http.StatusNotFound, "You don't have an active partner")
			return
		}
		helpers.Error(c, http.StatusInternalServerError, "Failed to fetch active partner")
		return
	}

	helpers.Success(c, http.StatusOK, "Active partner fetched successfully", partner)
}

// GetPartnerMoods menampilkan mood partner aktif. Opsional filter ?date=YYYY-MM-DD.
func GetPartnerMoods(c *gin.Context) {
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

	moods, err := models.GetActivePartnerMoods(config.DB.WithContext(c.Request.Context()), userId, date)
	if err != nil {
		if errors.Is(err, models.ErrPartnerNotActive) {
			helpers.Error(c, http.StatusForbidden, "You don't have an active partner")
			return
		}
		helpers.Error(c, http.StatusInternalServerError, "Failed to fetch partner moods")
		return
	}

	helpers.Success(c, http.StatusOK, "Partner moods fetched successfully", moods)
}

func AcceptPartner(c *gin.Context) {
	respondPartner(c, models.AcceptPartner, "Partner invitation accepted", "Failed to accept partner invitation")
}

func RejectPartner(c *gin.Context) {
	respondPartner(c, models.RejectPartner, "Partner invitation rejected", "Failed to reject partner invitation")
}

func DisablePartner(c *gin.Context) {
	respondPartner(c, models.DisablePartner, "Partner relationship disabled", "Failed to disable partner")
}

// respondPartner adalah handler bersama untuk aksi pada /partners/:id/*.
func respondPartner(c *gin.Context, action func(*gorm.DB, uint, uint) (models.Partner, error), successMsg, failMsg string) {
	userId, ok := currentUserID(c)
	if !ok {
		helpers.Error(c, http.StatusUnauthorized, "Invalid user session")
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		helpers.Error(c, http.StatusBadRequest, "Invalid partner id")
		return
	}

	partner, err := action(config.DB.WithContext(c.Request.Context()), uint(id), userId)
	if err != nil {
		partnerError(c, err, failMsg)
		return
	}

	helpers.Success(c, http.StatusOK, successMsg, partner)
}
